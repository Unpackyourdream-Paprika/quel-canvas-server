package landingdemo

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
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	webp "github.com/gen2brain/webp"
	_ "github.com/gen2brain/webp" // WebP 디코더 등록
	"github.com/supabase-community/supabase-go"
	"cloud.google.com/go/vertexai/genai"

	"quel-canvas-server/modules/common/config"
	vertexai "quel-canvas-server/modules/common/vertexai"
	"quel-canvas-server/modules/common/model"
	"quel-canvas-server/modules/common/org"
	redisutil "quel-canvas-server/modules/common/redis"
)

// attach_ids 업데이트용 뮤텍스 (production별)
var productionMutexes = make(map[string]*sync.Mutex)
var productionMutexLock sync.Mutex

func getProductionMutex(productionID string) *sync.Mutex {
	productionMutexLock.Lock()
	defer productionMutexLock.Unlock()
	if productionMutexes[productionID] == nil {
		productionMutexes[productionID] = &sync.Mutex{}
	}
	return productionMutexes[productionID]
}

// job attach_ids 업데이트용 뮤텍스 (job별)
var jobMutexes = make(map[string]*sync.Mutex)
var jobMutexLock sync.Mutex

func getJobMutex(jobID string) *sync.Mutex {
	jobMutexLock.Lock()
	defer jobMutexLock.Unlock()
	if jobMutexes[jobID] == nil {
		jobMutexes[jobID] = &sync.Mutex{}
	}
	return jobMutexes[jobID]
}

