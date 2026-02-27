package studio

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
	geminiretry "quel-canvas-server/modules/common/gemini"
	"quel-canvas-server/modules/common/model"
	"quel-canvas-server/modules/common/org"
	redisutil "quel-canvas-server/modules/common/redis"
	"quel-canvas-server/modules/unified-prompt/common"
)

type Service struct {
	supabase *supabase.Client
	redis    *redis.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ [Studio] Failed to create Supabase client: %v", err)
		return nil
	}

	// Redis 클라이언트 초기화
	redisClient := redisutil.Connect(cfg)
	if redisClient == nil {
		log.Printf("⚠️ [Studio] Failed to connect to Redis")
	}

	log.Println("✅ [Studio] Service initialized")
	return &Service{
		supabase: supabaseClient,
		redis:    redisClient,
	}
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

// GenerateImage - 이미지 생성 (동기 방식 - Sandbox용)
func (s *Service) GenerateImage(ctx context.Context, req *StudioGenerateRequest) (*StudioGenerateResponse, error) {
	cfg := config.GetConfig()

	// 카테고리 검증
	if !common.IsValidCategory(req.Category) {
		return &StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Invalid category",
			ErrorCode:    common.ErrCodeInvalidCategory,
		}, nil
	}

	// 크레딧 확인
	credits, err := s.CheckUserCredits(ctx, req.UserID)
	if err != nil {
		log.Printf("⚠️ [Studio] Failed to check credits: %v", err)
		// 크레딧 확인 실패해도 계속 진행 (개발 환경)
	} else if credits < cfg.ImagePerPrice {
		return &StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Insufficient credits",
			ErrorCode:    common.ErrCodeUnauthorized,
		}, nil
	}

	// Aspect ratio 기본값
	aspectRatio := req.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "1:1"
	}

	log.Printf("🎨 [Studio] Generating image - category: %s, prompt: %s, images: %d, ratio: %s",
		req.Category, truncateString(req.Prompt, 50), len(req.ReferenceImages), aspectRatio)

	// Gemini API 호출 준비
	var parts []*genai.Part

	// 레퍼런스 이미지 추가
	for i, imgBase64 := range req.ReferenceImages {
		base64Data := imgBase64
		if idx := findBase64Start(imgBase64); idx > 0 {
			base64Data = imgBase64[idx:]
		}

		imageData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			log.Printf("⚠️ [Studio] Failed to decode image %d: %v", i, err)
			continue
		}

		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     imageData,
			},
		})
		log.Printf("📎 [Studio] Added reference image %d (%d bytes)", i+1, len(imageData))
	}

	// 카테고리별 프롬프트 생성
	prompt := BuildStudioPrompt(req.Prompt, req.Category, len(req.ReferenceImages))
	parts = append(parts, genai.NewPartFromText(prompt))

	// Content 생성
	content := &genai.Content{
		Parts: parts,
	}

	// Gemini API 호출
	log.Printf("📤 [Studio] Calling Gemini API for category: %s", req.Category)
	result, err := geminiretry.GenerateContentWithRetry(
		ctx,
		cfg.GeminiAPIKey,
		cfg.GeminiModel,
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			ImageConfig: &genai.ImageConfig{
				AspectRatio: aspectRatio,
			},
			Temperature: floatPtr(0.7), // 더 창의적인 이미지 생성을 위해
		},
	)
	if err != nil {
		log.Printf("❌ [Studio] Gemini API error: %v", err)
		return &StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Image generation failed",
			ErrorCode:    common.ErrCodeInternalError,
		}, err
	}

	// 응답에서 이미지 추출
	if len(result.Candidates) == 0 {
		return &StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "No image generated",
			ErrorCode:    common.ErrCodeInternalError,
		}, fmt.Errorf("no candidates in response")
	}

	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				imageData := part.InlineData.Data
				log.Printf("✅ [Studio] Image generated: %d bytes", len(imageData))

				// Storage에 업로드 및 Attach 레코드 생성
				filePath, fileSize, err := s.UploadImageToStorage(ctx, imageData, req.UserID)
				if err != nil {
					log.Printf("⚠️ [Studio] Failed to upload image: %v", err)
					// 업로드 실패해도 Base64로 반환
					return &StudioGenerateResponse{
						Success:     true,
						JobID:       uuid.New().String(),
						ImageBase64: base64.StdEncoding.EncodeToString(imageData),
					}, nil
				}

				attachID, err := s.CreateAttachRecord(ctx, filePath, fileSize)
				if err != nil {
					log.Printf("⚠️ [Studio] Failed to create attach record: %v", err)
				}

				// 크레딧 차감
				if err := s.DeductCredits(ctx, req.UserID, attachID); err != nil {
					log.Printf("⚠️ [Studio] Failed to deduct credits: %v", err)
				}

				// 이미지 URL 생성
				imageURL := cfg.SupabaseStorageBaseURL + filePath

				return &StudioGenerateResponse{
					Success:     true,
					JobID:       uuid.New().String(),
					ImageURL:    imageURL,
					ImageBase64: base64.StdEncoding.EncodeToString(imageData),
					AttachID:    attachID,
				}, nil
			}
		}
	}

	return &StudioGenerateResponse{
		Success:      false,
		ErrorMessage: "No image data in response",
		ErrorCode:    common.ErrCodeInternalError,
	}, fmt.Errorf("no image data in response")
}

