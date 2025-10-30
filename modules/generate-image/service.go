package generateimage

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

	"github.com/kolesa-team/go-webp/encoder"
	_ "github.com/kolesa-team/go-webp/decoder" // WebP 디코더 등록
	"github.com/kolesa-team/go-webp/webp"
	"github.com/supabase-community/supabase-go"
	"google.golang.org/genai"
)

type Service struct {
	supabase    *supabase.Client
	genaiClient *genai.Client
}

// ImageCategories - 카테고리별 이미지 분류 구조체
type ImageCategories struct {
	Model       []byte   // 모델 이미지 (최대 1장)
	Clothing    [][]byte // 의류 이미지 배열 (top, pants, outer)
	Accessories [][]byte // 악세사리 이미지 배열 (shoes, bag, accessory)
	Background  []byte   // 배경 이미지 (최대 1장)
}

func NewService() *Service {
	config := GetConfig()

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(config.SupabaseURL, config.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ Failed to create Supabase client: %v", err)
		return nil
	}

	// Genai 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("❌ Failed to create Genai client: %v", err)
		return nil
	}

	log.Println("✅ Supabase and Genai clients initialized")
	return &Service{
		supabase:    supabaseClient,
		genaiClient: genaiClient,
	}
}

// FetchJobFromSupabase - Supabase에서 Job 데이터 조회
func (s *Service) FetchJobFromSupabase(jobID string) (*ProductionJob, error) {
	log.Printf("🔍 Fetching job from Supabase: %s", jobID)

	var jobs []ProductionJob

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

	if status == StatusProcessing {
		updateData["started_at"] = "now()"
	} else if status == StatusCompleted || status == StatusFailed {
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
func (s *Service) FetchAttachInfo(attachID int) (*Attach, error) {
	log.Printf("🔍 Fetching attach info: %d", attachID)

	var attaches []Attach

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
	config := GetConfig()

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
	fullURL := config.SupabaseStorageBaseURL + filePath
	log.Printf("📥 Downloading image from: %s", fullURL)
	log.Printf("   🔗 Base URL: %s", config.SupabaseStorageBaseURL)
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
	config := GetConfig()

	// aspect-ratio 기본값 처리
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	log.Printf("🎨 Calling Gemini API (model: %s) with prompt length: %d, aspect-ratio: %s", config.GeminiModel, len(prompt), aspectRatio)

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
		config.GeminiModel,
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

// generateDynamicPrompt - 상황별 동적 프롬프트 생성
func generateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 🔥 맨 앞에 강력한 참조 이미지 조합 지시사항
	mainInstruction := "[CRITICAL - REFERENCE IMAGES MUST BE COMBINED]\n" +
		"You are provided with multiple reference images that MUST be combined into ONE single photograph.\n" +
		"All clothing and accessories shown in the reference images MUST appear as worn/carried by ONE person in ONE unified photo.\n" +
		"DO NOT display any items separately. DO NOT create product layouts. DO NOT show items floating.\n" +
		"Generate ONE complete fashion photograph where the person is wearing ALL items from the reference images.\n\n"

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 설명 추가
	if categories.Model != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d: Model's face and body (use this person's appearance)", imageIndex))
		imageIndex++
	}

	if len(categories.Clothing) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d: Clothing items - the person MUST wear ALL these items (tops, pants, outerwear)", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d: Accessories - the person MUST wear/carry ALL these items (shoes, bags, jewelry)", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d: Background environment (use this setting)", imageIndex))
		imageIndex++
	}

	// 기본 생성 지시사항
	var compositionInstruction string
	if categories.Model != nil {
		compositionInstruction = "\n[COMPOSITION REQUIREMENT]\n" +
			"Create ONE professional fashion photograph showing the referenced model wearing ALL clothing and accessories from the reference images"
	} else {
		compositionInstruction = "\n[COMPOSITION REQUIREMENT]\n" +
			"Create ONE professional fashion photograph showing a model wearing ALL clothing and accessories from the reference images"
	}

	if categories.Background != nil {
		compositionInstruction += " PHYSICALLY GROUNDED and NATURALLY INTEGRATED into the referenced background environment.\n\n" +
			"[BACKGROUND INTEGRATION - ABSOLUTELY CRITICAL]\n" +
			"🔥🔥🔥 The person MUST be STANDING ON THE GROUND in the background location - NOT FLOATING!\n\n" +
			"[REQUIRED - PHYSICAL GROUNDING]\n" +
			"✓ Person's feet MUST be ON THE GROUND/FLOOR of the background scene\n" +
			"✓ Create realistic contact shadows where feet meet the ground\n" +
			"✓ Person MUST cast shadows consistent with the background lighting\n" +
			"✓ Match the exact lighting direction, intensity, and color of the background\n" +
			"✓ Apply same atmospheric effects (haze, depth, air perspective) to the person\n" +
			"✓ Person should be affected by same environmental lighting as background objects\n" +
			"✓ Embed the person INTO the scene depth - not in front of it\n" +
			"✓ Match perspective and viewing angle perfectly with the background\n" +
			"✓ Person should have same color grading and tone as the background\n" +
			"✓ Create natural depth of field - person should be part of the scene's focus plane\n" +
			"✓ It MUST look like ONE photograph taken with ONE camera in ONE location\n\n" +
			"[ABSOLUTELY FORBIDDEN - WILL REJECT IMAGE]\n" +
			"❌❌❌ Person looking like they are FLOATING above the ground\n" +
			"❌❌❌ Person looking like a CUTOUT or STICKER pasted on the background\n" +
			"❌❌❌ Person having DIFFERENT LIGHTING than the background\n" +
			"❌❌❌ NO SHADOWS or WRONG shadow direction\n" +
			"❌❌❌ Green screen effect or obvious composite look\n" +
			"❌❌❌ Person appearing to be in FRONT of the background instead of IN the scene\n" +
			"❌❌❌ Different color temperature or atmosphere between person and background"
	} else {
		compositionInstruction += " in a clean, professional studio setting."
	}

	// CRITICAL RULES 추가
	criticalRules := "\n\n[ABSOLUTELY FORBIDDEN]\n" +
		"❌ NO separate product images floating in the frame\n" +
		"❌ NO clothing displayed separately from the person\n" +
		"❌ NO split screen or collage layout showing items separately\n" +
		"❌ NO e-commerce product layout\n" +
		"❌ NO grid showing multiple views\n" +
		"❌ NO empty margins or letterboxing on sides\n" +
		"❌ NO white/gray bars on left or right\n\n" +
		"[REQUIRED OUTPUT]\n" +
		"✓ ONE unified photograph taken with ONE camera shutter\n" +
		"✓ ALL reference items MUST be worn/carried by the person\n" +
		"✓ FILL entire frame edge-to-edge with NO empty space\n" +
		"✓ Natural, asymmetric composition (left side ≠ right side)\n" +
		"✓ Professional magazine editorial style\n" +
		"✓ Single continuous moment in time"

	// 16:9 비율 전용 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "16:9" {
		aspectRatioInstruction = "\n\n[16:9 WIDE FRAME - CRITICAL SPATIAL INTEGRATION]\n" +
			"🔥🔥🔥 This is a WIDE HORIZONTAL frame - special attention required!\n\n" +
			"[DEPTH & PERSPECTIVE - MANDATORY]\n" +
			"✓ Create STRONG DEPTH and REALISTIC PERSPECTIVE in the wide frame\n" +
			"✓ Person MUST be embedded IN the 3D space of the scene, not flat against it\n" +
			"✓ Use foreground, midground, and background layers for spatial depth\n" +
			"✓ Apply proper atmospheric perspective (distant objects slightly hazier)\n" +
			"✓ Ensure person casts realistic shadows that interact with the ground plane\n" +
			"✓ Match the scale and proportions naturally within the environment\n\n" +
			"[WIDE FRAME COMPOSITION]\n" +
			"✓ Utilize the FULL WIDTH naturally - fill horizontal space with scene context\n" +
			"✓ Position subject slightly off-center (rule of thirds) for natural look\n" +
			"✓ Background should extend naturally to frame edges, not feel cropped\n" +
			"✓ Person should feel like they BELONG in this wide environmental shot\n\n" +
			"[LIGHTING & INTEGRATION]\n" +
			"✓ Person MUST have IDENTICAL lighting as the environment (same direction, color, intensity)\n" +
			"✓ Apply environmental light wrap and ambient occlusion\n" +
			"✓ Match color temperature and atmospheric conditions perfectly\n" +
			"✓ Person should be affected by the same light sources visible in the background\n\n" +
			"[ABSOLUTELY FORBIDDEN IN 16:9]\n" +
			"❌❌❌ Person looking PASTED or COMPOSITED onto background\n" +
			"❌❌❌ Flat, cardboard cutout appearance\n" +
			"❌❌❌ Different lighting on person vs environment\n" +
			"❌❌❌ Floating or disconnected from ground plane\n" +
			"❌❌❌ Unrealistic scale or perspective mismatch\n" +
			"❌❌❌ Center-framed subject in wide shot (use off-center composition)\n\n" +
			"GOAL: It must look like ONE REAL PHOTOGRAPH taken in ONE LOCATION with ONE CAMERA."
	}

	// 최종 조합: 강력한 지시사항 → 참조 이미지 설명 → 구성 요구사항 → 금지사항 → 16:9 특화
	finalPrompt := mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + criticalRules + aspectRatioInstruction

	if userPrompt != "" {
		finalPrompt += "\n\n[ADDITIONAL STYLING]\n" + userPrompt
	}

	return finalPrompt
}