type Service struct {
	genaiClient *genai.Client
	supabase    *supabase.Client
	redis       *redis.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ [LandingDemo] Failed to create Supabase client: %v", err)
		return nil
	}

	// Vertex AI 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := vertexai.NewVertexAIClient(ctx, cfg.VertexAIProject, cfg.VertexAILocation)
	if err != nil {
		log.Printf("❌ [LandingDemo] Failed to create Vertex AI client: %v", err)
		return nil
	}

	// Redis 클라이언트 초기화
	redisClient := redisutil.Connect(cfg)
	if redisClient == nil {
		log.Printf("⚠️ [LandingDemo] Failed to connect to Redis - cancel feature will be disabled")
	}

	log.Println("✅ [LandingDemo] Service initialized with Vertex AI")
	return &Service{
		supabase:    supabaseClient,
		genaiClient: genaiClient,
		redis:       redisClient,
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

	// Vertex AI GenerativeModel 가져오기
	model := s.genaiClient.GenerativeModel(cfg.GeminiModel)
	model.SetTemperature(0.45)

	// Note: ResponseMIMEType should NOT be set for image generation with Gemini

	// 각 이미지 생성
	for i := 0; i < quantity; i++ {
		// Parts 구성: 카테고리 순서대로 (Model → Clothing → Accessories → Background)
		var parts []genai.Part

		if resizedModel != nil {
			parts = append(parts, genai.ImageData("image/png", resizedModel))
			log.Printf("📎 [LandingDemo] Added Model image (resized)")
		}

		if mergedClothing != nil {
			parts = append(parts, genai.ImageData("image/png", mergedClothing))
			log.Printf("📎 [LandingDemo] Added Clothing image (merged from %d items)", len(categories.Clothing))
		}

		if mergedAccessories != nil {
			parts = append(parts, genai.ImageData("image/png", mergedAccessories))
			log.Printf("📎 [LandingDemo] Added Accessories image (merged from %d items)", len(categories.Accessories))
		}

		if resizedBG != nil {
			parts = append(parts, genai.ImageData("image/png", resizedBG))
			log.Printf("📎 [LandingDemo] Added Background image (resized)")
		}

		// 동적 프롬프트 생성 (fashion 모듈과 동일)
		prompt := BuildDynamicPrompt(categories, req.Prompt, aspectRatio)
		parts = append(parts, genai.Text(prompt))

		// Vertex AI 호출
		log.Printf("📤 [LandingDemo] Calling Vertex AI for image %d/%d with %d parts...", i+1, quantity, len(parts))
		result, err := model.GenerateContent(ctx, parts...)
		if err != nil {
			log.Printf("❌ [LandingDemo] Vertex AI error for image %d: %v", i+1, err)
			continue
		}

		// 응답에서 이미지 추출
		for _, candidate := range result.Candidates {
			if candidate.Content == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				// Vertex AI SDK는 Part를 Blob 타입으로 반환
				if blob, ok := part.(genai.Blob); ok {
					if len(blob.Data) > 0 {
						imageBase64 := base64.StdEncoding.EncodeToString(blob.Data)
						generatedImages = append(generatedImages, imageBase64)
						log.Printf("✅ [LandingDemo] Image %d generated: %d bytes", i+1, len(blob.Data))
						break // 첫 번째 이미지만
					}
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

// ============================================
// Worker용 메서드들 (DB 연동)
// ============================================

// NewServiceWithDB - DB 연동 포함 서비스 초기화 (Worker용)
func NewServiceWithDB() *Service {
	cfg := config.GetConfig()

	// Vertex AI 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := vertexai.NewVertexAIClient(ctx, cfg.VertexAIProject, cfg.VertexAILocation)
	if err != nil {
		log.Printf("❌ [Landing] Failed to create Vertex AI client: %v", err)
		return nil
	}

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, nil)
	if err != nil {
		log.Printf("❌ [Landing] Failed to create Supabase client: %v", err)
		return nil
	}

	log.Println("✅ [Landing] Service with DB initialized (Vertex AI)")
	return &Service{
		genaiClient: genaiClient,
		supabase:    supabaseClient,
	}
}

// FetchJobFromSupabase - Job 데이터 조회
func (s *Service) FetchJobFromSupabase(jobID string) (*model.ProductionJob, error) {
	log.Printf("🔍 [Landing] Fetching job: %s", jobID)

	var jobs []model.ProductionJob
	data, _, err := s.supabase.From("quel_production_jobs").
		Select("*", "exact", false).
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return nil, fmt.Errorf("failed to query job: %w", err)
	}

	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse job: %w", err)
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return &jobs[0], nil
}

// UpdateJobStatus - Job 상태 업데이트
func (s *Service) UpdateJobStatus(ctx context.Context, jobID string, status string) error {
	log.Printf("📝 [Landing] Updating job %s status to: %s", jobID, status)

	updateData := map[string]interface{}{
		"job_status": status,
		"updated_at": "now()",
	}

	_, _, err := s.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	log.Printf("✅ [Landing] Job %s status updated to: %s", jobID, status)
	return nil
}

// UpdateProductionPhotoStatus - Production 상태 업데이트
func (s *Service) UpdateProductionPhotoStatus(ctx context.Context, productionID string, status string) error {
	log.Printf("📝 [Landing] Updating production %s status to: %s", productionID, status)

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

	log.Printf("✅ [Landing] Production %s status updated to: %s", productionID, status)
	return nil
}

// IsJobCancelled - Job 취소 여부 확인
func (s *Service) IsJobCancelled(jobID string) bool {
	if s.redis == nil {
		return false
	}
	return redisutil.IsJobCancelled(s.redis, jobID)
}

// FetchAttachInfo - Attach 정보 조회
func (s *Service) FetchAttachInfo(attachID int) (*model.Attach, error) {
	log.Printf("🔍 [Landing] Fetching attach info: %d", attachID)

	var attaches []model.Attach
	data, _, err := s.supabase.From("quel_attach").
		Select("*", "exact", false).
		Eq("attach_id", fmt.Sprintf("%d", attachID)).
		Execute()

	if err != nil {
		return nil, fmt.Errorf("failed to query attach: %w", err)
	}

	if err := json.Unmarshal(data, &attaches); err != nil {
		return nil, fmt.Errorf("failed to parse attach: %w", err)
	}

	if len(attaches) == 0 {
		return nil, fmt.Errorf("attach not found: %d", attachID)
	}

	return &attaches[0], nil
}

// DownloadImageFromStorage - Storage에서 이미지 다운로드
func (s *Service) DownloadImageFromStorage(attachID int) ([]byte, error) {
	cfg := config.GetConfig()

	attach, err := s.FetchAttachInfo(attachID)
	if err != nil {
		return nil, err
	}

	var filePath string
	if attach.AttachFilePath != nil && *attach.AttachFilePath != "" {
		filePath = *attach.AttachFilePath
	} else if attach.AttachDirectory != nil && *attach.AttachDirectory != "" {
		filePath = *attach.AttachDirectory
	} else {
		return nil, fmt.Errorf("no file path for attach: %d", attachID)
	}

	// uploads/ 폴더 자동 추가
	if len(filePath) > 0 && filePath[0] != '/' && len(filePath) >= 7 && filePath[:7] == "upload-" {
		filePath = "uploads/" + filePath
	}

	fullURL := cfg.SupabaseStorageBaseURL + filePath
	log.Printf("📥 [Landing] Downloading image: %s", fullURL)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	log.Printf("✅ [Landing] Image downloaded: %d bytes", len(imageData))
	return imageData, nil
}

// ConvertToWebP - 이미지를 WebP로 변환 (PNG, JPEG 등 모든 포맷 지원)
func (s *Service) ConvertPNGToWebP(imageData []byte, quality float32) ([]byte, error) {
	reader := bytes.NewReader(imageData)
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	log.Printf("🔄 [Landing] Converting %s to WebP", format)

	var webpBuffer bytes.Buffer
	err = webp.Encode(&webpBuffer, img, webp.Options{Quality: int(quality)})
	if err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %w", err)
	}

	return webpBuffer.Bytes(), nil
}

// UploadImageToStorage - Storage에 이미지 업로드
func (s *Service) UploadImageToStorage(ctx context.Context, imageData []byte, userID string) (string, int64, error) {
	cfg := config.GetConfig()

	// PNG to WebP
	webpData, err := s.ConvertPNGToWebP(imageData, 90.0)
	if err != nil {
		return "", 0, fmt.Errorf("webp conversion failed: %w", err)
	}

	// 파일명 생성
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	randomID := rand.Intn(999999)
	fileName := fmt.Sprintf("generated_%d_%d.webp", timestamp, randomID)
	filePath := fmt.Sprintf("generated-images/user-%s/%s", userID, fileName)

	log.Printf("📤 [Landing] Uploading image: %s", filePath)

	uploadURL := fmt.Sprintf("%s/storage/v1/object/attachments/%s", cfg.SupabaseURL, filePath)
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(webpData))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceKey)
	req.Header.Set("Content-Type", "image/webp")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("upload failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	webpSize := int64(len(webpData))
	log.Printf("✅ [Landing] Image uploaded: %s (%d bytes)", filePath, webpSize)
	return filePath, webpSize, nil
}

