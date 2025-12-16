package fashion

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // JPEG 디코더 등록
	"image/png"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gen2brain/webp"
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

// ImageCategories - 카테고리별 이미지 분류 구조체
type ImageCategories struct {
	Model       []byte   // 모델 이미지 (최대 1장)
	Clothing    [][]byte // 의류 이미지 배열 (top, pants, outer)
	Accessories [][]byte // 악세사리 이미지 배열 (shoes, bag, accessory)
	Background  []byte   // 배경 이미지 (최대 1장)
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ Failed to create Supabase client: %v", err)
		return nil
	}

	// Genai 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("❌ Failed to create Genai client: %v", err)
		return nil
	}

	// Redis 클라이언트 초기화
	redisClient := redisutil.Connect(cfg)
	if redisClient == nil {
		log.Printf("⚠️ Failed to connect to Redis - cancel feature will be disabled")
	}

	log.Println("✅ Supabase and Genai clients initialized")
	return &Service{
		supabase:    supabaseClient,
		genaiClient: genaiClient,
		redis:       redisClient,
	}
}

// IsJobCancelled - Job 취소 여부 확인
func (s *Service) IsJobCancelled(jobID string) bool {
	if s.redis == nil {
		return false
	}
	return redisutil.IsJobCancelled(s.redis, jobID)
}

// FetchJobFromSupabase - Supabase에서 Job 데이터 조회
func (s *Service) FetchJobFromSupabase(jobID string) (*model.ProductionJob, error) {
	log.Printf("🔍 Fetching job from Supabase: %s", jobID)

	var jobs []model.ProductionJob

	// Supabase에서 Job 조회
	data, _, err := s.supabase.From("quel_production_jobs").
		Select("*", "exact", false).
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return nil, fmt.Errorf("failed to query Supabase: %w", err)
	}

	// JSON 파싱
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	job := &jobs[0]
	log.Printf("✅ Job fetched successfully: %s (status: %s, total_images: %d)",
		job.JobID, job.JobStatus, job.TotalImages)

	return job, nil
}

// UpdateJobStatus - Job 상태 업데이트
func (s *Service) UpdateJobStatus(ctx context.Context, jobID string, status string) error {
	log.Printf("📝 Updating job %s status to: %s", jobID, status)

	updateData := map[string]interface{}{
		"job_status": status,
		"updated_at": "now()",
	}

	if status == model.StatusProcessing {
		updateData["started_at"] = "now()"
	} else if status == model.StatusCompleted || status == model.StatusFailed {
		updateData["completed_at"] = "now()"
	}

	_, _, err := s.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	log.Printf("✅ Job %s status updated to: %s", jobID, status)
	return nil
}

// FetchAttachInfo - quel_attach 테이블에서 파일 정보 조회
func (s *Service) FetchAttachInfo(attachID int) (*model.Attach, error) {
	log.Printf("🔍 Fetching attach info: %d", attachID)

	var attaches []model.Attach

	// Supabase에서 Attach 조회
	data, _, err := s.supabase.From("quel_attach").
		Select("*", "exact", false).
		Eq("attach_id", fmt.Sprintf("%d", attachID)).
		Execute()

	if err != nil {
		return nil, fmt.Errorf("failed to query quel_attach: %w", err)
	}

	// JSON 파싱
	if err := json.Unmarshal(data, &attaches); err != nil {
		return nil, fmt.Errorf("failed to parse attach response: %w", err)
	}

	if len(attaches) == 0 {
		return nil, fmt.Errorf("attach not found: %d", attachID)
	}

	attach := &attaches[0]

	// 실제 경로 값 출력
	var pathStr string
	if attach.AttachFilePath != nil {
		pathStr = *attach.AttachFilePath
	} else if attach.AttachDirectory != nil {
		pathStr = *attach.AttachDirectory
	} else {
		pathStr = "null"
	}

	log.Printf("✅ Attach info fetched: ID=%d, Path=%s", attach.AttachID, pathStr)

	return attach, nil
}

// DownloadImageFromStorage - Supabase Storage에서 이미지 다운로드
func (s *Service) DownloadImageFromStorage(attachID int) ([]byte, error) {
	cfg := config.GetConfig()

	// 1. quel_attach에서 파일 경로 조회
	attach, err := s.FetchAttachInfo(attachID)
	if err != nil {
		return nil, err
	}

	// 2. attach_file_path 확인 (없으면 attach_directory 사용)
	var filePath string
	if attach.AttachFilePath != nil && *attach.AttachFilePath != "" {
		filePath = *attach.AttachFilePath
		log.Printf("🔍 Using attach_file_path: %s", filePath)
	} else if attach.AttachDirectory != nil && *attach.AttachDirectory != "" {
		filePath = *attach.AttachDirectory
		log.Printf("🔍 Using attach_directory: %s", filePath)
	} else {
		log.Printf("❌ DB values - FilePath: %v, Directory: %v", attach.AttachFilePath, attach.AttachDirectory)
		return nil, fmt.Errorf("no file path found for attach_id: %d", attachID)
	}

	// 2.5. uploads/ 폴더가 누락된 경우 자동 추가 (upload-로 시작하는 경우)
	if len(filePath) > 0 && filePath[0] != '/' &&
		len(filePath) >= 7 && filePath[:7] == "upload-" {
		filePath = "uploads/" + filePath
		log.Printf("🔧 Auto-fixed path to include uploads/ folder: %s", filePath)
	}

	// 3. Full URL 생성
	fullURL := cfg.SupabaseStorageBaseURL + filePath
	log.Printf("📥 Downloading image from: %s", fullURL)
	log.Printf("   🔗 Base URL: %s", cfg.SupabaseStorageBaseURL)
	log.Printf("   📁 File Path: %s", filePath)

	// 4. HTTP GET으로 직접 다운로드 (30초 타임아웃)
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	httpResp, err := client.Get(fullURL)
	if err != nil {
		log.Printf("❌ HTTP GET failed: %v", err)
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		log.Printf("❌ Download failed - Status: %d, URL: %s", httpResp.StatusCode, fullURL)
		log.Printf("❌ Response body: %s", string(body))
		return nil, fmt.Errorf("failed to download image: status %d, body: %s", httpResp.StatusCode, string(body))
	}

	// 5. 이미지 데이터 읽기
	imageData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	log.Printf("✅ Image downloaded successfully: %d bytes", len(imageData))
	return imageData, nil
}