// GenerateImageWithGeminiMultiple - 카테고리별 이미지로 Gemini API 호출
func (s *Service) GenerateImageWithGeminiMultiple(ctx context.Context, categories *ImageCategories, userPrompt string, aspectRatio string) (string, error) {
	config := GetConfig()

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
		config.GeminiModel,
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
	config := GetConfig()

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
		config.SupabaseURL, filePath)

	// HTTP Request 생성 (WebP 데이터 사용)
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(webpData))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.SupabaseServiceKey)
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
	var attaches []Attach
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

	updateData := map[string]interface{}{
		"completed_images":     completedImages,
		"generated_attach_ids": generatedAttachIds,
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

// DeductCredits - 크레딧 차감 및 트랜잭션 기록
func (s *Service) DeductCredits(ctx context.Context, userID string, productionID string, attachIds []int) error {
	config := GetConfig()
	creditsPerImage := config.ImagePerPrice
	totalCredits := len(attachIds) * creditsPerImage

	log.Printf("💰 Deducting credits: User=%s, Images=%d, Total=%d credits", userID, len(attachIds), totalCredits)

	// 1. 현재 크레딧 조회
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

	currentCredits := members[0].QuelMemberCredit
	newBalance := currentCredits - totalCredits

	log.Printf("💰 Credit balance: %d → %d (-%d)", currentCredits, newBalance, totalCredits)

	// 2. 크레딧 차감
	_, _, err = s.supabase.From("quel_member").
		Update(map[string]interface{}{
			"quel_member_credit": newBalance,
		}, "", "").
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to deduct credits: %w", err)
	}

	// 3. 각 이미지에 대해 트랜잭션 기록
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

	log.Printf("✅ Credits deducted successfully: %d credits from user %s", totalCredits, userID)
	return nil
}