// CreateAttachRecord - Attach 레코드 생성
func (s *Service) CreateAttachRecord(ctx context.Context, filePath string, fileSize int64) (int, error) {
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
		return 0, fmt.Errorf("insert failed: %w", err)
	}

	var attaches []model.Attach
	if err := json.Unmarshal(data, &attaches); err != nil {
		return 0, fmt.Errorf("parse failed: %w", err)
	}

	if len(attaches) == 0 {
		return 0, fmt.Errorf("no attach returned")
	}

	attachID := int(attaches[0].AttachID)
	log.Printf("✅ [Landing] Attach created: ID=%d", attachID)
	return attachID, nil
}

// UpdateJobProgress - Job 진행 상황 업데이트
func (s *Service) UpdateJobProgress(ctx context.Context, jobID string, completedImages int, generatedAttachIds []int) error {
	// 중복 제거
	uniqueIds := make([]int, 0, len(generatedAttachIds))
	seen := make(map[int]bool)
	for _, id := range generatedAttachIds {
		if !seen[id] {
			seen[id] = true
			uniqueIds = append(uniqueIds, id)
		}
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
		return fmt.Errorf("update failed: %w", err)
	}

	log.Printf("✅ [Landing] Progress updated: %d images", completedImages)
	return nil
}

// UpdateJobProgressWithURLs - URL 포함 진행 상황 업데이트 (빠른 응답용)
func (s *Service) UpdateJobProgressWithURLs(ctx context.Context, jobID string, completedImages int, imageURLs []string) error {
	updateData := map[string]interface{}{
		"completed_images": completedImages,
		"generated_urls":   imageURLs, // 즉시 표시용 URL
		"updated_at":       "now()",
	}

	_, _, err := s.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	log.Printf("✅ [Landing] Progress updated with URLs: %d images", completedImages)
	return nil
}

