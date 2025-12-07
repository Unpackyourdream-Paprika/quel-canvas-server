package multiview

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gen2brain/webp"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/supabase-community/supabase-go"
	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/model"
	redisutil "quel-canvas-server/modules/common/redis"
)

type Service struct {
	supabase    *supabase.Client
	genaiClient *genai.Client
	redis       *redis.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ [Multiview] Failed to create Supabase client: %v", err)
		return nil
	}

	// Genai 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("❌ [Multiview] Failed to create Genai client: %v", err)
		return nil
	}

	// Redis 클라이언트 초기화
	redisClient := redisutil.Connect(cfg)
	if redisClient == nil {
		log.Printf("⚠️ [Multiview] Failed to connect to Redis")
	}

	log.Println("✅ [Multiview] Service initialized")
	return &Service{
		supabase:    supabaseClient,
		genaiClient: genaiClient,
		redis:       redisClient,
	}
}

// GenerateMultiview - 360도 다각도 이미지 생성 (동기 방식)
func (s *Service) GenerateMultiview(ctx context.Context, req *MultiviewGenerateRequest) (*MultiviewGenerateResponse, error) {
	cfg := config.GetConfig()
	jobID := uuid.New().String()

	log.Printf("🔄 [Multiview] Starting multiview generation - JobID: %s, User: %s", jobID, req.UserID)

	// 원본 이미지 필수 체크
	if req.SourceImage == "" {
		return &MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: "Source image is required",
			ErrorCode:    ErrCodeImageRequired,
		}, nil
	}

	// 각도 설정 (기본값: 8개 각도)
	angles := req.Angles
	if len(angles) == 0 {
		angles = DefaultAngles
	}

	// 각도 유효성 검사
	for _, angle := range angles {
		if !IsValidAngle(angle) {
			return &MultiviewGenerateResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Invalid angle: %d (must be 0-359)", angle),
				ErrorCode:    ErrCodeInvalidAngle,
			}, nil
		}
	}

	// 크레딧 확인 (각도 개수만큼 필요)
	requiredCredits := len(angles) * cfg.ImagePerPrice
	credits, err := s.CheckUserCredits(ctx, req.UserID)
	if err != nil {
		log.Printf("⚠️ [Multiview] Failed to check credits: %v", err)
	} else if credits < requiredCredits {
		return &MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Insufficient credits. Required: %d, Available: %d", requiredCredits, credits),
			ErrorCode:    ErrCodeInsufficientCredits,
		}, nil
	}

	// 원본 이미지 디코딩
	sourceImageData, err := s.decodeBase64Image(req.SourceImage)
	if err != nil {
		return &MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: "Failed to decode source image",
			ErrorCode:    ErrCodeInvalidRequest,
		}, err
	}

	// 레퍼런스 이미지를 각도별로 매핑
	referenceMap := make(map[int][]byte)
	for _, ref := range req.ReferenceImages {
		refData, err := s.decodeBase64Image(ref.Image)
		if err != nil {
			log.Printf("⚠️ [Multiview] Failed to decode reference image for angle %d: %v", ref.Angle, err)
			continue
		}
		referenceMap[ref.Angle] = refData
		log.Printf("📎 [Multiview] Reference image loaded for angle %d", ref.Angle)
	}

	// Aspect ratio 기본값
	aspectRatio := req.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "1:1"
	}

	// 각 각도별로 이미지 생성
	var generatedImages []GeneratedAngleImage
	var totalCreditsUsed int

	for _, angle := range angles {
		log.Printf("🎨 [Multiview] Generating angle %d (%s)...", angle, GetAngleLabel(angle))

		// 0도는 원본 이미지 그대로 사용
		if angle == 0 {
			// 원본 이미지를 저장하고 attach 생성
			filePath, fileSize, err := s.UploadImageToStorage(ctx, sourceImageData, req.UserID, angle)
			if err != nil {
				log.Printf("⚠️ [Multiview] Failed to upload source image: %v", err)
				generatedImages = append(generatedImages, GeneratedAngleImage{
					Angle:        angle,
					AngleLabel:   GetAngleLabel(angle),
					Success:      false,
					ErrorMessage: "Failed to save source image",
				})
				continue
			}

			attachID, _ := s.CreateAttachRecord(ctx, filePath, fileSize)
			imageURL := cfg.SupabaseStorageBaseURL + filePath

			generatedImages = append(generatedImages, GeneratedAngleImage{
				Angle:       angle,
				AngleLabel:  GetAngleLabel(angle),
				ImageURL:    imageURL,
				ImageBase64: base64.StdEncoding.EncodeToString(sourceImageData),
				AttachID:    attachID,
				Success:     true,
			})
			continue
		}

		// Gemini API 호출 준비
		var parts []*genai.Part

		// 원본 이미지 추가
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     sourceImageData,
			},
		})

		// 해당 각도에 레퍼런스가 있으면 추가
		hasReference := false
		if refData, ok := referenceMap[angle]; ok {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: "image/png",
					Data:     refData,
				},
			})
			hasReference = true
			log.Printf("📎 [Multiview] Using reference image for angle %d", angle)
		}

		// 프롬프트 생성
		prompt := BuildMultiviewPrompt(0, angle, req.Category, req.OriginalPrompt, hasReference)
		parts = append(parts, genai.NewPartFromText(prompt))

		// Content 생성
		content := &genai.Content{
			Parts: parts,
		}

		// Gemini API 호출
		result, err := s.genaiClient.Models.GenerateContent(
			ctx,
			cfg.GeminiModel,
			[]*genai.Content{content},
			&genai.GenerateContentConfig{
				ImageConfig: &genai.ImageConfig{
					AspectRatio: aspectRatio,
				},
				Temperature: floatPtr(0.5), // 일관성을 위해 낮은 temperature
			},
		)

		if err != nil {
			log.Printf("❌ [Multiview] Gemini API error for angle %d: %v", angle, err)
			generatedImages = append(generatedImages, GeneratedAngleImage{
				Angle:        angle,
				AngleLabel:   GetAngleLabel(angle),
				Success:      false,
				ErrorMessage: fmt.Sprintf("Generation failed: %v", err),
			})
			continue
		}

		// 응답에서 이미지 추출
		imageExtracted := false
		if len(result.Candidates) > 0 {
			for _, candidate := range result.Candidates {
				if candidate.Content == nil {
					continue
				}

				for _, part := range candidate.Content.Parts {
					if part.InlineData != nil && len(part.InlineData.Data) > 0 {
						imageData := part.InlineData.Data
						log.Printf("✅ [Multiview] Image generated for angle %d: %d bytes", angle, len(imageData))

						// Storage에 업로드
						filePath, fileSize, err := s.UploadImageToStorage(ctx, imageData, req.UserID, angle)
						if err != nil {
							log.Printf("⚠️ [Multiview] Failed to upload image for angle %d: %v", angle, err)
							generatedImages = append(generatedImages, GeneratedAngleImage{
								Angle:       angle,
								AngleLabel:  GetAngleLabel(angle),
								ImageBase64: base64.StdEncoding.EncodeToString(imageData),
								Success:     true,
							})
						} else {
							attachID, _ := s.CreateAttachRecord(ctx, filePath, fileSize)
							imageURL := cfg.SupabaseStorageBaseURL + filePath

							generatedImages = append(generatedImages, GeneratedAngleImage{
								Angle:       angle,
								AngleLabel:  GetAngleLabel(angle),
								ImageURL:    imageURL,
								ImageBase64: base64.StdEncoding.EncodeToString(imageData),
								AttachID:    attachID,
								Success:     true,
							})

							totalCreditsUsed += cfg.ImagePerPrice
						}

						imageExtracted = true
						break
					}
				}
				if imageExtracted {
					break
				}
			}
		}

		if !imageExtracted {
			log.Printf("⚠️ [Multiview] No image in response for angle %d", angle)
			generatedImages = append(generatedImages, GeneratedAngleImage{
				Angle:        angle,
				AngleLabel:   GetAngleLabel(angle),
				Success:      false,
				ErrorMessage: "No image in API response",
			})
		}
	}

	// 크레딧 차감
	if totalCreditsUsed > 0 {
		if err := s.DeductCredits(ctx, req.UserID, totalCreditsUsed); err != nil {
			log.Printf("⚠️ [Multiview] Failed to deduct credits: %v", err)
		}
	}

	// 성공한 이미지 수 계산
	successCount := 0
	for _, img := range generatedImages {
		if img.Success {
			successCount++
		}
	}

	log.Printf("✅ [Multiview] Generation completed - JobID: %s, Success: %d/%d", jobID, successCount, len(angles))

	remainingCredits, _ := s.CheckUserCredits(ctx, req.UserID)

	return &MultiviewGenerateResponse{
		Success:          successCount > 0,
		JobID:            jobID,
		GeneratedImages:  generatedImages,
		TotalImages:      len(angles),
		CreditsUsed:      totalCreditsUsed,
		CreditsRemaining: remainingCredits,
	}, nil
}