// ConvertImageToBase64 - 이미지 바이너리를 base64로 변환
func (s *Service) ConvertImageToBase64(imageData []byte) string {
	base64Str := base64.StdEncoding.EncodeToString(imageData)
	log.Printf("🔄 Image converted to base64: %d chars (preview: %s...)",
		len(base64Str),
		base64Str[:min(50, len(base64Str))])
	return base64Str
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ConvertPNGToWebP - PNG 바이너리를 WebP로 변환
func (s *Service) ConvertPNGToWebP(pngData []byte, quality float32) ([]byte, error) {
	log.Printf("🔄 Converting PNG to WebP (quality: %.1f)", quality)

	// PNG 디코딩
	pngReader := bytes.NewReader(pngData)
	img, err := png.Decode(pngReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	// WebP 인코딩 (gen2brain/webp 사용)
	var webpBuffer bytes.Buffer
	err = webp.Encode(&webpBuffer, img, webp.Options{Quality: int(quality)})
	if err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %w", err)
	}

	webpData := webpBuffer.Bytes()

	log.Printf("✅ PNG converted to WebP: %d bytes → %d bytes (%.1f%% reduction)",
		len(pngData), len(webpData),
		float64(len(pngData)-len(webpData))/float64(len(pngData))*100)

	return webpData, nil
}

// UpdateProductionPhotoStatus - Production Photo 상태 업데이트
func (s *Service) UpdateProductionPhotoStatus(ctx context.Context, productionID string, status string) error {
	log.Printf("📝 Updating production %s status to: %s", productionID, status)

	updateData := map[string]interface{}{
		"production_status": status,
	}

	_, _, err := s.supabase.From("quel_production_photo").
		Update(updateData, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update production status: %w", err)
	}

	log.Printf("✅ Production %s status updated to: %s", productionID, status)
	return nil
}

// GenerateImageWithGemini - Gemini API로 이미지 생성
func (s *Service) GenerateImageWithGemini(ctx context.Context, base64Image string, prompt string, aspectRatio string) (string, error) {
	cfg := config.GetConfig()

	// aspect-ratio 기본값 처리
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	log.Printf("🎨 Calling Gemini API (model: %s) with prompt length: %d, aspect-ratio: %s", cfg.GeminiModel, len(prompt), aspectRatio)

	// Base64 디코딩
	imageData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	// Content 생성
	content := &genai.Content{
		Parts: []*genai.Part{
			genai.NewPartFromText(prompt + "\n\nPlease generate 1 different variation of this image."),
			genai.NewPartFromBytes(imageData, "image/png"),
		},
	}

	// API 호출 (새 google.golang.org/genai 패키지 사용)
	log.Printf("📤 Sending request to Gemini API with aspect-ratio: %s", aspectRatio)
	result, err := s.genaiClient.Models.GenerateContent(
		ctx,
		cfg.GeminiModel,
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			ImageConfig: &genai.ImageConfig{
				AspectRatio: aspectRatio,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("Gemini API call failed: %w", err)
	}

	// 응답 처리
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			// InlineData 확인 (이미지는 InlineData로 반환됨)
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				log.Printf("✅ Received image from Gemini: %d bytes", len(part.InlineData.Data))
				// Base64로 인코딩하여 반환
				return base64.StdEncoding.EncodeToString(part.InlineData.Data), nil
			}
		}
	}

	return "", fmt.Errorf("no image data in response")
}

// mergeImages - 여러 이미지를 Grid 방식으로 병합 (resize 없음, 원본 그대로)
func mergeImages(images [][]byte, aspectRatio string) ([]byte, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("no images to merge")
	}

	if len(images) == 1 {
		// 단일 이미지는 원본 그대로 반환
		log.Printf("✅ Single image - returning original")
		return images[0], nil
	}

	// 이미지 디코드 (WebP, PNG, JPEG 자동 감지)
	decodedImages := []image.Image{}
	for i, imgData := range images {
		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			log.Printf("⚠️  Failed to decode image %d: %v", i, err)
			continue
		}
		log.Printf("🔍 Decoded image %d format: %s", i, format)
		decodedImages = append(decodedImages, img)
	}

	if len(decodedImages) == 0 {
		return nil, fmt.Errorf("no valid images to merge")
	}

	// Grid 방식으로 배치 (2x2, 2x3 등)
	numImages := len(decodedImages)
	cols := int(math.Ceil(math.Sqrt(float64(numImages))))      // 열 개수
	rows := int(math.Ceil(float64(numImages) / float64(cols))) // 행 개수

	// 각 셀의 최대 너비/높이 계산
	maxCellWidth := 0
	maxCellHeight := 0
	for _, img := range decodedImages {
		bounds := img.Bounds()
		if bounds.Dx() > maxCellWidth {
			maxCellWidth = bounds.Dx()
		}
		if bounds.Dy() > maxCellHeight {
			maxCellHeight = bounds.Dy()
		}
	}

	// 전체 그리드 크기
	totalWidth := cols * maxCellWidth
	totalHeight := rows * maxCellHeight

	// 새 이미지 생성
	merged := image.NewRGBA(image.Rect(0, 0, totalWidth, totalHeight))

	// Grid에 이미지 배치
	for idx, img := range decodedImages {
		row := idx / cols
		col := idx % cols

		x := col * maxCellWidth
		y := row * maxCellHeight

		bounds := img.Bounds()
		// 중앙 정렬
		xOffset := x + (maxCellWidth-bounds.Dx())/2
		yOffset := y + (maxCellHeight-bounds.Dy())/2

		draw.Draw(merged,
			image.Rect(xOffset, yOffset, xOffset+bounds.Dx(), yOffset+bounds.Dy()),
			img, image.Point{0, 0}, draw.Src)
	}

	log.Printf("✅ Merged %d images into %dx%d grid (%dx%d total)", len(decodedImages), rows, cols, totalWidth, totalHeight)

	// 1:1 비율이 아닌 경우만 aspect-ratio에 맞게 리사이즈
	var finalImage image.Image = merged
	if aspectRatio != "1:1" {
		// aspect-ratio에 따른 목표 크기 설정
		var targetWidth, targetHeight int
		switch aspectRatio {
		case "16:9":
			targetWidth, targetHeight = 1344, 768
		case "9:16":
			targetWidth, targetHeight = 768, 1344
		case "4:3":
			targetWidth, targetHeight = 1152, 896
		case "3:4":
			targetWidth, targetHeight = 896, 1152
		default:
			targetWidth, targetHeight = 1024, 1024
		}

		finalImage = resizeImage(merged, targetWidth, targetHeight)
		log.Printf("✅ Resized merged grid to %dx%d (aspect-ratio: %s)", targetWidth, targetHeight, aspectRatio)
	} else {
		log.Printf("✅ 1:1 aspect-ratio - skipping resize, keeping original grid size")
	}

	// PNG 인코딩
	var buf bytes.Buffer
	if err := png.Encode(&buf, finalImage); err != nil {
		return nil, fmt.Errorf("failed to encode merged image: %w", err)
	}

	return buf.Bytes(), nil
}

