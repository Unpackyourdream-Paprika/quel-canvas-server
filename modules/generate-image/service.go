package generateimage

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

	"github.com/google/generative-ai-go/genai"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/supabase-community/supabase-go"
	"google.golang.org/api/option"
)

type Service struct {
	supabase *supabase.Client
}

func NewService() *Service {
	config := GetConfig()

	// Supabase 클라이언트 초기화
	client, err := supabase.NewClient(config.SupabaseURL, config.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ Failed to create Supabase client: %v", err)
		return nil
	}

	log.Println("✅ Supabase client initialized")
	return &Service{
		supabase: client,
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

	log.Printf("🎨 Calling Gemini API with prompt length: %d, aspect-ratio: %s", len(prompt), aspectRatio)

	// Gemini 클라이언트 생성
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.GeminiAPIKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	// 모델 선택
	model := client.GenerativeModel(config.GeminiModel)

	// Base64 디코딩
	imageData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	// API 호출 (WithGenerationConfig 사용)
	log.Printf("📤 Sending request to Gemini API with aspect-ratio: %s", aspectRatio)
	resp, err := model.GenerateContent(ctx,
		genai.Text(prompt+"\n\nPlease generate 1 different variation of this image."),
		genai.ImageData("png", imageData),
		genai.WithGenerationConfig(genai.GenerationConfig{
			AspectRatio:    aspectRatio,
			NumberOfImages: 1,
		}),
	)
	if err != nil {
		return "", fmt.Errorf("Gemini API call failed: %w", err)
	}

	// 응답 처리
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			// 이미지 데이터 찾기
			if blob, ok := part.(genai.Blob); ok {
				log.Printf("✅ Received image from Gemini: %d bytes", len(blob.Data))
				// Base64로 인코딩하여 반환
				return base64.StdEncoding.EncodeToString(blob.Data), nil
			}
		}
	}

	return "", fmt.Errorf("no image data in response")
}

// GenerateImageWithGeminiMultiple - Gemini API로 여러 입력 이미지 기반 이미지 생성
func (s *Service) GenerateImageWithGeminiMultiple(ctx context.Context, base64Images []string, prompt string, aspectRatio string) (string, error) {
	config := GetConfig()

	// aspect-ratio 기본값 처리
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	log.Printf("🎨 Calling Gemini API with %d input images, prompt length: %d, aspect-ratio: %s", len(base64Images), len(prompt), aspectRatio)

	// Gemini 클라이언트 생성
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.GeminiAPIKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	// 모델 선택
	model := client.GenerativeModel(config.GeminiModel)

	// Content Parts 생성 - 프롬프트 먼저, 그 다음 여러 이미지
	parts := []genai.Part{
		genai.Text(prompt + "\n\nGenerate exactly 1 image that follows these instructions. The output must be a single, transformed portrait photo."),
	}

	// 모든 입력 이미지를 Parts에 추가
	for i, base64Image := range base64Images {
		imageData, err := base64.StdEncoding.DecodeString(base64Image)
		if err != nil {
			log.Printf("⚠️  Failed to decode base64 image %d: %v", i, err)
			continue
		}

		parts = append(parts, genai.ImageData("png", imageData))
		log.Printf("📎 Added input image %d to request (%d bytes)", i+1, len(imageData))
	}

	// WithGenerationConfig 추가
	parts = append(parts, genai.WithGenerationConfig(genai.GenerationConfig{
		AspectRatio:    aspectRatio,
		NumberOfImages: 1,
	}))

	// API 호출
	log.Printf("📤 Sending request to Gemini API with %d parts (1 text + %d images + 1 config)...", len(parts), len(base64Images))
	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", fmt.Errorf("Gemini API call failed: %w", err)
	}

	// 응답 처리
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			// 이미지 데이터 찾기
			if blob, ok := part.(genai.Blob); ok {
				log.Printf("✅ Received image from Gemini: %d bytes", len(blob.Data))
				// Base64로 인코딩하여 반환
				return base64.StdEncoding.EncodeToString(blob.Data), nil
			}
		}
	}

	return "", fmt.Errorf("no image data in response")
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