// CheckUserCredits - 사용자 크레딧 확인
func (s *Service) CheckUserCredits(ctx context.Context, userID string) (int, error) {
	var members []struct {
		QuelMemberCredit int `json:"quel_member_credit"`
	}

	data, _, err := s.supabase.From("quel_member").
		Select("quel_member_credit", "", false).
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		return 0, fmt.Errorf("failed to fetch user credits: %w", err)
	}

	if err := json.Unmarshal(data, &members); err != nil {
		return 0, fmt.Errorf("failed to parse member data: %w", err)
	}

	if len(members) == 0 {
		return 0, fmt.Errorf("user not found: %s", userID)
	}

	return members[0].QuelMemberCredit, nil
}

// DeductCredits - 크레딧 차감
func (s *Service) DeductCredits(ctx context.Context, userID string, amount int) error {
	// 현재 크레딧 조회
	currentCredits, err := s.CheckUserCredits(ctx, userID)
	if err != nil {
		return err
	}

	newBalance := currentCredits - amount

	log.Printf("💰 [Multiview] Deducting credits: user=%s, %d → %d (-%d)",
		userID, currentCredits, newBalance, amount)

	// 크레딧 차감
	_, _, err = s.supabase.From("quel_member").
		Update(map[string]interface{}{
			"quel_member_credit": newBalance,
		}, "", "").
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to deduct credits: %w", err)
	}

	// 트랜잭션 기록
	transactionData := map[string]interface{}{
		"user_id":          userID,
		"transaction_type": "DEDUCT",
		"amount":           -amount,
		"balance_after":    newBalance,
		"description":      "Multiview 360 Image Generation",
	}

	_, _, err = s.supabase.From("quel_credits").
		Insert(transactionData, false, "", "", "").
		Execute()

	if err != nil {
		log.Printf("⚠️ [Multiview] Failed to record transaction: %v", err)
	}

	log.Printf("✅ [Multiview] Credits deducted: %d credits from user %s", amount, userID)
	return nil
}

