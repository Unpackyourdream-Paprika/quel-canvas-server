package landingdemo

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // JPEG 디코더 등록
	"image/png"
	"log"
	"math"
	"strings"

	_ "github.com/gen2brain/webp" // WebP 디코더 등록
	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
)

type Service struct {
	genaiClient *genai.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Genai 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("❌ [LandingDemo] Failed to create Genai client: %v", err)
		return nil
	}

	log.Println("✅ [LandingDemo] Service initialized")
	return &Service{
		genaiClient: genaiClient,
	}
}

// classifyImages - 이미지를 카테고리별로 분류 (fashion 모듈과 동일한 방식)
func classifyImages(images []ImageWithCategory) *ImageCategories {
	categories := &ImageCategories{
		Clothing:    [][]byte{},
		Accessories: [][]byte{},
	}

	for i, img := range images {
		// base64 디코딩
		base64Data := img.Data
		if idx := findBase64Start(img.Data); idx > 0 {
			base64Data = img.Data[idx:]
		}

		imageData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			log.Printf("⚠️ [LandingDemo] Failed to decode image %d: %v", i, err)
			continue
		}

		category := strings.ToLower(img.Category)
		log.Printf("📎 [LandingDemo] Image %d category: %s (%d bytes)", i+1, category, len(imageData))

		switch category {
		case "model":
			categories.Model = imageData
		case "bg", "background":
			categories.Background = imageData
		case "top", "pants", "outer", "dress", "skirt", "bottom":
			categories.Clothing = append(categories.Clothing, imageData)
		case "shoes", "bag", "accessory", "hat", "glasses", "watch", "jewelry":
			categories.Accessories = append(categories.Accessories, imageData)
		default:
			// 알 수 없는 카테고리는 의류로 분류
			categories.Clothing = append(categories.Clothing, imageData)
		}
	}

	log.Printf("📊 [LandingDemo] Classified: Model=%v, Clothing=%d, Accessories=%d, BG=%v",
		categories.Model != nil, len(categories.Clothing), len(categories.Accessories), categories.Background != nil)

	return categories
}

// GenerateImages - 이미지 생성 (동기 방식, fashion 모듈과 동일한 카테고리 분류)
func (s *Service) GenerateImages(ctx context.Context, req *LandingDemoRequest) (*LandingDemoResponse, error) {
	cfg := config.GetConfig()

	// 기본값 설정
	aspectRatio := req.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "4:5"
	}

	quantity := req.Quantity
	if quantity <= 0 || quantity > 4 {
		quantity = 1
	}

	log.Printf("🎨 [LandingDemo] Generating %d image(s) - prompt: %s, ratio: %s, images: %d",
		quantity, truncateString(req.Prompt, 50), aspectRatio, len(req.Images))

	// 카테고리별 이미지 분류
	categories := classifyImages(req.Images)

	// 결과 이미지 배열
	var generatedImages []string

	// 카테고리별 병합 및 resize (fashion 모듈과 동일)
	var mergedClothing []byte
	var mergedAccessories []byte
	var err error

	if len(categories.Clothing) > 0 {
		mergedClothing, err = mergeImages(categories.Clothing, aspectRatio)
		if err != nil {
			log.Printf("⚠️ [LandingDemo] Failed to merge clothing images: %v", err)
		}
	}

	if len(categories.Accessories) > 0 {
		mergedAccessories, err = mergeImages(categories.Accessories, aspectRatio)
		if err != nil {
			log.Printf("⚠️ [LandingDemo] Failed to merge accessory images: %v", err)
		}
	}

	// 모델 이미지도 리사이즈
	var resizedModel []byte
	if categories.Model != nil {
		resizedModel, err = mergeImages([][]byte{categories.Model}, aspectRatio)
		if err != nil {
			log.Printf("⚠️ [LandingDemo] Failed to resize model image: %v", err)
			resizedModel = categories.Model
		}
	}

	// 배경 이미지도 리사이즈
	var resizedBG []byte
	if categories.Background != nil {
		resizedBG, err = mergeImages([][]byte{categories.Background}, aspectRatio)
		if err != nil {
			log.Printf("⚠️ [LandingDemo] Failed to resize background image: %v", err)
			resizedBG = categories.Background
		}
	}

	// 각 이미지 생성
	for i := 0; i < quantity; i++ {
		// Parts 구성: 카테고리 순서대로 (Model → Clothing → Accessories → Background)
		// 병합된 이미지 사용 (fashion 모듈과 동일)
		var parts []*genai.Part

		if resizedModel != nil {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{MIMEType: "image/png", Data: resizedModel},
			})
			log.Printf("📎 [LandingDemo] Added Model image (resized)")
		}

		if mergedClothing != nil {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{MIMEType: "image/png", Data: mergedClothing},
			})
			log.Printf("📎 [LandingDemo] Added Clothing image (merged from %d items)", len(categories.Clothing))
		}

		if mergedAccessories != nil {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{MIMEType: "image/png", Data: mergedAccessories},
			})
			log.Printf("📎 [LandingDemo] Added Accessories image (merged from %d items)", len(categories.Accessories))
		}

		if resizedBG != nil {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{MIMEType: "image/png", Data: resizedBG},
			})
			log.Printf("📎 [LandingDemo] Added Background image (resized)")
		}

		// 동적 프롬프트 생성 (fashion 모듈과 동일)
		prompt := BuildDynamicPrompt(categories, req.Prompt, aspectRatio)
		parts = append(parts, genai.NewPartFromText(prompt))

		// Content 생성
		content := &genai.Content{
			Parts: parts,
		}

		// Gemini API 호출
		log.Printf("📤 [LandingDemo] Calling Gemini API for image %d/%d with %d parts...", i+1, quantity, len(parts))
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
			log.Printf("❌ [LandingDemo] Gemini API error for image %d: %v", i+1, err)
			continue
		}

		// 응답에서 이미지 추출
		for _, candidate := range result.Candidates {
			if candidate.Content == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil && len(part.InlineData.Data) > 0 {
					imageBase64 := base64.StdEncoding.EncodeToString(part.InlineData.Data)
					generatedImages = append(generatedImages, imageBase64)
					log.Printf("✅ [LandingDemo] Image %d generated: %d bytes", i+1, len(part.InlineData.Data))
					break // 첫 번째 이미지만
				}
			}
		}
	}

	if len(generatedImages) == 0 {
		return &LandingDemoResponse{
			Success:      false,
			ErrorMessage: "Failed to generate images",
		}, nil
	}

	log.Printf("✅ [LandingDemo] Generated %d images successfully", len(generatedImages))

	return &LandingDemoResponse{
		Success: true,
		Images:  generatedImages,
	}, nil
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