// resizeImage - 이미지를 지정된 크기로 resize (비율 유지하며 fit, 투명 배경)
func resizeImage(src image.Image, targetWidth, targetHeight int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// 비율 계산
	scaleX := float64(targetWidth) / float64(srcWidth)
	scaleY := float64(targetHeight) / float64(srcHeight)
	scale := math.Min(scaleX, scaleY)

	// 스케일된 크기 계산
	newWidth := int(float64(srcWidth) * scale)
	newHeight := int(float64(srcHeight) * scale)

	// 새 이미지 생성 (목표 크기, 검은 배경)
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// 중앙 정렬을 위한 오프셋 계산
	xOffset := (targetWidth - newWidth) / 2
	yOffset := (targetHeight - newHeight) / 2

	// Nearest Neighbor 방식으로 리사이즈
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			dst.Set(x+xOffset, y+yOffset, src.At(srcX, srcY))
		}
	}

	return dst
}

// generateDynamicPrompt - 상황별 동적 프롬프트 생성
// shotType: "tight", "middle", "full" (기본값: "full")
func generateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string, shotType string) string {
	// 케이스 분석을 위한 변수 정의
	hasModel := categories.Model != nil
	hasClothing := len(categories.Clothing) > 0
	hasAccessories := len(categories.Accessories) > 0
	hasProducts := hasClothing || hasAccessories
	hasBackground := categories.Background != nil

	// shotType 기본값
	if shotType == "" {
		shotType = "full"
	}

	// 케이스별 메인 지시사항
	var mainInstruction string
	if hasModel {
		// 모델 있음 → 패션 에디토리얼 (샷 타입별 분기)
		switch shotType {
		case "tight":
			mainInstruction = "[FASHION EDITORIAL - TIGHT SHOT / CLOSE-UP]\n" +
				"You are a fashion photographer shooting a CLOSE-UP portrait.\n" +
				"This is SOLO FASHION MODEL photography - ONLY ONE PERSON in the frame.\n\n" +
				"⚠️ CRITICAL FRAMING - TIGHT SHOT:\n" +
				"🚨 FRAME FROM SHOULDERS UP ONLY\n" +
				"🚨 CROP BELOW THE SHOULDERS - do NOT show chest/torso\n" +
				"🚨 Focus on FACE and SHOULDERS only\n" +
				"🚨 DO NOT show waist, arms below shoulders, or any lower body\n\n" +
				"Create ONE photorealistic photograph:\n" +
				"• ONLY ONE MODEL - solo fashion shoot\n" +
				"• TIGHT CLOSE-UP - shoulders and head only\n" +
				"• Face is the main focus\n" +
				"• Use the EXACT background from the reference image\n\n"
		case "middle":
			mainInstruction = "[FASHION EDITORIAL - MEDIUM SHOT / WAIST-UP]\n" +
				"You are a fashion photographer shooting a MEDIUM portrait.\n" +
				"This is SOLO FASHION MODEL photography - ONLY ONE PERSON in the frame.\n\n" +
				"⚠️ CRITICAL FRAMING - MEDIUM SHOT:\n" +
				"🚨 FRAME FROM WAIST UP ONLY\n" +
				"🚨 CROP AT THE WAIST - do NOT show hips, legs, or feet\n" +
				"🚨 Show upper body, arms, and head\n" +
				"🚨 DO NOT show anything below the waist\n\n" +
				"Create ONE photorealistic photograph:\n" +
				"• ONLY ONE MODEL - solo fashion shoot\n" +
				"• MEDIUM SHOT - waist up only, showing upper body outfit\n" +
				"• Show clothing details on upper body\n" +
				"• Use the EXACT background from the reference image\n\n"
		default: // "full"
			mainInstruction = "[FASHION EDITORIAL - FULL BODY SHOT]\n" +
				"You are a fashion photographer shooting an editorial campaign.\n" +
				"This is SOLO FASHION MODEL photography - ONLY ONE PERSON in the frame.\n" +
				"The PERSON is the HERO - their natural proportions are SACRED.\n\n" +
				"⚠️ CRITICAL FRAMING - FULL BODY:\n" +
				"🚨 ENTIRE BODY from HEAD to TOE must be visible\n" +
				"🚨 FEET MUST BE VISIBLE - both feet completely in frame\n" +
				"🚨 DO NOT crop at ankles, calves, or knees\n\n" +
				"Create ONE photorealistic photograph:\n" +
				"• ONLY ONE MODEL - solo fashion shoot\n" +
				"• FULL BODY SHOT - model's ENTIRE body from head to TOE visible\n" +
				"• FEET MUST BE VISIBLE - both feet and shoes completely in frame\n" +
				"• STRONG POSTURE - elongated body lines, poised stance\n" +
				"• The model wears ALL clothing and accessories\n" +
				"• Use the EXACT background from the reference image\n\n"
		}
	} else if hasProducts {
		// 프로덕트만 → 프로덕트 포토그래피
		mainInstruction = "[PRODUCT PHOTOGRAPHER]\n" +
			"You are a product photographer creating still life.\n" +
			"The PRODUCTS are the STARS.\n" +
			"⚠️ CRITICAL: NO people or models in this shot - products only.\n" +
			"⚠️ CRITICAL: Do NOT invent new items or props. Show ONLY the items provided in the reference images. The count and types must match exactly.\n" +
			"⚠️ IF ONLY ONE PRODUCT is provided: show exactly that single item by itself on a clean surface/background. Do NOT add shoes, hats, sunglasses, jewelry, watches, wallets, chains, papers, books, boxes, or any extra objects.\n\n" +
			"Create ONE photorealistic photograph:\n" +
			"• Artistic arrangement of all items\n" +
			"• Good lighting that highlights textures\n" +
			"• Use the EXACT background from the reference if provided\n\n"
	} else {
		// 배경만 → 환경 포토그래피
		mainInstruction = "[ENVIRONMENTAL PHOTOGRAPHER]\n" +
			"You are a photographer capturing atmosphere.\n" +
			"⚠️ CRITICAL: NO people, models, or products in this shot - environment only.\n\n" +
			"Create ONE photorealistic photograph of the referenced environment.\n\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명
	if categories.Model != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (MODEL - FACE/BODY ONLY): ⚠️ CRITICAL: You MUST use this EXACT person's FACE and BODY only. Copy this person's face EXACTLY - same ethnicity, same facial structure, same skin tone, same bone structure, same eyes, same nose, same lips, same hair color, same hair style. DO NOT change or replace with a different person. DO NOT change the face to look more Western or more Asian. The model's identity must be 100%% preserved.\n\n⚠️ IGNORE FROM THIS MODEL IMAGE:\n❌ IGNORE the background in this model photo - use ONLY the separate BACKGROUND reference image\n❌ IGNORE the clothing/outfit in this model photo - use ONLY the separate CLOTHING reference images\n❌ This model image is ONLY for face and body reference - NOTHING else", imageIndex))
		imageIndex++
	}

	if len(categories.Clothing) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (CLOTHING): ALL visible garments - tops, bottoms, dresses, outerwear, layers. The person MUST wear EVERY piece shown here", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (ACCESSORIES): ALL items - shoes, bags, hats, glasses, jewelry, watches. Use ONLY the items actually visible in the reference; DO NOT invent or add any extra items. If only one item is visible, show exactly that single item alone.", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (BACKGROUND - MUST USE EXACTLY): ⚠️ CRITICAL: You MUST use this EXACT background. If it is a white/gray studio, use a WHITE/GRAY STUDIO. If it is an outdoor location, use that EXACT outdoor location. DO NOT invent a different background. The background must match the reference image 100%%.", imageIndex))
		imageIndex++
	}

	// 구성 지시사항
	var compositionInstruction string

	// 케이스 1: 모델 이미지가 있는 경우
	if hasModel {
		compositionInstruction = "\n[FASHION EDITORIAL COMPOSITION]\n" +
			"Generate ONE photorealistic photograph showing the referenced model wearing the complete outfit."
	} else if hasProducts {
		// 케이스 2: 모델 없이 의상/액세서리만 → 프로덕트 샷
		compositionInstruction = "\n[PRODUCT PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic product photograph showcasing the clothing and accessories as OBJECTS.\n" +
			"⚠️ DO NOT add any people, models, or human figures.\n" +
			"⚠️ DO NOT add any extra products, props, or accessories that are not in the references.\n" +
			"⚠️ The number of products in the shot must match the references exactly. If only one product is referenced, show exactly that single item by itself on a clean surface.\n"

		if hasBackground {
			compositionInstruction += "The products are placed naturally within the referenced environment."
		} else {
			compositionInstruction += "Create a studio product shot with professional lighting."
		}
	} else if hasBackground {
		// 케이스 3: 배경만 → 환경 사진
		compositionInstruction = "\n[ENVIRONMENTAL PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic photograph of the referenced environment.\n" +
			"⚠️ DO NOT add any people, models, or products to this scene."
	} else {
		// 케이스 4: 아무것도 없는 경우
		compositionInstruction = "\n[COMPOSITION]\n" +
			"Generate a high-quality photorealistic image based on the references provided."
	}

	// 배경 관련 지시사항 - 모델이 있을 때만 추가
	if hasModel && hasBackground {
		// 모델 + 배경 케이스 → 배경 레퍼런스에 집중
		compositionInstruction += " in the EXACT background from the reference image.\n\n" +
			"[BACKGROUND - MUST MATCH REFERENCE]\n" +
			"⚠️ CRITICAL: The background MUST match the reference image EXACTLY.\n" +
			"⚠️ If the reference shows a WHITE STUDIO, use a WHITE STUDIO.\n" +
			"⚠️ If the reference shows a GRAY STUDIO, use a GRAY STUDIO.\n" +
			"⚠️ If the reference shows an outdoor location, use that EXACT location.\n" +
			"⚠️ DO NOT invent backgrounds. DO NOT add locations not in the reference.\n\n" +
			"[SUBJECT INTEGRATION]\n" +
			"✓ Place the subject naturally in the referenced background\n" +
			"✓ Lighting must match the background reference\n" +
			"✓ Natural shadows consistent with the background\n" +
			"✓ The subject and background must look like ONE unified photograph"
	} else if hasModel && !hasBackground {
		// 모델만 있고 배경 없음 → 기본 스튜디오
		compositionInstruction += " in a clean studio setting with professional lighting."
	}

	// 공통 금지사항
	commonForbidden := "\n\n[CRITICAL: FORBIDDEN]\n\n" +
		"⚠️ NO SPLIT/DUAL COMPOSITION:\n" +
		"❌ NO vertical dividing lines\n" +
		"❌ NO left-right split layouts\n" +
		"❌ NO duplicate subject on both sides\n" +
		"❌ NO grid or collage\n" +
		"❌ ONE continuous scene only\n\n" +
		"⚠️ ONLY ONE PERSON:\n" +
		"❌ NO multiple models\n" +
		"❌ NO background people\n" +
		"❌ This is SOLO photography\n\n" +
		"[REQUIRED]:\n" +
		"✓ ONE single photograph\n" +
		"✓ ONE unified moment\n" +
		"✓ Fill entire frame - NO empty margins\n" +
		"✓ Natural asymmetric composition\n"

	// 핵심 요구사항 - 케이스별로 다르게
	var criticalRules string
	if hasModel {
		// 모델 있는 케이스 - 샷 타입별 분기
		switch shotType {
		case "tight":
			criticalRules = commonForbidden + "\n[TIGHT SHOT REQUIREMENTS]\n" +
				"🎯 ONLY ONE MODEL in the photograph\n" +
				"🎯 SHOULDERS UP ONLY - close-up framing\n" +
				"🎯 Use EXACT background from reference\n\n" +
				"[FORBIDDEN]\n" +
				"❌ SHOWING BODY BELOW SHOULDERS\n" +
				"❌ WRONG BACKGROUND - must match reference exactly\n" +
				"❌ Multiple people"
		case "middle":
			criticalRules = commonForbidden + "\n[MEDIUM SHOT REQUIREMENTS]\n" +
				"🎯 ONLY ONE MODEL in the photograph\n" +
				"🎯 WAIST UP ONLY - medium framing\n" +
				"🎯 Show upper body outfit details\n" +
				"🎯 Use EXACT background from reference\n\n" +
				"[FORBIDDEN]\n" +
				"❌ SHOWING LEGS OR FEET\n" +
				"❌ WRONG BACKGROUND - must match reference exactly\n" +
				"❌ Multiple people"
		default: // "full"
			criticalRules = commonForbidden + "\n[FULL BODY REQUIREMENTS]\n" +
				"🎯 ONLY ONE MODEL in the photograph\n" +
				"🎯 FULL BODY SHOT - head to TOE visible\n" +
				"🎯 FEET MUST BE VISIBLE - both feet in frame\n" +
				"🎯 ALL clothing and accessories worn\n" +
				"🎯 Use EXACT background from reference\n\n" +
				"[FORBIDDEN]\n" +
				"❌ CROPPED FEET - feet must be visible\n" +
				"❌ WRONG BACKGROUND - must match reference exactly\n" +
				"❌ Multiple people\n" +
				"❌ Distorted proportions"
		}
	} else if hasProducts {
		// 프로덕트 샷 케이스
		criticalRules = commonForbidden + "\n[PRODUCT REQUIREMENTS]\n" +
			"🎯 Showcase products beautifully\n" +
			"🎯 Good lighting\n" +
			"🎯 ALL items displayed clearly\n" +
			"🎯 Use EXACT background from reference\n\n" +
			"[FORBIDDEN]\n" +
			"❌ ANY people or models\n" +
			"❌ Products looking pasted\n" +
			"❌ Adding ANY extra items not present in the reference. If only one product reference is provided, show EXACTLY that single item alone."
	} else {
		// 배경만 있는 케이스
		criticalRules = commonForbidden + "\n[ENVIRONMENT REQUIREMENTS]\n" +
			"🎯 Capture the atmosphere of the location\n\n" +
			"[FORBIDDEN]\n" +
			"❌ DO NOT add people or products"
	}

	// aspect ratio별 추가 지시사항 (샷 타입 고려)
	var aspectRatioInstruction string
	if aspectRatio == "9:16" {
		if hasModel {
			switch shotType {
			case "tight":
				aspectRatioInstruction = "\n\n[9:16 VERTICAL - TIGHT SHOT]\n" +
					"✓ Close-up portrait framing\n" +
					"✓ SHOULDERS UP ONLY\n" +
					"✓ Use EXACT background from reference"
			case "middle":
				aspectRatioInstruction = "\n\n[9:16 VERTICAL - MEDIUM SHOT]\n" +
					"✓ WAIST UP framing\n" +
					"✓ Show upper body outfit\n" +
					"✓ Use EXACT background from reference"
			default:
				aspectRatioInstruction = "\n\n[9:16 VERTICAL - FULL BODY]\n" +
					"✓ Model's ENTIRE BODY from head to TOE must fit\n" +
					"✓ FEET MUST BE VISIBLE at bottom\n" +
					"✓ Leave space below feet\n" +
					"✓ Use EXACT background from reference"
			}
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL PRODUCT SHOT]\n" +
				"✓ Products arranged vertically\n" +
				"✓ Use EXACT background from reference"
		} else {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL SHOT]\n" +
				"✓ Use the HEIGHT to capture vertical elements"
		}
	} else if aspectRatio == "16:9" {
		if hasModel {
			switch shotType {
			case "tight":
				aspectRatioInstruction = "\n\n[16:9 WIDE - TIGHT SHOT]\n" +
					"✓ Close-up portrait in wide frame\n" +
					"✓ SHOULDERS UP ONLY - face centered\n" +
					"✓ Use EXACT background from reference"
			case "middle":
				aspectRatioInstruction = "\n\n[16:9 WIDE - MEDIUM SHOT]\n" +
					"✓ WAIST UP framing in wide format\n" +
					"✓ Subject positioned using rule of thirds\n" +
					"✓ Use EXACT background from reference"
			default:
				aspectRatioInstruction = "\n\n[16:9 WIDE - FULL BODY]\n" +
					"✓ Model's ENTIRE BODY from head to TOE must be visible\n" +
					"✓ FEET MUST BE VISIBLE at bottom\n" +
					"✓ Subject positioned using rule of thirds\n" +
					"✓ Use EXACT background from reference\n\n" +
					"⚠️ BACKGROUND RULE:\n" +
					"⚠️ If reference shows WHITE/GRAY STUDIO, use WHITE/GRAY STUDIO\n" +
					"⚠️ If reference shows outdoor location, use that EXACT location\n" +
					"⚠️ DO NOT invent locations not in reference"
			}
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[16:9 WIDE PRODUCT SHOT]\n" +
				"✓ Products positioned using the full width\n" +
				"✓ Use EXACT background from reference"
		} else {
			aspectRatioInstruction = "\n\n[16:9 WIDE SHOT]\n" +
				"✓ Use the full WIDTH to capture the environment"
		}
	} else {
		// 1:1 및 기타 비율
		if hasModel {
			switch shotType {
			case "tight":
				aspectRatioInstruction = "\n\n[SQUARE - TIGHT SHOT]\n" +
					"✓ Close-up portrait framing\n" +
					"✓ SHOULDERS UP ONLY\n" +
					"✓ Use EXACT background from reference"
			case "middle":
				aspectRatioInstruction = "\n\n[SQUARE - MEDIUM SHOT]\n" +
					"✓ WAIST UP framing\n" +
					"✓ Balanced composition\n" +
					"✓ Use EXACT background from reference"
			default:
				aspectRatioInstruction = "\n\n[SQUARE - FULL BODY]\n" +
					"✓ Model's ENTIRE BODY from head to TOE must fit\n" +
					"✓ FEET MUST BE VISIBLE at bottom\n" +
					"✓ Balanced composition\n" +
					"✓ Use EXACT background from reference"
			}
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[SQUARE PRODUCT SHOT]\n" +
				"✓ Balanced product arrangement\n" +
				"✓ Use EXACT background from reference"
		} else {
			aspectRatioInstruction = "\n\n[SQUARE SHOT]\n" +
				"✓ Balanced composition"
		}
	}

	// ⚠️ 최우선 지시사항 (샷 타입별 분기)
	var criticalHeader string
	switch shotType {
	case "tight":
		criticalHeader = "⚠️ CRITICAL REQUIREMENTS - TIGHT SHOT ⚠️\n\n" +
			"[MANDATORY - FRAMING]:\n" +
			"🚨 TIGHT SHOT = SHOULDERS UP ONLY\n" +
			"🚨 CROP BELOW SHOULDERS - NO chest, NO torso\n" +
			"🚨 FACE is the main subject\n\n" +
			"[MANDATORY - BACKGROUND]:\n" +
			"🚨 USE EXACT BACKGROUND FROM REFERENCE\n" +
			"🚨 If reference is WHITE STUDIO, use WHITE STUDIO\n" +
			"🚨 DO NOT invent outdoor/urban/nature locations\n\n" +
			"[FORBIDDEN]:\n" +
			"❌ NO full body - this is a CLOSE-UP\n" +
			"❌ NO waist or below showing\n" +
			"❌ NO split layouts, NO grid, NO collage\n" +
			"❌ NO multiple people\n\n"
	case "middle":
		criticalHeader = "⚠️ CRITICAL REQUIREMENTS - MEDIUM SHOT ⚠️\n\n" +
			"[MANDATORY - FRAMING]:\n" +
			"🚨 MEDIUM SHOT = WAIST UP ONLY\n" +
			"🚨 CROP AT WAIST - NO hips, NO legs, NO feet\n" +
			"🚨 Show upper body and outfit details\n\n" +
			"[MANDATORY - BACKGROUND]:\n" +
			"🚨 USE EXACT BACKGROUND FROM REFERENCE\n" +
			"🚨 If reference is WHITE STUDIO, use WHITE STUDIO\n" +
			"🚨 DO NOT invent outdoor/urban/nature locations\n\n" +
			"[FORBIDDEN]:\n" +
			"❌ NO full body - this is WAIST-UP only\n" +
			"❌ NO legs or feet showing\n" +
			"❌ NO split layouts, NO grid, NO collage\n" +
			"❌ NO multiple people\n\n"
	default: // "full"
		criticalHeader = "⚠️ CRITICAL REQUIREMENTS - FULL BODY ⚠️\n\n" +
			"[MANDATORY - FEET VISIBLE]:\n" +
			"🚨 BOTH FEET MUST APPEAR IN FRAME\n" +
			"🚨 DO NOT CROP AT ANKLES OR CALVES\n" +
			"🚨 FULL BODY means HEAD TO TOE\n\n" +
			"[MANDATORY - BACKGROUND]:\n" +
			"🚨 USE EXACT BACKGROUND FROM REFERENCE\n" +
			"🚨 If reference is WHITE STUDIO, use WHITE STUDIO\n" +
			"🚨 If reference is GRAY STUDIO, use GRAY STUDIO\n" +
			"🚨 DO NOT invent outdoor/urban/nature locations\n\n" +
			"[FORBIDDEN]:\n" +
			"❌ NO split layouts, NO grid, NO collage\n" +
			"❌ NO multiple people\n" +
			"❌ NO cropped feet\n" +
			"❌ NO wrong background\n\n"
	}

	// 최종 조합
	var finalPrompt string

	if userPrompt != "" {
		finalPrompt = criticalHeader + "[USER REQUEST]\n" + userPrompt + "\n\n"
	} else {
		finalPrompt = criticalHeader
	}

	// 카테고리별 스타일 가이드
	categoryStyleGuide := "\n\n[STYLE GUIDE]\n" +
		"Fashion photography style. Professional lighting. High-end editorial composition.\n\n" +
		"[TECHNICAL]\n" +
		"Fill entire frame. NO empty margins. NO letterboxing.\n"

	finalPrompt += mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + categoryStyleGuide + criticalRules + aspectRatioInstruction

	return finalPrompt
}