// UploadImageToStorage - Supabase Storage에 이미지 업로드 (WebP 변환)
func (s *Service) UploadImageToStorage(ctx context.Context, imageData []byte, userID string) (string, int64, error) {
	cfg := config.GetConfig()

	// PNG를 WebP로 변환
	webpData, err := s.ConvertPNGToWebP(imageData, 90.0)
	if err != nil {
		log.Printf("⚠️ [Studio] WebP conversion failed, using original: %v", err)
		webpData = imageData
	}

	// 파일명 생성
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	randomID := rand.Intn(999999)
	fileName := fmt.Sprintf("sandbox_%d_%d.webp", timestamp, randomID)

	// 파일 경로 생성
	filePath := fmt.Sprintf("sandbox-images/user-%s/%s", userID, fileName)

	log.Printf("📤 [Studio] Uploading image to storage: %s", filePath)

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
	log.Printf("✅ [Studio] Image uploaded: %s (%d bytes)", filePath, webpSize)
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
	log.Printf("💾 [Studio] Creating attach record: %s", filePath)

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
	log.Printf("✅ [Studio] Attach record created: ID=%d", attachID)

	return attachID, nil
}

// DeductCredits - 크레딧 차감 (개인/조직 크레딧 지원)
func (s *Service) DeductCredits(ctx context.Context, userID string, attachID int) error {
	cfg := config.GetConfig()
	creditsToDeduct := cfg.ImagePerPrice

	// userID로 org_id 조회
	orgID, err := s.GetUserOrganization(ctx, userID)
	if err != nil {
		log.Printf("⚠️ [Studio] Failed to get user organization: %v", err)
	}

	var orgIDPtr *string
	if orgID != "" {
		orgIDPtr = &orgID
	}

	// 조직 크레딧인지 개인 크레딧인지 구분
	isOrgCredit := org.ShouldUseOrgCredit(s.supabase, orgIDPtr)

	if isOrgCredit {
		log.Printf("💰 [Studio] Deducting ORGANIZATION credits: OrgID=%s, User=%s, Amount=%d", orgID, userID, creditsToDeduct)
	} else {
		log.Printf("💰 [Studio] Deducting PERSONAL credits: User=%s, Amount=%d", userID, creditsToDeduct)
	}

	var currentCredits int
	var newBalance int

	if isOrgCredit {
		// 조직 크레딧 차감
		var orgs []struct {
			OrgCredit int64 `json:"org_credit"`
		}
		data, _, err := s.supabase.From("quel_organization").
			Select("org_credit", "", false).
			Eq("org_id", orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to fetch org credits: %w", err)
		}

		if err := json.Unmarshal(data, &orgs); err != nil {
			return fmt.Errorf("failed to parse org data: %w", err)
		}

		if len(orgs) == 0 {
			return fmt.Errorf("organization not found: %s", orgID)
		}

		currentCredits = int(orgs[0].OrgCredit)
		newBalance = currentCredits - creditsToDeduct

		_, _, err = s.supabase.From("quel_organization").
			Update(map[string]interface{}{
				"org_credit": newBalance,
			}, "", "").
			Eq("org_id", orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to deduct org credits: %w", err)
		}
	} else {
		// 개인 크레딧 차감
		currentCredits, err = s.CheckUserCredits(ctx, userID)
		if err != nil {
			return err
		}

		newBalance = currentCredits - creditsToDeduct

		_, _, err = s.supabase.From("quel_member").
			Update(map[string]interface{}{
				"quel_member_credit": newBalance,
			}, "", "").
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to deduct credits: %w", err)
		}
	}

	log.Printf("💰 [Studio] Credit balance: %d → %d (-%d)", currentCredits, newBalance, creditsToDeduct)

	// 트랜잭션 기록
	transactionData := map[string]interface{}{
		"user_id":          userID,
		"transaction_type": "DEDUCT",
		"amount":           -creditsToDeduct,
		"balance_after":    newBalance,
		"description":      "Studio Image Generation",
		"api_provider":     "gemini",
		"attach_idx":       attachID,
	}

	if isOrgCredit {
		transactionData["org_id"] = orgID
		transactionData["used_by_member_id"] = userID
	}

	_, _, err = s.supabase.From("quel_credits").
		Insert(transactionData, false, "", "", "").
		Execute()

	if err != nil {
		log.Printf("⚠️ [Studio] Failed to record transaction: %v", err)
	}

	log.Printf("✅ [Studio] Credits deducted: %d credits from user %s", creditsToDeduct, userID)
	return nil
}