// mergeImages - 여러 이미지를 Grid 방식으로 병합 (fashion 모듈과 동일)
func mergeImages(images [][]byte, aspectRatio string) ([]byte, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("no images to merge")
	}

	// 단일 이미지도 리사이즈 처리
	if len(images) == 1 {
		log.Printf("🔄 [LandingDemo] Single image - resizing to aspect ratio: %s", aspectRatio)
		img, format, err := image.Decode(bytes.NewReader(images[0]))
		if err != nil {
			log.Printf("⚠️ [LandingDemo] Failed to decode single image: %v - returning original", err)
			return images[0], nil
		}
		log.Printf("🔍 [LandingDemo] Single image format: %s, size: %dx%d", format, img.Bounds().Dx(), img.Bounds().Dy())

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

		resized := resizeImage(img, targetWidth, targetHeight)
		log.Printf("✅ [LandingDemo] Resized single image to %dx%d", targetWidth, targetHeight)

		var buf bytes.Buffer
		if err := png.Encode(&buf, resized); err != nil {
			return nil, fmt.Errorf("failed to encode resized image: %w", err)
		}
		return buf.Bytes(), nil
	}

	// 이미지 디코드 (WebP, PNG, JPEG 자동 감지)
	decodedImages := []image.Image{}
	for i, imgData := range images {
		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			log.Printf("⚠️ [LandingDemo] Failed to decode image %d: %v", i, err)
			continue
		}
		log.Printf("🔍 [LandingDemo] Decoded image %d format: %s", i, format)
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

	log.Printf("✅ [LandingDemo] Merged %d images into %dx%d grid (%dx%d total)", len(decodedImages), rows, cols, totalWidth, totalHeight)

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
		log.Printf("✅ [LandingDemo] Resized merged grid to %dx%d (aspect-ratio: %s)", targetWidth, targetHeight, aspectRatio)
	} else {
		log.Printf("✅ [LandingDemo] 1:1 aspect-ratio - skipping resize, keeping original grid size")
	}

	// PNG 인코딩
	var buf bytes.Buffer
	if err := png.Encode(&buf, finalImage); err != nil {
		return nil, fmt.Errorf("failed to encode merged image: %w", err)
	}

	return buf.Bytes(), nil
}

// resizeImage - 이미지를 지정된 크기로 resize (비율 유지하며 fit)
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

	// 새 이미지 생성 (목표 크기)
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