// GenerateImageWithGeminiMultiple - 카테고리별 이미지로 Gemini API 호출
// shotType: "tight", "middle", "full" (기본값: "full")
func (s *Service) GenerateImageWithGeminiMultiple(ctx context.Context, categories *ImageCategories, userPrompt string, aspectRatio string, shotType string) (string, error) {
	cfg := config.GetConfig()

	// aspect-ratio 기본값 처리
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	log.Printf("🎨 Calling Gemini API with categories - Model:%v, Clothing:%d, Accessories:%d, BG:%v",
		categories.Model != nil, len(categories.Clothing), len(categories.Accessories), categories.Background != nil)

	// 카테고리별 병합 및 resize
	var mergedClothing []byte
	var mergedAccessories []byte
	var err error

	if len(categories.Clothing) > 0 {
		mergedClothing, err = mergeImages(categories.Clothing, aspectRatio)
		if err != nil {
			return "", fmt.Errorf("failed to merge clothing images: %w", err)
		}
	}

	if len(categories.Accessories) > 0 {
		mergedAccessories, err = mergeImages(categories.Accessories, aspectRatio)
		if err != nil {
			return "", fmt.Errorf("failed to merge accessory images: %w", err)
		}
	}

	// Gemini Part 배열 구성
	var parts []*genai.Part

	// 순서: Model → Clothing → Accessories → Background
	if categories.Model != nil {
		// Model 이미지도 resize
		resizedModel, err := mergeImages([][]byte{categories.Model}, aspectRatio)
		if err != nil {
			return "", fmt.Errorf("failed to resize model image: %w", err)
		}
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     resizedModel,
			},
		})
		log.Printf("📎 Added Model image (resized)")
	}

	if mergedClothing != nil {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     mergedClothing,
			},
		})
		log.Printf("📎 Added Clothing image (merged from %d items)", len(categories.Clothing))
	}

	if mergedAccessories != nil {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     mergedAccessories,
			},
		})
		log.Printf("📎 Added Accessories image (merged from %d items)", len(categories.Accessories))
	}

	if categories.Background != nil {
		// Background 이미지도 resize
		resizedBG, err := mergeImages([][]byte{categories.Background}, aspectRatio)
		if err != nil {
			return "", fmt.Errorf("failed to resize background image: %w", err)
		}
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     resizedBG,
			},
		})
		log.Printf("📎 Added Background image (resized)")
	}

	// 동적 프롬프트 생성 (shotType 전달)
	dynamicPrompt := generateDynamicPrompt(categories, userPrompt, aspectRatio, shotType)
	parts = append(parts, genai.NewPartFromText(dynamicPrompt))

	log.Printf("📝 Generated dynamic prompt (%d chars)", len(dynamicPrompt))

	// Content 생성
	content := &genai.Content{
		Parts: parts,
	}

	// API 호출
	log.Printf("📤 Sending request to Gemini API with %d parts...", len(parts))
	result, err := s.genaiClient.Models.GenerateContent(
		ctx,
		cfg.GeminiModel,
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			ImageConfig: &genai.ImageConfig{
				AspectRatio: aspectRatio,
			},
			Temperature: floatPtr(0.45),
		},
	)
	if err != nil {
		return "", fmt.Errorf("Gemini API call failed: %w", err)
	}

	// 응답 처리
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				log.Printf("✅ Received image from Gemini: %d bytes", len(part.InlineData.Data))
				return base64.StdEncoding.EncodeToString(part.InlineData.Data), nil
			}
		}
	}

	return "", fmt.Errorf("no image data in response")
}