// GetUserOrganization - 사용자의 조직 ID 조회
func (s *Service) GetUserOrganization(ctx context.Context, userID string) (string, error) {
	var members []struct {
		OrgID string `json:"org_id"`
	}

	data, _, err := s.supabase.From("quel_organization_member").
		Select("org_id", "", false).
		Eq("member_id", userID).
		Execute()

	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(data, &members); err != nil {
		return "", err
	}

	if len(members) > 0 {
		return members[0].OrgID, nil
	}

	return "", nil
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

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

// AnalyzeImage - 이미지 분석하여 상세 프롬프트 추출 (레시피 생성용)
func (s *Service) AnalyzeImage(ctx context.Context, req *StudioAnalyzeRequest) (*StudioAnalyzeResponse, error) {
	cfg := config.GetConfig()
	log.Printf("🔍 [Studio] Analyzing image for recipe - category: %s", req.Category)

	// 이미지 데이터 준비
	var parts []*genai.Part

	// 이미지 URL 또는 Base64 처리
	if req.ImageURL != "" {
		var imageData []byte
		var err error

		if findBase64Start(req.ImageURL) > 0 {
			// Base64 이미지
			base64Data := req.ImageURL[findBase64Start(req.ImageURL):]
			imageData, err = base64.StdEncoding.DecodeString(base64Data)
			if err != nil {
				log.Printf("❌ [Studio] Failed to decode base64 image: %v", err)
				return &StudioAnalyzeResponse{
					Success:      false,
					ErrorMessage: "Failed to decode image",
				}, err
			}
		} else if len(req.ImageURL) > 100 && !hasHTTPPrefix(req.ImageURL) {
			// Raw Base64 (prefix 없이)
			imageData, err = base64.StdEncoding.DecodeString(req.ImageURL)
			if err != nil {
				log.Printf("❌ [Studio] Failed to decode raw base64: %v", err)
				return &StudioAnalyzeResponse{
					Success:      false,
					ErrorMessage: "Failed to decode image",
				}, err
			}
		} else {
			// HTTP URL - 이미지 다운로드
			resp, err := http.Get(req.ImageURL)
			if err != nil {
				log.Printf("❌ [Studio] Failed to fetch image: %v", err)
				return &StudioAnalyzeResponse{
					Success:      false,
					ErrorMessage: "Failed to fetch image",
				}, err
			}
			defer resp.Body.Close()

			imageData, err = io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("❌ [Studio] Failed to read image: %v", err)
				return &StudioAnalyzeResponse{
					Success:      false,
					ErrorMessage: "Failed to read image",
				}, err
			}
		}

		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     imageData,
			},
		})
		log.Printf("📎 [Studio] Image loaded for analysis (%d bytes)", len(imageData))
	}

	// 분석 프롬프트 생성
	analysisPrompt := buildAnalysisPrompt(req.Category, req.OriginalPrompt)
	parts = append(parts, genai.NewPartFromText(analysisPrompt))

	// Content 생성
	content := &genai.Content{
		Parts: parts,
	}

	// Gemini API 호출
	log.Printf("📤 [Studio] Calling Gemini API for image analysis")
	result, err := geminiretry.GenerateContentWithRetry(
		ctx,
		cfg.GeminiAPIKey,
		"gemini-2.0-flash", // 분석용은 빠른 모델 사용
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			Temperature: floatPtr(0.3), // 분석은 일관성 있게
		},
	)
	if err != nil {
		log.Printf("❌ [Studio] Gemini API error: %v", err)
		return &StudioAnalyzeResponse{
			Success:      false,
			ErrorMessage: "Image analysis failed",
		}, err
	}

	// 응답에서 텍스트 추출
	if len(result.Candidates) == 0 {
		return &StudioAnalyzeResponse{
			Success:      false,
			ErrorMessage: "No analysis result",
		}, fmt.Errorf("no candidates in response")
	}

	var analyzedPrompt string
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				analyzedPrompt = part.Text
				break
			}
		}
	}

	if analyzedPrompt == "" {
		return &StudioAnalyzeResponse{
			Success:      false,
			ErrorMessage: "No prompt extracted",
		}, fmt.Errorf("no text in response")
	}

	log.Printf("✅ [Studio] Image analyzed: %s", truncateString(analyzedPrompt, 100))

	return &StudioAnalyzeResponse{
		Success: true,
		Prompt:  analyzedPrompt,
	}, nil
}

