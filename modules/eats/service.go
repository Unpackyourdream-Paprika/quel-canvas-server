package eats

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // JPEG 디코더 등록
	"image/draw"
	"image/png"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kolesa-team/go-webp/encoder"
	_ "github.com/kolesa-team/go-webp/decoder" // WebP 디코더 등록
	"github.com/kolesa-team/go-webp/webp"
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

// ImageCategories - 카테고리별 이미지 분류 구조체 (음식용)
type ImageCategories struct {
	Model       []byte   // 메인 요리 이미지 (최대 1장)
	Clothing    [][]byte // 부재료/사이드 이미지 배열
	Accessories [][]byte // 토핑/가니쉬 이미지 배열
	Background  []byte   // 레스토랑/세팅 배경 이미지 (최대 1장)
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

	// 4. HTTP GET으로 직접 다운로드
	httpResp, err := http.Get(fullURL)
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

	// WebP 인코딩
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, quality)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebP encoder options: %w", err)
	}

	var webpBuffer bytes.Buffer
	err = webp.Encode(&webpBuffer, img, options)
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
	cols := int(math.Ceil(math.Sqrt(float64(numImages)))) // 열 개수
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

// generateDynamicPrompt - Eats 모듈 전용 프롬프트 생성 (음식 사진)
func generateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasMainDish := categories.Model != nil
	hasIngredients := len(categories.Clothing) > 0
	hasToppings := len(categories.Accessories) > 0
	hasFoodItems := hasIngredients || hasToppings
	hasRestaurant := categories.Background != nil

	// 케이스별 메인 지시사항
	var mainInstruction string
	if hasMainDish {
		// 메인 요리 있음 → 음식 에디토리얼
		mainInstruction = "[PROFESSIONAL FOOD PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class culinary photographer shooting for a Michelin-star restaurant editorial.\n" +
			"The DISH is the HERO - its natural colors, textures, and composition are SACRED and CANNOT be altered.\n" +
			"The plating and presentation are PERFECT - showcase them with editorial excellence.\n\n" +
			"Create ONE photorealistic photograph with HIGH-END CULINARY EDITORIAL STYLE:\n" +
			"• ONE beautifully plated dish - this is professional food photography\n" +
			"• AUTHENTIC FOOD STYLING - natural, appetizing, editorial presentation\n" +
			"• Perfect plating with ALL ingredients and toppings visible\n" +
			"• Professional restaurant photography aesthetic\n" +
			"• Directional lighting highlights textures, colors, and steam\n" +
			"• This is a MOMENT of culinary artistry and gastronomic excellence\n\n"
	} else if hasFoodItems {
		// 음식 재료만 → 재료 스틸라이프
		mainInstruction = "[CULINARY STILL LIFE PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class food photographer creating editorial-style ingredient photography.\n" +
			"The INGREDIENTS are the STARS - showcase them as fresh, beautiful objects with perfect details.\n" +
			"⚠️ CRITICAL: NO people or hands in this shot - ingredients only.\n\n" +
			"Create ONE photorealistic photograph with EDITORIAL FOOD STYLING:\n" +
			"• Artistic arrangement of fresh ingredients - creative composition\n" +
			"• Dramatic lighting that highlights textures and natural colors\n" +
			"• Restaurant kitchen or rustic table atmosphere\n" +
			"• This is high-end culinary still life with editorial quality\n\n"
	} else {
		// 배경만 → 레스토랑 환경 사진
		mainInstruction = "[RESTAURANT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class restaurant photographer capturing dining atmosphere.\n" +
			"The RESTAURANT is the SUBJECT - showcase its ambiance, design, and character.\n" +
			"⚠️ CRITICAL: NO people or food in this shot - environment only.\n\n" +
			"Create ONE photorealistic photograph with ATMOSPHERIC STORYTELLING:\n" +
			"• Dramatic composition that captures the restaurant's essence\n" +
			"• Interior design, lighting, and dining atmosphere\n" +
			"• Professional architectural photography of dining spaces\n" +
			"• This is editorial restaurant photography with cinematic quality\n\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명 (음식 용어로)
	if categories.Model != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (MAIN DISH - FOOD ONLY): This is a FOOD/DISH photograph showing plating, colors, textures, and presentation. This is NOT a person - it's FOOD. Recreate this DISH EXACTLY with the same culinary style and plating", imageIndex))
		imageIndex++
	}

	if len(categories.Clothing) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (INGREDIENTS/SIDES): ALL visible ingredients, side dishes, or components. The dish MUST include EVERY item shown here", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (TOPPINGS/GARNISH): ALL toppings, garnishes, sauces, herbs, or finishing touches. The dish MUST feature EVERY element shown here", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (RESTAURANT/SETTING INSPIRATION): This shows the ATMOSPHERE and DINING ENVIRONMENT you should recreate. Use this to understand the setting mood, lighting style, and restaurant ambiance. Generate a COMPLETELY NEW environment inspired by this reference", imageIndex))
		imageIndex++
	}

	// 구성 지시사항
	var compositionInstruction string

	// 케이스 1: 메인 요리가 있는 경우 → 플레이팅 샷
	if hasMainDish {
		compositionInstruction = "\n[CULINARY EDITORIAL COMPOSITION]\n" +
			"Generate ONE photorealistic culinary photograph showing the referenced dish with professional plating (including all ingredients + toppings).\n" +
			"This is high-end restaurant photography with the dish as the centerpiece."
	} else if hasFoodItems {
		// 케이스 2: 재료만 → 재료 스틸라이프
		compositionInstruction = "\n[INGREDIENT STILL LIFE PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic food photograph showcasing the ingredients as fresh, beautiful OBJECTS.\n" +
			"⚠️ DO NOT add any people, hands, or cooking in progress.\n" +
			"⚠️ Display the items artistically arranged - like high-end food magazine photography.\n"

		if hasRestaurant {
			compositionInstruction += "The ingredients are placed naturally within the referenced restaurant environment - " +
				"as if styled by a professional food photographer on location.\n" +
				"The items interact with the space (resting on wooden boards, marble counters, rustic tables)."
		} else {
			compositionInstruction += "Create a stunning culinary still life with professional lighting and composition.\n" +
				"The ingredients are arranged artistically - overhead flat lay, rustic board, or elegantly displayed."
		}
	} else if hasRestaurant {
		// 케이스 3: 레스토랑만 → 환경 사진
		compositionInstruction = "\n[RESTAURANT ENVIRONMENTAL PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic restaurant photograph of the referenced dining environment.\n" +
			"⚠️ DO NOT add any people or food to this scene.\n" +
			"Focus on capturing the atmosphere, interior design, and ambiance of the restaurant space."
	} else {
		// 케이스 4: 아무것도 없는 경우
		compositionInstruction = "\n[CULINARY PHOTOGRAPHY]\n" +
			"Generate a high-quality photorealistic food image based on the references provided."
	}

	// 배경 관련 지시사항 - 메인 요리가 있을 때만 추가
	if hasMainDish && hasRestaurant {
		compositionInstruction += " photographed in a restaurant setting with environmental storytelling.\n\n" +
			"[FOOD PHOTOGRAPHER'S APPROACH TO LOCATION]\n" +
			"The photographer CHOSE this dining environment to complement the dish - not to overwhelm it.\n" +
			"🎬 Use the restaurant reference as INSPIRATION ONLY:\n" +
			"   • Recreate the dining atmosphere, lighting mood, and interior style\n" +
			"   • Generate a NEW scene - do NOT paste or overlay the reference\n" +
			"   • The restaurant serves as a STAGE for the culinary presentation\n\n" +
			"[ABSOLUTE PRIORITY: DISH INTEGRITY]\n" +
			"⚠️ CRITICAL: The dish's colors and textures are UNTOUCHABLE\n" +
			"⚠️ DO NOT distort, over-saturate, or artificially enhance the food\n" +
			"⚠️ The plating and presentation are PERFECT - show them authentically\n\n" +
			"[PROFESSIONAL FOOD PHOTOGRAPHY INTEGRATION]\n" +
			"✓ Dish positioned naturally on table or serving surface\n" +
			"✓ Realistic table setting with natural shadows and reflections\n" +
			"✓ Restaurant elements create DEPTH - use foreground/background layers\n" +
			"✓ Directional lighting from windows or restaurant lights enhances textures\n" +
			"✓ Natural light or warm ambient lighting wraps around the dish\n" +
			"✓ Atmospheric perspective adds editorial depth\n" +
			"✓ Shot composition tells a STORY - this is dining as experience\n\n" +
			"[TECHNICAL EXECUTION]\n" +
			"✓ Single camera angle - this is ONE photograph\n" +
			"✓ Editorial food photography aesthetic with natural color grading\n" +
			"✓ Shallow depth of field focuses attention on the dish\n" +
			"✓ The environment and dish look appetizing and naturally integrated"
	} else if hasMainDish && !hasRestaurant {
		// 메인 요리만 있고 배경 없음 → 스튜디오 테이블
		compositionInstruction += " on a professional table setting with editorial food lighting."
	}

	// 핵심 요구사항 - 케이스별로 다르게
	var criticalRules string

	// 공통 금지사항
	commonForbidden := "\n\n[CRITICAL: ABSOLUTELY FORBIDDEN - THESE WILL CAUSE IMMEDIATE REJECTION]\n\n" +
		"⚠️ NO VERTICAL DIVIDING LINES - ZERO TOLERANCE:\n" +
		"❌ NO white vertical line down the center\n" +
		"❌ NO colored vertical line separating the image\n" +
		"❌ NO border or separator dividing left and right\n" +
		"❌ NO panel division or split layout\n" +
		"❌ The image must be ONE continuous scene without ANY vertical dividers\n\n" +
		"⚠️ NO DUAL/SPLIT COMPOSITION - THIS IS NOT A COMPARISON IMAGE:\n" +
		"❌ DO NOT show the same dish twice (left side vs right side)\n" +
		"❌ DO NOT create before/after, comparison, or variation layouts\n" +
		"❌ DO NOT duplicate the subject on both sides\n" +
		"❌ This is ONE SINGLE MOMENT with ONE DISH in ONE UNIFIED SCENE\n" +
		"❌ Left side and right side must be PART OF THE SAME TABLE, not separate panels\n\n" +
		"⚠️ SINGLE UNIFIED COMPOSITION ONLY:\n" +
		"✓ ONE continuous background that flows naturally across the entire frame\n" +
		"✓ ONE dish in ONE presentation at ONE moment in time\n" +
		"✓ NO repeating elements on left and right sides\n" +
		"✓ The entire image is ONE COHESIVE PHOTOGRAPH - not a collage or split screen\n" +
		"✓ Background elements (table, walls, windows) must be CONTINUOUS with no breaks or seams\n"

	if hasMainDish {
		// 메인 요리 있는 케이스 - 음식 에디토리얼 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS - CULINARY EDITORIAL]\n" +
			"🎯 ONLY ONE DISH in the photograph - this is professional plating photography\n" +
			"🎯 AUTHENTIC FOOD COLORS - natural, appetizing, NOT over-saturated or artificial\n" +
			"🎯 PROFESSIONAL PLATING - elegant presentation, chef-quality composition\n" +
			"🎯 FOOD TEXTURES VISIBLE - show steam, moisture, freshness, natural appeal\n" +
			"🎯 Dish's natural appearance is PERFECT - ZERO tolerance for distortion or fake enhancement\n" +
			"🎯 The dish is the STAR - everything else supports its presentation\n" +
			"🎯 Michelin-star restaurant aesthetic - high-end culinary editorial, NOT fast food catalog\n" +
			"🎯 Dramatic composition with ELEGANCE and APPETITE APPEAL\n" +
			"🎯 Gastronomic storytelling - what's the dining experience of this moment?\n" +
			"🎯 ALL ingredients and toppings plated simultaneously\n" +
			"🎯 Single cohesive photograph - looks like ONE shot from ONE camera\n" +
			"🎯 Editorial food photography aesthetic - warm, natural, appetizing\n" +
			"🎯 Dynamic framing - use negative space and shallow depth of field\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"❌ TWO or more identical dishes in the frame - this is NOT a catalog grid\n" +
			"❌ Multiple portions, duplicate plating, or buffet-style arrangement\n" +
			"❌ ANY distortion of the food's colors (over-saturated, neon, fake-looking)\n" +
			"❌ Food looking plastic, artificial, or CGI-rendered\n" +
			"❌ Hands, people, or cooking in progress visible in frame\n" +
			"❌ Messy, unappetizing, or amateur plating\n" +
			"❌ Fast food catalog style - this is FINE DINING editorial\n" +
			"❌ Centered, boring composition without depth\n" +
			"❌ Flat lighting that doesn't enhance food textures"
	} else if hasFoodItems {
		// 재료 케이스 - 음식 스틸라이프 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS - INGREDIENT PHOTOGRAPHY]\n" +
			"🎯 Showcase the ingredients as fresh, beautiful OBJECTS with perfect details\n" +
			"🎯 Artistic arrangement - creative composition like high-end food magazine\n" +
			"🎯 Dramatic lighting that highlights natural textures and colors\n" +
			"🎯 Fresh, organic, appetizing appearance - peak ingredient quality\n" +
			"🎯 ALL items displayed clearly and beautifully\n" +
			"🎯 Single cohesive photograph - ONE shot from ONE camera\n" +
			"🎯 Editorial food styling aesthetic - natural, rustic, elegant\n" +
			"🎯 Dynamic framing - use negative space and depth creatively\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"❌ ANY people, hands, or cooking in progress in the frame\n" +
			"❌ Ingredients looking artificial, plastic, or fake\n" +
			"❌ Boring, flat catalog-style layouts\n" +
			"❌ Cluttered composition without focal point\n" +
			"❌ Flat lighting that doesn't create appetite appeal"
	} else {
		// 레스토랑만 있는 케이스 - 환경 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS - RESTAURANT PHOTOGRAPHY]\n" +
			"🎯 Capture the pure atmosphere and dining ambiance\n" +
			"🎯 Dramatic composition with architectural depth and visual interest\n" +
			"🎯 Environmental storytelling - what story does this dining space tell?\n" +
			"🎯 Professional interior photography aesthetic\n" +
			"🎯 Dynamic framing - use negative space and layers creatively\n\n" +
			"[FORBIDDEN]\n" +
			"❌ DO NOT add people or food to the scene\n" +
			"❌ Flat, boring composition without depth"
	}

	// aspect ratio별 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "1:1" {
		if hasMainDish {
			// 메인 요리가 있는 1:1 케이스 (정사각형 - 음식 에디토리얼)
			aspectRatioInstruction = "\n\n[1:1 SQUARE CULINARY EDITORIAL - OVERHEAD PLATING]\n" +
				"This is a SQUARE format - perfect for Instagram-style food photography and overhead plating shots.\n\n" +
				"🎬 SQUARE PLATING COMPOSITION:\n" +
				"✓ Balanced composition utilizing the square format\n" +
				"✓ Overhead (bird's eye) or 45-degree angle works beautifully\n" +
				"✓ Dish centered or using rule of thirds for visual interest\n" +
				"✓ Surrounding table elements (cutlery, napkin, drink) create context\n" +
				"✓ Negative space around the dish creates elegance\n\n" +
				"🎬 PLATING PHOTOGRAPHY EXECUTION:\n" +
				"✓ Directional lighting from above or side highlights textures\n" +
				"✓ Natural food photography aesthetic with warm tones\n" +
				"✓ Shallow depth of field emphasizes the dish\n" +
				"✓ Dynamic styling - NOT static or boring\n\n" +
				"GOAL: A stunning square food photograph like Bon Appétit or Kinfolk magazine - \n" +
				"showcasing the dish's beauty with editorial sophistication."
		} else if hasFoodItems {
			// 재료 샷 1:1 케이스
			aspectRatioInstruction = "\n\n[1:1 SQUARE INGREDIENT SHOT]\n" +
				"This is a SQUARE format ingredient shot - balanced and elegant.\n\n" +
				"🎬 SQUARE INGREDIENT COMPOSITION:\n" +
				"✓ Ingredients arranged to utilize the square space creatively\n" +
				"✓ Overhead flat lay or rustic board presentation\n" +
				"✓ Balanced composition with artistic arrangement\n" +
				"✓ Negative space creates visual breathing room\n\n" +
				"🎬 EXECUTION:\n" +
				"✓ Directional lighting creates drama and highlights freshness\n" +
				"✓ Natural food photography aesthetic\n\n" +
				"GOAL: A stunning square ingredient shot."
		} else {
			// 레스토랑만 있는 1:1 케이스
			aspectRatioInstruction = "\n\n[1:1 SQUARE RESTAURANT SHOT]\n" +
				"This is a SQUARE environmental shot - balanced composition.\n\n" +
				"🎬 SQUARE COMPOSITION:\n" +
				"✓ Balanced framing utilizing the square format\n" +
				"✓ Architectural layers create depth\n\n" +
				"🎬 EXECUTION:\n" +
				"✓ Restaurant lighting creates ambiance\n" +
				"✓ Professional interior photography aesthetic\n\n" +
				"GOAL: A stunning square restaurant shot."
		}
	}

	// ⚠️ 최우선 지시사항 - 맨 앞에 배치
	criticalHeader := "⚠️⚠️⚠️ CRITICAL REQUIREMENTS - ABSOLUTE PRIORITY - IMAGE WILL BE REJECTED IF NOT FOLLOWED ⚠️⚠️⚠️\n\n" +
		"[MANDATORY - AUTHENTIC FOOD PHOTOGRAPHY]:\n" +
		"🚨 100% PHOTOREALISTIC - must look like real food photography, NOT CGI or illustration\n" +
		"🚨 NATURAL FOOD COLORS - appetizing, authentic, NOT over-saturated or fake-looking\n" +
		"🚨 REAL FOOD TEXTURES - show moisture, steam, freshness, natural appeal\n" +
		"🚨 NO CARTOON, NO PAINTING, NO ILLUSTRATION STYLE - this is editorial food photography\n" +
		"🚨 Professional restaurant photography aesthetic - Michelin-star quality\n\n" +
		"[MANDATORY - PROFESSIONAL PLATING]:\n" +
		"🚨 CHEF-QUALITY PRESENTATION - elegant, sophisticated, high-end plating\n" +
		"🚨 ALL ingredients visible and beautifully arranged\n" +
		"🚨 Professional food styling - NOT messy or amateur\n" +
		"🚨 This is FINE DINING editorial - NOT fast food catalog\n\n" +
		"[FORBIDDEN - IMAGE WILL BE REJECTED]:\n" +
		"❌ NO cartoon style, illustration, painting, or artistic interpretation\n" +
		"❌ NO over-saturated neon colors or fake CGI food appearance\n" +
		"❌ NO left-right split, NO side-by-side layout, NO duplicate dishes\n" +
		"❌ NO grid, NO collage, NO comparison view, NO before/after layout\n" +
		"❌ NO vertical dividing line, NO center split\n" +
		"❌ NO white/gray borders, NO letterboxing, NO empty margins\n" +
		"❌ ONLY ONE DISH in the photograph - NO multiple identical portions\n\n" +
		"[REQUIRED - MUST GENERATE THIS WAY]:\n" +
		"✓ ONE single photograph taken with ONE camera shutter\n" +
		"✓ ONE unified moment in time - NOT multiple dishes combined\n" +
		"✓ ONLY ONE DISH/SERVING in the entire frame\n" +
		"✓ PHOTOREALISTIC food photography - looks like a real restaurant photograph\n" +
		"✓ Natural, appetizing colors - warm, inviting, delicious-looking\n" +
		"✓ Professional editorial style - Bon Appétit, Kinfolk, Saveur magazine quality\n" +
		"✓ Natural asymmetric composition - left side different from right side\n\n"

	// 최종 조합
	var finalPrompt string

	// 1️⃣ 크리티컬 요구사항을 맨 앞에 배치
	if userPrompt != "" {
		finalPrompt = criticalHeader + "[ADDITIONAL STYLING]\n" + userPrompt + "\n\n"
	} else {
		finalPrompt = criticalHeader
	}

	// 2️⃣ 나머지 지시사항들
	finalPrompt += mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + criticalRules + aspectRatioInstruction

	return finalPrompt
}

// GenerateImageWithGeminiMultiple - 카테고리별 이미지로 Gemini API 호출
func (s *Service) GenerateImageWithGeminiMultiple(ctx context.Context, categories *ImageCategories, userPrompt string, aspectRatio string) (string, error) {
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

	// 동적 프롬프트 생성
	dynamicPrompt := generateDynamicPrompt(categories, userPrompt, aspectRatio)
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