// floatPtr - float64를 *float32로 변환
func floatPtr(f float64) *float32 {
	f32 := float32(f)
	return &f32
}

// UploadImageToStorage - Supabase Storage에 이미지 업로드 (WebP 변환 포함)
func (s *Service) UploadImageToStorage(ctx context.Context, imageData []byte, userID string) (string, int64, error) {
	cfg := config.GetConfig()

	// PNG를 WebP로 변환 (quality: 90)
	webpData, err := s.ConvertPNGToWebP(imageData, 90.0)
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert PNG to WebP: %w", err)
	}

	// 파일명 생성 (WebP 확장자)
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	randomID := rand.Intn(999999)
	fileName := fmt.Sprintf("generated_%d_%d.webp", timestamp, randomID)

	// 파일 경로 생성
	filePath := fmt.Sprintf("generated-images/user-%s/%s", userID, fileName)

	log.Printf("📤 Uploading WebP image to storage: %s", filePath)

	// Supabase Storage API URL
	uploadURL := fmt.Sprintf("%s/storage/v1/object/attachments/%s",
		cfg.SupabaseURL, filePath)

	// HTTP Request 생성 (WebP 데이터 사용)
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
	log.Printf("✅ WebP image uploaded successfully: %s (%d bytes)", filePath, webpSize)
	return filePath, webpSize, nil
}