// UpdateProductionAttachIds - Production attach_ids 업데이트
func (s *Service) UpdateProductionAttachIds(ctx context.Context, productionID string, newAttachIds []int) error {
	// Race condition 방지를 위한 뮤텍스
	mutex := getProductionMutex(productionID)
	mutex.Lock()
	defer mutex.Unlock()

	log.Printf("📎 [Landing] Updating production %s attach_ids: %d IDs", productionID, len(newAttachIds))

	// 기존 attach_ids 조회
	var productions []struct {
		AttachIds []interface{} `json:"attach_ids"`
	}

	data, _, err := s.supabase.From("quel_production_photo").
		Select("attach_ids", "", false).
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	if err := json.Unmarshal(data, &productions); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}

	// 병합
	var existingIds []int
	if len(productions) > 0 && productions[0].AttachIds != nil {
		for _, id := range productions[0].AttachIds {
			if floatID, ok := id.(float64); ok {
				existingIds = append(existingIds, int(floatID))
			}
		}
	}

	mergedIds := append(existingIds, newAttachIds...)

	// 업데이트
	updateData := map[string]interface{}{
		"attach_ids": mergedIds,
	}

	_, _, err = s.supabase.From("quel_production_photo").
		Update(updateData, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	log.Printf("✅ [Landing] Production attach_ids updated: %v", mergedIds)
	return nil
}

// AppendProductionAttachId - 단일 attach_id를 production에 추가 (백그라운드용)
func (s *Service) AppendProductionAttachId(ctx context.Context, productionID string, attachID int) error {
	return s.UpdateProductionAttachIds(ctx, productionID, []int{attachID})
}

// AppendJobAttachId - 단일 attach_id를 Job에 추가 (백그라운드용)
func (s *Service) AppendJobAttachId(ctx context.Context, jobID string, attachID int) error {
	// Race condition 방지를 위한 뮤텍스
	mutex := getJobMutex(jobID)
	mutex.Lock()
	defer mutex.Unlock()

	log.Printf("📎 [Landing] Appending attach_id %d to job %s", attachID, jobID)

	// 기존 generated_attach_ids 조회
	var jobs []struct {
		GeneratedAttachIds []interface{} `json:"generated_attach_ids"`
	}

	data, _, err := s.supabase.From("quel_production_jobs").
		Select("generated_attach_ids", "", false).
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	if err := json.Unmarshal(data, &jobs); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}

	// 병합
	var existingIds []int
	if len(jobs) > 0 && jobs[0].GeneratedAttachIds != nil {
		for _, id := range jobs[0].GeneratedAttachIds {
			if floatID, ok := id.(float64); ok {
				existingIds = append(existingIds, int(floatID))
			}
		}
	}

	mergedIds := append(existingIds, attachID)

	// 업데이트
	_, _, err = s.supabase.From("quel_production_jobs").
		Update(map[string]interface{}{
			"generated_attach_ids": mergedIds,
			"updated_at":           "now()",
		}, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	log.Printf("✅ [Landing] Job attach_ids updated: %v", mergedIds)
	return nil
}