// UploadImageToStorage - Supabase Storage에 이미지 업로드 (WebP 변환)
func (s *Service) UploadImageToStorage(ctx context.Context, imageData []byte, userID string, angle int) (string, int64, error) {
	cfg := config.GetConfig()

	// PNG를 WebP로 변환
	webpData, err := s.ConvertPNGToWebP(imageData, 90.0)
	if err != nil {
		log.Printf("⚠️ [Multiview] WebP conversion failed, using original: %v", err)
		webpData = imageData
	}

	// 파일명 생성
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	randomID := rand.Intn(999999)
	fileName := fmt.Sprintf("multiview_%d_angle%d_%d.webp", timestamp, angle, randomID)

	// 파일 경로 생성
	filePath := fmt.Sprintf("multiview-images/user-%s/%s", userID, fileName)

	log.Printf("📤 [Multiview] Uploading image to storage: %s", filePath)

	// Supabase Storage API URL
	uploadURL := fmt.Sprintf("%s/storage/v1/object/attachments/%s",
		cfg.SupabaseURL, filePath)

	// HTTP Request 생성
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(webpData))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceKey)
	req.Header.Set("Content-Type", "image/webp")

	// 업로드 실행
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to upload image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	webpSize := int64(len(webpData))
	log.Printf("✅ [Multiview] Image uploaded: %s (%d bytes)", filePath, webpSize)
	return filePath, webpSize, nil
}

// ConvertPNGToWebP - PNG를 WebP로 변환
func (s *Service) ConvertPNGToWebP(pngData []byte, quality float32) ([]byte, error) {
	pngReader := bytes.NewReader(pngData)
	img, err := png.Decode(pngReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	var webpBuffer bytes.Buffer
	err = webp.Encode(&webpBuffer, img, webp.Options{Quality: int(quality)})
	if err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %w", err)
	}

	return webpBuffer.Bytes(), nil
}

// CreateAttachRecord - quel_attach 테이블에 레코드 생성
func (s *Service) CreateAttachRecord(ctx context.Context, filePath string, fileSize int64) (int, error) {
	log.Printf("💾 [Multiview] Creating attach record: %s", filePath)

	// 파일명 추출
	fileName := filePath
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' {
			fileName = filePath[i+1:]
			break
		}
	}

	insertData := map[string]interface{}{
		"attach_original_name": fileName,
		"attach_file_name":     fileName,
		"attach_file_path":     filePath,
		"attach_file_size":     fileSize,
		"attach_file_type":     "image/webp",
		"attach_directory":     filePath,
		"attach_storage_type":  "supabase",
	}

	data, _, err := s.supabase.From("quel_attach").
		Insert(insertData, false, "", "", "").
		Execute()

	if err != nil {
		return 0, fmt.Errorf("failed to insert attach record: %w", err)
	}

	var attaches []model.Attach
	if err := json.Unmarshal(data, &attaches); err != nil {
		return 0, fmt.Errorf("failed to parse attach response: %w", err)
	}

	if len(attaches) == 0 {
		return 0, fmt.Errorf("no attach record returned")
	}

	attachID := int(attaches[0].AttachID)
	log.Printf("✅ [Multiview] Attach record created: ID=%d", attachID)

	return attachID, nil
}

// decodeBase64Image - Base64 이미지 디코딩
func (s *Service) decodeBase64Image(imgBase64 string) ([]byte, error) {
	base64Data := imgBase64

	// data:image/xxx;base64, prefix 제거
	if idx := findBase64Start(imgBase64); idx > 0 {
		base64Data = imgBase64[idx:]
	}

	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	return imageData, nil
}

// Helper functions
func findBase64Start(s string) int {
	marker := ";base64,"
	for i := 0; i < len(s)-len(marker); i++ {
		if s[i:i+len(marker)] == marker {
			return i + len(marker)
		}
	}
	return 0
}

func floatPtr(f float64) *float32 {
	f32 := float32(f)
	return &f32
}