// CreateAttachRecord - quel_attach 테이블에 레코드 생성
func (s *Service) CreateAttachRecord(ctx context.Context, filePath string, fileSize int64) (int, error) {
	log.Printf("💾 Creating attach record for: %s", filePath)

	// 파일명 추출
	fileName := filePath[len(filePath)-1:]
	if idx := len(filePath) - 1; idx >= 0 {
		for i := len(filePath) - 1; i >= 0; i-- {
			if filePath[i] == '/' {
				fileName = filePath[i+1:]
				break
			}
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

	// attach_id 추출
	var attaches []model.Attach
	if err := json.Unmarshal(data, &attaches); err != nil {
		return 0, fmt.Errorf("failed to parse attach response: %w", err)
	}

	if len(attaches) == 0 {
		return 0, fmt.Errorf("no attach record returned")
	}

	attachID := int(attaches[0].AttachID)
	log.Printf("✅ Attach record created: ID=%d", attachID)

	return attachID, nil
}

// UpdateJobProgress - Job 진행 상황 업데이트
func (s *Service) UpdateJobProgress(ctx context.Context, jobID string, completedImages int, generatedAttachIds []int) error {
	log.Printf("📊 Updating job progress: %d/%d completed", completedImages, len(generatedAttachIds))

	// 중복 제거: 같은 attach_id가 여러 번 포함되지 않도록
	uniqueIds := make([]int, 0, len(generatedAttachIds))
	seen := make(map[int]bool)
	for _, id := range generatedAttachIds {
		if !seen[id] {
			seen[id] = true
			uniqueIds = append(uniqueIds, id)
		}
	}

	if len(uniqueIds) != len(generatedAttachIds) {
		log.Printf("⚠️  Removed %d duplicate attach IDs (before: %d, after: %d)",
			len(generatedAttachIds)-len(uniqueIds), len(generatedAttachIds), len(uniqueIds))
	}

	updateData := map[string]interface{}{
		"completed_images":     completedImages,
		"generated_attach_ids": uniqueIds,
		"updated_at":           "now()",
	}

	_, _, err := s.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update job progress: %w", err)
	}

	log.Printf("✅ Job progress updated: %d images completed", completedImages)
	return nil
}

// UpdateProductionAttachIds - Production Photo의 attach_ids 배열에 추가
func (s *Service) UpdateProductionAttachIds(ctx context.Context, productionID string, newAttachIds []int) error {
	log.Printf("📎 Updating production %s attach_ids with %d new IDs", productionID, len(newAttachIds))

	// 1. 기존 attach_ids 조회
	var productions []struct {
		AttachIds []interface{} `json:"attach_ids"`
	}

	data, _, err := s.supabase.From("quel_production_photo").
		Select("attach_ids", "", false).
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to fetch existing attach_ids: %w", err)
	}

	// JSON 파싱
	if err := json.Unmarshal(data, &productions); err != nil {
		return fmt.Errorf("failed to parse productions: %w", err)
	}

	// 2. 기존 배열과 병합
	var existingIds []int
	if len(productions) > 0 && productions[0].AttachIds != nil {
		for _, id := range productions[0].AttachIds {
			if floatID, ok := id.(float64); ok {
				existingIds = append(existingIds, int(floatID))
			}
		}
	}

	// 3. 새로운 ID들 추가
	mergedIds := append(existingIds, newAttachIds...)
	log.Printf("📎 Merged attach_ids: %d existing + %d new = %d total", len(existingIds), len(newAttachIds), len(mergedIds))

	// 4. Production 업데이트 (JSONB는 직접 배열로 전달)
	updateData := map[string]interface{}{
		"attach_ids": mergedIds,
	}

	_, _, err = s.supabase.From("quel_production_photo").
		Update(updateData, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update production attach_ids: %w", err)
	}

	log.Printf("✅ Production attach_ids updated: %v", mergedIds)
	return nil
}