// DeductCredits - 크레딧 차감
func (s *Service) DeductCredits(ctx context.Context, userID string, orgID *string, productionID string, attachIds []int, apiProvider string) error {
	cfg := config.GetConfig()
	creditsPerImage := cfg.ImagePerPrice
	totalCredits := len(attachIds) * creditsPerImage

	// 조직 크레딧인지 개인 크레딧인지 구분 (공통 함수 사용)
	isOrgCredit := org.ShouldUseOrgCredit(s.supabase, orgID)

	if isOrgCredit {
		log.Printf("💰 [Landing] Deducting ORG credits: %s, %d credits", *orgID, totalCredits)

		var orgs []struct {
			OrgCredit int64 `json:"org_credit"`
		}
		data, _, err := s.supabase.From("quel_organization").
			Select("org_credit", "", false).
			Eq("org_id", *orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("fetch org failed: %w", err)
		}

		if err := json.Unmarshal(data, &orgs); err != nil || len(orgs) == 0 {
			return fmt.Errorf("org not found: %s", *orgID)
		}

		newBalance := int(orgs[0].OrgCredit) - totalCredits

		_, _, err = s.supabase.From("quel_organization").
			Update(map[string]interface{}{"org_credit": newBalance}, "", "").
			Eq("org_id", *orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("deduct org failed: %w", err)
		}

		// 트랜잭션 기록
		for _, attachID := range attachIds {
			s.supabase.From("quel_credits").
				Insert(map[string]interface{}{
					"user_id":           userID,
					"org_id":            *orgID,
					"used_by_member_id": userID,
					"transaction_type":  "DEDUCT",
					"amount":            -creditsPerImage,
					"balance_after":     newBalance,
					"description":       "Landing Template Generated",
					"attach_idx":        attachID,
					"production_idx":    productionID,
					"api_provider":      apiProvider,
				}, false, "", "", "").
				Execute()
		}
	} else {
		log.Printf("💰 [Landing] Deducting PERSONAL credits: %s, %d credits", userID, totalCredits)

		var members []struct {
			QuelMemberCredit int `json:"quel_member_credit"`
		}
		data, _, err := s.supabase.From("quel_member").
			Select("quel_member_credit", "", false).
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("fetch member failed: %w", err)
		}

		if err := json.Unmarshal(data, &members); err != nil || len(members) == 0 {
			return fmt.Errorf("member not found: %s", userID)
		}

		newBalance := members[0].QuelMemberCredit - totalCredits

		_, _, err = s.supabase.From("quel_member").
			Update(map[string]interface{}{"quel_member_credit": newBalance}, "", "").
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("deduct member failed: %w", err)
		}

		// 트랜잭션 기록
		for _, attachID := range attachIds {
			s.supabase.From("quel_credits").
				Insert(map[string]interface{}{
					"user_id":          userID,
					"transaction_type": "DEDUCT",
					"amount":           -creditsPerImage,
					"balance_after":    newBalance,
					"description":      "Landing Template Generated",
					"attach_idx":       attachID,
					"production_idx":   productionID,
					"api_provider":     apiProvider,
				}, false, "", "", "").
				Execute()
		}
	}

	log.Printf("✅ [Landing] Credits deducted: %d (api: %s)", totalCredits, apiProvider)
	return nil
}