// buildAnalysisPrompt - 이미지 분석용 프롬프트 생성
func buildAnalysisPrompt(category string, originalPrompt string) string {
	categoryContext := ""
	switch category {
	case "fashion":
		categoryContext = "fashion photography, clothing, style, pose, lighting"
	case "beauty":
		categoryContext = "beauty photography, makeup, skincare, cosmetics"
	case "eats":
		categoryContext = "food photography, cuisine, plating, ingredients"
	case "cinema":
		categoryContext = "cinematic photography, mood, lighting, composition"
	case "cartoon":
		categoryContext = "illustration style, character design, colors, art style"
	default:
		categoryContext = "commercial photography"
	}

	prompt := fmt.Sprintf(`Analyze this image and create a detailed prompt that could recreate a similar image.

CONTEXT:
- Category: %s
- Original user prompt: %s

TASK:
Generate a detailed, concise prompt (2-3 sentences) that captures:
1. Main subject/object description
2. Style, lighting, and mood
3. Key visual elements and composition

OUTPUT FORMAT:
Return ONLY the prompt text, no explanations or labels. The prompt should be in English and suitable for image generation.

Example output format:
"A woman wearing an elegant red silk evening gown, studio lighting with soft shadows, fashion editorial style, full body shot, front view"`, categoryContext, originalPrompt)

	return prompt
}

// hasHTTPPrefix - URL이 http로 시작하는지 확인
func hasHTTPPrefix(s string) bool {
	return len(s) >= 4 && (s[:4] == "http" || s[:5] == "https")
}