// DeductCredits - 크레딧 차감 및 트랜잭션 기록 (개인/조직 크레딧 지원)
func (s *Service) DeductCredits(ctx context.Context, userID string, orgID *string, productionID string, attachIds []int) error {
	cfg := config.GetConfig()
	creditsPerImage := cfg.ImagePerPrice
	totalCredits := len(attachIds) * creditsPerImage

	// 조직 크레딧인지 개인 크레딧인지 구분
	isOrgCredit := orgID != nil && *orgID != ""

	if isOrgCredit {
		log.Printf("💰 Deducting ORGANIZATION credits: OrgID=%s, User=%s, Images=%d, Total=%d credits", *orgID, userID, len(attachIds), totalCredits)
	} else {
		log.Printf("💰 Deducting PERSONAL credits: User=%s, Images=%d, Total=%d credits", userID, len(attachIds), totalCredits)
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
			Eq("org_id", *orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to fetch organization credits: %w", err)
		}

		if err := json.Unmarshal(data, &orgs); err != nil {
			return fmt.Errorf("failed to parse organization data: %w", err)
		}

		if len(orgs) == 0 {
			return fmt.Errorf("organization not found: %s", *orgID)
		}

		currentCredits = int(orgs[0].OrgCredit)
		newBalance = currentCredits - totalCredits

		log.Printf("💰 Organization credit balance: %d → %d (-%d)", currentCredits, newBalance, totalCredits)

		// 조직 크레딧 차감
		_, _, err = s.supabase.From("quel_organization").
			Update(map[string]interface{}{
				"org_credit": newBalance,
			}, "", "").
			Eq("org_id", *orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to deduct organization credits: %w", err)
		}

		// 트랜잭션 기록 - 조직 크레딧
		for _, attachID := range attachIds {
			transactionData := map[string]interface{}{
				"user_id":            userID,
				"org_id":             *orgID,
				"used_by_member_id":  userID,
				"transaction_type":   "DEDUCT",
				"amount":             -creditsPerImage,
				"balance_after":      newBalance,
				"description":        "Organization Generated With Image",
				"attach_idx":         attachID,
				"production_idx":     productionID,
			}

			_, _, err := s.supabase.From("quel_credits").
				Insert(transactionData, false, "", "", "").
				Execute()

			if err != nil {
				log.Printf("⚠️  Failed to record organization transaction for attach_id %d: %v", attachID, err)
			}
		}

		log.Printf("✅ Organization credits deducted successfully: %d credits from org %s (used by %s)", totalCredits, *orgID, userID)
	} else {
		// 개인 크레딧 차감 (기존 로직)
		var members []struct {
			QuelMemberCredit int `json:"quel_member_credit"`
		}

		data, _, err := s.supabase.From("quel_member").
			Select("quel_member_credit", "", false).
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to fetch user credits: %w", err)
		}

		if err := json.Unmarshal(data, &members); err != nil {
			return fmt.Errorf("failed to parse member data: %w", err)
		}

		if len(members) == 0 {
			return fmt.Errorf("user not found: %s", userID)
		}

		currentCredits = members[0].QuelMemberCredit
		newBalance = currentCredits - totalCredits

		log.Printf("💰 Personal credit balance: %d → %d (-%d)", currentCredits, newBalance, totalCredits)

		// 개인 크레딧 차감
		_, _, err = s.supabase.From("quel_member").
			Update(map[string]interface{}{
				"quel_member_credit": newBalance,
			}, "", "").
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to deduct credits: %w", err)
		}

		// 트랜잭션 기록 - 개인 크레딧
		for _, attachID := range attachIds {
			transactionData := map[string]interface{}{
				"user_id":          userID,
				"transaction_type": "DEDUCT",
				"amount":           -creditsPerImage,
				"balance_after":    newBalance,
				"description":      "Generated With Image",
				"attach_idx":       attachID,
				"production_idx":   productionID,
			}

			_, _, err := s.supabase.From("quel_credits").
				Insert(transactionData, false, "", "", "").
				Execute()

			if err != nil {
				log.Printf("⚠️  Failed to record transaction for attach_id %d: %v", attachID, err)
			}
		}

		log.Printf("✅ Personal credits deducted successfully: %d credits from user %s", totalCredits, userID)
	}

	return nil
}