// GetUserOrganization - 유저가 속한 조직 ID 조회
func (s *Service) GetUserOrganization(ctx context.Context, userID string) (string, error) {
	var members []struct {
		OrgID string `json:"org_id"`
	}

	data, _, err := s.supabase.From("quel_organization_member").
		Select("org_id", "", false).
		Eq("member_id", userID).
		Eq("status", "active").
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

// GenerateImageWithGeminiMultiple - 카테고리별 이미지로 Gemini 호출
func (s *Service) GenerateImageWithGeminiMultiple(ctx context.Context, categories *ImageCategories, userPrompt string, aspectRatio string) (string, error) {
	cfg := config.GetConfig()

	if aspectRatio == "" {
		aspectRatio = "1:1"
	}

	log.Printf("🎨 [Landing] Generating with categories - Clothing:%d, Accessories:%d, Model:%v, BG:%v",
		len(categories.Clothing), len(categories.Accessories), categories.Model != nil, categories.Background != nil)

	// Vertex AI GenerativeModel 가져오기
	model := s.genaiClient.GenerativeModel(cfg.GeminiModel)
	model.SetTemperature(0.45)

	// Note: ResponseMIMEType should NOT be set for image generation with Gemini

	// Parts 구성
	var parts []genai.Part

	// 모델 이미지
	if categories.Model != nil {
		parts = append(parts, genai.ImageData("image/png", categories.Model))
	}

	// Clothing 이미지 (최대 6장)
	maxImages := 6
	clothingCount := len(categories.Clothing)
	if clothingCount > maxImages {
		clothingCount = maxImages
	}
	for i := 0; i < clothingCount; i++ {
		parts = append(parts, genai.ImageData("image/png", categories.Clothing[i]))
	}

	// Accessories 이미지 (최대 6장)
	accessoryCount := len(categories.Accessories)
	if accessoryCount > maxImages {
		accessoryCount = maxImages
	}
	for i := 0; i < accessoryCount; i++ {
		parts = append(parts, genai.ImageData("image/png", categories.Accessories[i]))
	}

	// 배경 이미지
	if categories.Background != nil {
		parts = append(parts, genai.ImageData("image/png", categories.Background))
	}

	// 프롬프트
	prompt := BuildDynamicPrompt(categories, userPrompt, aspectRatio)
	parts = append(parts, genai.Text(prompt))

	// API 호출
	log.Printf("📤 [Landing] Calling Vertex AI with %d parts...", len(parts))
	result, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", fmt.Errorf("Vertex AI error: %w", err)
	}

	// 응답에서 이미지 추출
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if blob, ok := part.(genai.Blob); ok {
				if len(blob.Data) > 0 {
					imageBase64 := base64.StdEncoding.EncodeToString(blob.Data)
					log.Printf("✅ [Landing] Image generated: %d bytes", len(blob.Data))
					return imageBase64, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no image in response")
}

// ============================================
// Runware API 호출 함수들
// ============================================

// GenerateImageWithRunware - Runware API로 이미지 생성
func (s *Service) GenerateImageWithRunware(ctx context.Context, prompt string, modelID string, aspectRatio string, steps int, cfgScale float64, negativePrompt string, inputImageBase64 string) ([]byte, error) {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		return nil, fmt.Errorf("RUNWARE_API_KEY not configured")
	}

	// Seedream 모델 체크
	isSeedream := strings.Contains(modelID, "seedream") || strings.HasPrefix(modelID, "bytedance:")

	// 해상도 계산
	var width, height int
	if isSeedream {
		width, height = 2048, 2048
	} else {
		width, height = 1024, 1024
	}

	switch aspectRatio {
	case "16:9":
		if isSeedream {
			width, height = 2048, 1152
		} else {
			width, height = 1024, 576
		}
	case "9:16":
		if isSeedream {
			width, height = 1152, 2048
		} else {
			width, height = 576, 1024
		}
	case "4:5":
		if isSeedream {
			width, height = 1638, 2048
		} else {
			width, height = 819, 1024
		}
	}

	// 요청 구성
	reqBody := RunwareRequest{
		TaskType:       "imageInference",
		TaskUUID:       generateUUID(),
		PositivePrompt: prompt,
		Model:          modelID,
		Width:          width,
		Height:         height,
		NumberResults:  1,
		OutputFormat:   "JPEG",
	}

	// Seedream이 아닌 경우에만 steps, cfgScale 추가
	if !isSeedream {
		if steps > 0 {
			reqBody.Steps = steps
		}
		if cfgScale > 0 {
			reqBody.CFGScale = cfgScale
		}
		if negativePrompt != "" {
			reqBody.NegativePrompt = negativePrompt
		}
	}

	// 입력 이미지 처리
	if inputImageBase64 != "" {
		if isSeedream {
			reqBody.ReferenceImages = []string{"data:image/png;base64," + inputImageBase64}
		} else {
			reqBody.InputImage = "data:image/png;base64," + inputImageBase64
			reqBody.Strength = 0.7
		}
	}

	log.Printf("🎨 [Landing] Runware request: model=%s, size=%dx%d, seedream=%v", modelID, width, height, isSeedream)

	// API 호출
	jsonBody, err := json.Marshal([]RunwareRequest{reqBody})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.RunwareAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.RunwareAPIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Runware API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Runware API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var runwareResp RunwareResponse
	if err := json.NewDecoder(resp.Body).Decode(&runwareResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(runwareResp.Data) == 0 || runwareResp.Data[0].ImageURL == "" {
		return nil, fmt.Errorf("no image in Runware response")
	}

	// 이미지 다운로드
	imageURL := runwareResp.Data[0].ImageURL
	log.Printf("📥 [Landing] Downloading Runware image: %s", imageURL[:50]+"...")

	imgResp, err := http.Get(imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer imgResp.Body.Close()

	imageData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	log.Printf("✅ [Landing] Runware image generated: %d bytes", len(imageData))
	return imageData, nil
}

// GenerateImageWithRunwareURL - Runware로 이미지 생성 후 URL만 반환 (빠른 응답용)
func (s *Service) GenerateImageWithRunwareURL(ctx context.Context, prompt string, modelID string, aspectRatio string, steps int, cfgScale float64, negativePrompt string, inputImageBase64 string) (string, error) {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		return "", fmt.Errorf("RUNWARE_API_KEY not configured")
	}

	// Seedream 모델 체크
	isSeedream := strings.Contains(modelID, "seedream") || strings.HasPrefix(modelID, "bytedance:")

	// 해상도 계산
	var width, height int
	if isSeedream {
		width, height = 2048, 2048
	} else {
		width, height = 1024, 1024
	}

	switch aspectRatio {
	case "16:9":
		if isSeedream {
			width, height = 2048, 1152
		} else {
			width, height = 1024, 576
		}
	case "9:16":
		if isSeedream {
			width, height = 1152, 2048
		} else {
			width, height = 576, 1024
		}
	case "4:5":
		if isSeedream {
			width, height = 1638, 2048
		} else {
			width, height = 819, 1024
		}
	}

	// 요청 구성
	reqBody := RunwareRequest{
		TaskType:       "imageInference",
		TaskUUID:       generateUUID(),
		PositivePrompt: prompt,
		Model:          modelID,
		Width:          width,
		Height:         height,
		NumberResults:  1,
		OutputFormat:   "JPEG",
	}

	// Seedream이 아닌 경우에만 steps, cfgScale 추가
	if !isSeedream {
		if steps > 0 {
			reqBody.Steps = steps
		}
		if cfgScale > 0 {
			reqBody.CFGScale = cfgScale
		}
		if negativePrompt != "" {
			reqBody.NegativePrompt = negativePrompt
		}
	}

	// 입력 이미지 처리
	if inputImageBase64 != "" {
		if isSeedream {
			reqBody.ReferenceImages = []string{"data:image/png;base64," + inputImageBase64}
		} else {
			reqBody.InputImage = "data:image/png;base64," + inputImageBase64
			reqBody.Strength = 0.7
		}
	}

	log.Printf("🎨 [Landing] Runware URL request: model=%s, size=%dx%d", modelID, width, height)

	// API 호출
	jsonBody, err := json.Marshal([]RunwareRequest{reqBody})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.RunwareAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.RunwareAPIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Runware API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Runware API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var runwareResp RunwareResponse
	if err := json.NewDecoder(resp.Body).Decode(&runwareResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(runwareResp.Data) == 0 || runwareResp.Data[0].ImageURL == "" {
		return "", fmt.Errorf("no image in Runware response")
	}

	imageURL := runwareResp.Data[0].ImageURL
	log.Printf("✅ [Landing] Runware URL received: %s", imageURL[:50]+"...")
	return imageURL, nil
}

// DownloadImageFromURL - URL에서 이미지 다운로드 (백그라운드 저장용)
func (s *Service) DownloadImageFromURL(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// RefinePromptWithOpenAI - OpenAI GPT-4o로 프롬프트 정제
func (s *Service) RefinePromptWithOpenAI(ctx context.Context, userPrompt string, templatePrompt string) (string, error) {
	cfg := config.GetConfig()

	if cfg.OpenAIAPIKey == "" {
		log.Printf("⚠️ [Landing] OpenAI API key not configured, using original prompt")
		if templatePrompt != "" {
			return templatePrompt + ", " + userPrompt, nil
		}
		return userPrompt, nil
	}

	systemMessage := `You are an expert image generation prompt engineer. Your task is to:
1. Take the user's input (which may be in any language - Korean, Japanese, Chinese, etc.)
2. Understand their intent and what kind of image they want
3. Transform it into a clear, detailed English prompt optimized for image generation AI
4. Add helpful details about lighting, composition, style, and quality if not specified
5. Keep the prompt concise but descriptive (max 200 words)

IMPORTANT: Only output the refined English prompt, nothing else. No explanations, no quotes, just the prompt.`

	if templatePrompt != "" {
		systemMessage += fmt.Sprintf("\n\nThe user has selected a template with this base prompt: \"%s\". Incorporate their input while maintaining the template's style.", templatePrompt)
	}

	reqBody := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemMessage},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   500,
		Temperature: 0.7,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return userPrompt, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return userPrompt, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ [Landing] OpenAI API error: %v, using original prompt", err)
		if templatePrompt != "" {
			return templatePrompt + ", " + userPrompt, nil
		}
		return userPrompt, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️ [Landing] OpenAI API error: status %d, body: %s", resp.StatusCode, string(body))
		if templatePrompt != "" {
			return templatePrompt + ", " + userPrompt, nil
		}
		return userPrompt, nil
	}

	var openaiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		log.Printf("⚠️ [Landing] Failed to decode OpenAI response: %v", err)
		if templatePrompt != "" {
			return templatePrompt + ", " + userPrompt, nil
		}
		return userPrompt, nil
	}

	if len(openaiResp.Choices) == 0 || openaiResp.Choices[0].Message.Content == "" {
		if templatePrompt != "" {
			return templatePrompt + ", " + userPrompt, nil
		}
		return userPrompt, nil
	}

	refinedPrompt := strings.TrimSpace(openaiResp.Choices[0].Message.Content)
	log.Printf("✅ [Landing] Prompt refined: %s", truncateString(refinedPrompt, 100))
	return refinedPrompt, nil
}

// generateUUID - UUID 생성 헬퍼 (UUIDv4 형식)
func generateUUID() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rand.Uint32(),
		rand.Uint32()&0xffff,
		(rand.Uint32()&0x0fff)|0x4000, // version 4
		(rand.Uint32()&0x3fff)|0x8000, // variant
		rand.Uint64()&0xffffffffffff,
	)
}

// IsRunwareModel - Runware 모델 여부 확인
func IsRunwareModel(modelID string) bool {
	if modelID == "" {
		return false
	}
	// 접두사 기반 체크
	if strings.HasPrefix(modelID, "runware:") ||
		strings.HasPrefix(modelID, "civitai:") ||
		strings.HasPrefix(modelID, "bytedance:") {
		return true
	}
	// 접두사 없는 Runware 모델명 직접 체크 (템플릿 호환성)
	runwareModels := []string{
		"flux-schnell",
		"flux-dev",
		"flux-pro",
		"sdxl",
		"sd-turbo",
	}
	for _, model := range runwareModels {
		if modelID == model {
			return true
		}
	}
	return false
}

// IsGeminiModel - Gemini 모델 여부 확인
func IsGeminiModel(modelID string) bool {
	if modelID == "" {
		return true // 기본값은 Gemini
	}
	return strings.HasPrefix(modelID, "gemini:")
}

// IsMultiviewModel - Multiview 모델 여부 확인
func IsMultiviewModel(modelID string) bool {
	if modelID == "" {
		return false
	}
	return strings.HasPrefix(modelID, "multiview:")
}

// IsNanobananaModel - Nanobanana (Gemini 2.5 Flash) 모델 여부 확인
func IsNanobananaModel(modelID string) bool {
	if modelID == "" {
		return false
	}
	// gemini:gemini-2.5-flash-image 형식 또는 gemini-2.0-flash, gemini-2.5-flash 포함
	if strings.HasPrefix(modelID, "gemini:") {
		return true
	}
	if strings.Contains(modelID, "gemini-2.0-flash") || strings.Contains(modelID, "gemini-2.5-flash") {
		return true
	}
	return false
}
