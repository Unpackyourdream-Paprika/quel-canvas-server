package modify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
)

// ProcessModifyJob - Modify Job 처리 메인 로직
func (s *Service) ProcessModifyJob(ctx context.Context, jobID string) error {
	log.Printf("🎨 ========== Starting Modify Job: %s ==========", jobID)

	// 1. Job 데이터 조회
	job, err := s.FetchJobFromSupabase(jobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	// 2. Job 상태를 processing으로 업데이트
	if err := s.UpdateJobStatus(ctx, jobID, StatusProcessing); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// 3. JobInputData 파싱
	inputData, err := s.parseInputData(job.JobInputData)
	if err != nil {
		s.UpdateJobStatus(ctx, jobID, StatusFailed)
		return fmt.Errorf("failed to parse input data: %w", err)
	}

	log.Printf("📋 Job Info:")
	log.Printf("  - Production ID: %s", *job.ProductionID)
	log.Printf("  - Total Images: %d", job.TotalImages)
	log.Printf("  - User ID: %s", inputData.UserID)
	log.Printf("  - Original Attach ID: %d", inputData.OriginalAttachID)
	log.Printf("  - Prompt: %s", inputData.Prompt)

	// 4. 원본 이미지 다운로드 및 Base64 변환
	imageBase64, imageMimeType, err := s.downloadAndEncodeImage(inputData.OriginalImageURL)
	if err != nil {
		s.UpdateJobStatus(ctx, jobID, StatusFailed)
		return fmt.Errorf("failed to download original image: %w", err)
	}

	// 5. Mask 데이터 추출 (Base64에서 data URL prefix 제거)
	maskBase64 := extractBase64Data(inputData.MaskDataURL)

	// 6. Reference 이미지 처리 (있는 경우)
	var referenceBase64 string
	var referenceMimeType string
	if inputData.ReferenceImageDataURL != nil && *inputData.ReferenceImageDataURL != "" {
		referenceBase64 = extractBase64Data(*inputData.ReferenceImageDataURL)
		referenceMimeType = extractMimeType(*inputData.ReferenceImageDataURL)
		log.Printf("📷 Reference image provided (type: %s)", referenceMimeType)
	}

	// 7. 생성된 이미지들을 저장할 배열
	generatedAttachIDs := make([]int64, 0, inputData.Quantity)
	completedCount := 0
	failedCount := 0

	// 8. Quantity만큼 이미지 생성 (순차 처리)
	for i := 0; i < inputData.Quantity; i++ {
		log.Printf("🖼️  Generating image %d/%d...", i+1, inputData.Quantity)

		// Gemini API로 Inpaint 수행
		generatedImageBase64, generatedMimeType, err := s.performInpaint(
			ctx,
			imageBase64,
			imageMimeType,
			maskBase64,
			inputData.Prompt,
			referenceBase64,
			referenceMimeType,
		)

		if err != nil {
			log.Printf("❌ Failed to generate image %d/%d: %v", i+1, inputData.Quantity, err)
			failedCount++
			continue
		}

		// Supabase Storage에 업로드 및 DB 저장
		attachID, err := s.uploadAndSaveImage(
			ctx,
			generatedImageBase64,
			generatedMimeType,
			*job.ProductionID,
			inputData.UserID,
			i,
		)

		if err != nil {
			log.Printf("❌ Failed to save image %d/%d: %v", i+1, inputData.Quantity, err)
			failedCount++
			continue
		}

		generatedAttachIDs = append(generatedAttachIDs, attachID)
		completedCount++

		// 진행 상황 업데이트
		s.UpdateJobProgress(ctx, jobID, completedCount, failedCount, generatedAttachIDs, *job.ProductionID)

		log.Printf("✅ Image %d/%d generated successfully (attach_id: %d)", i+1, inputData.Quantity, attachID)

		// API Rate Limit 방지
		if i < inputData.Quantity-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 9. Production 상태 및 image_count 업데이트
	productionStatus := "completed"
	if completedCount == 0 {
		productionStatus = "failed"
	}

	if err := s.updateProductionStatus(ctx, *job.ProductionID, completedCount, productionStatus); err != nil {
		log.Printf("⚠️  Failed to update production: %v", err)
	}

	// 10. Job 상태 업데이트
	finalStatus := StatusCompleted
	if completedCount == 0 {
		finalStatus = StatusFailed
	}

	if err := s.UpdateJobStatus(ctx, jobID, finalStatus); err != nil {
		return fmt.Errorf("failed to update final job status: %w", err)
	}

	log.Printf("🎉 ========== Modify Job Completed: %s ==========", jobID)
	log.Printf("📊 Results: %d succeeded, %d failed out of %d total", completedCount, failedCount, inputData.Quantity)

	return nil
}

// parseInputData - JobInputData 파싱
func (s *Service) parseInputData(data map[string]interface{}) (*ModifyInputData, error) {
	inputData := &ModifyInputData{}

	if v, ok := data["originalImageUrl"].(string); ok {
		inputData.OriginalImageURL = v
	}
	if v, ok := data["originalAttachId"].(float64); ok {
		inputData.OriginalAttachID = int(v)
	}
	if v, ok := data["maskDataUrl"].(string); ok {
		inputData.MaskDataURL = v
	}
	if v, ok := data["prompt"].(string); ok {
		inputData.Prompt = v
	}
	if v, ok := data["referenceImageDataUrl"].(string); ok && v != "" {
		inputData.ReferenceImageDataURL = &v
	}
	if v, ok := data["quantity"].(float64); ok {
		inputData.Quantity = int(v)
	}
	if v, ok := data["userId"].(string); ok {
		inputData.UserID = v
		inputData.QuelMemberID = v
	}

	return inputData, nil
}

// performInpaint - Gemini API를 사용한 이미지 인페인팅
func (s *Service) performInpaint(
	ctx context.Context,
	imageBase64 string,
	imageMimeType string,
	maskBase64 string,
	prompt string,
	referenceBase64 string,
	referenceMimeType string,
) (string, string, error) {

	log.Printf("🤖 Starting inpaint with Gemini API...")

	// 기본 프롬프트 설정
	if prompt == "" {
		prompt = "Seamlessly fill in the selected area with natural content"
	}

	// 템플릿 기반 프롬프트 구성
	inpaintPrompt := fmt.Sprintf(`Using the provided image, change only the [%s] to [new element/description]. Keep everything else in the image exactly the same, preserving the original style, lighting, and composition.`, prompt)

	// Reference 이미지가 있는 경우 프롬프트에 추가
	if referenceBase64 != "" {
		inpaintPrompt += "\n\nUse the reference image as a style guide for the modification."
	}

	// Base64 디코딩
	imageData := mustDecodeBase64(imageBase64)
	maskData := mustDecodeBase64(maskBase64)

	if len(imageData) == 0 || len(maskData) == 0 {
		return "", "", fmt.Errorf("failed to decode image or mask data")
	}

	log.Printf("📤 Sending inpaint request to Gemini...")
	log.Printf("  - Prompt: %s", inpaintPrompt)
	log.Printf("  - Image size: %d bytes", len(imageData))
	log.Printf("  - Mask size: %d bytes", len(maskData))

	// Content 생성 - Parts 배열 구성
	parts := []*genai.Part{
		genai.NewPartFromText(inpaintPrompt),
		genai.NewPartFromBytes(imageData, imageMimeType),
		genai.NewPartFromBytes(maskData, "image/png"), // 마스크는 PNG
	}

	// Reference 이미지 추가 (있는 경우)
	if referenceBase64 != "" && referenceMimeType != "" {
		referenceData := mustDecodeBase64(referenceBase64)
		if len(referenceData) > 0 {
			parts = append(parts, genai.NewPartFromBytes(referenceData, referenceMimeType))
			log.Printf("  - Reference image size: %d bytes", len(referenceData))
		}
	}

	content := &genai.Content{
		Parts: parts,
	}

	// Gemini API 호출 (gemini-2.5-flash-image 모델 사용)
	cfg := config.GetConfig()
	result, err := s.genaiClient.Models.GenerateContent(
		ctx,
		cfg.GeminiModel, // "gemini-2.5-flash-image"
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			ImageConfig: &genai.ImageConfig{
				AspectRatio: "16:9", // 기본 aspect ratio
			},
		},
	)
	if err != nil {
		return "", "", fmt.Errorf("Gemini API request failed: %w", err)
	}

	// 응답 검증
	if len(result.Candidates) == 0 {
		return "", "", fmt.Errorf("no candidates in Gemini response")
	}

	// 생성된 이미지 데이터 추출
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			// InlineData 확인 (이미지는 InlineData로 반환됨)
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				log.Printf("✅ Gemini inpaint completed (size: %d bytes, type: %s)",
					len(part.InlineData.Data), part.InlineData.MIMEType)
				// Base64로 인코딩하여 반환
				return base64.StdEncoding.EncodeToString(part.InlineData.Data), part.InlineData.MIMEType, nil
			}
		}
	}

	return "", "", fmt.Errorf("no image data in Gemini response")
}

// uploadAndSaveImage - Supabase Storage 업로드 및 DB 저장
func (s *Service) uploadAndSaveImage(
	ctx context.Context,
	imageBase64 string,
	mimeType string,
	productionID string,
	userID string,
	index int,
) (int64, error) {

	// Base64 디코딩
	imageData, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		return 0, fmt.Errorf("failed to decode base64: %w", err)
	}

	// 파일명 생성
	fileName := fmt.Sprintf("modify_%s_%d_%d.png", productionID, index, time.Now().Unix())
	filePath := fmt.Sprintf("%s/%s", userID, fileName)

	// Supabase Storage 업로드 (HTTP API 직접 호출)
	log.Printf("☁️  Uploading to Supabase Storage: %s", filePath)

	cfg := config.GetConfig()
	uploadURL := fmt.Sprintf("%s/storage/v1/object/attachments/%s", cfg.SupabaseURL, filePath)

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(imageData))
	if err != nil {
		return 0, fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceKey)
	req.Header.Set("Content-Type", mimeType)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to upload to storage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("storage upload failed (status %d): %s", resp.StatusCode, string(body))
	}

	// quel_attach 레코드 생성
	attach := map[string]interface{}{
		"attach_original_name": fileName,
		"attach_file_name":     fileName,
		"attach_file_path":     filePath,
		"attach_file_size":     len(imageData),
		"attach_file_type":     "image/png",
		"attach_directory":     filePath,
		"attach_storage_type":  "supabase",
	}

	var attachResults []Attach
	data, _, err := s.supabase.From("quel_attach").
		Insert(attach, false, "", "", "returning").
		Execute()

	if err != nil {
		return 0, fmt.Errorf("failed to create attach record: %w", err)
	}

	if err := json.Unmarshal(data, &attachResults); err != nil {
		return 0, fmt.Errorf("failed to parse attach response: %w", err)
	}

	if len(attachResults) == 0 {
		return 0, fmt.Errorf("no attach record created")
	}

	attachID := attachResults[0].AttachID

	// quel_production_attach 관계 생성
	productionAttach := map[string]interface{}{
		"production_id": productionID,
		"attach_id":     attachID,
	}

	_, _, err = s.supabase.From("quel_production_attach").
		Insert(productionAttach, false, "", "", "").
		Execute()

	if err != nil {
		log.Printf("⚠️  Failed to create production_attach relation: %v", err)
	}

	log.Printf("✅ Image saved (attach_id: %d)", attachID)
	return attachID, nil
}

// updateProductionStatus - Production 상태 및 이미지 개수 업데이트
func (s *Service) updateProductionStatus(ctx context.Context, productionID string, imageCount int, status string) error {
	_, _, err := s.supabase.From("quel_production_photo").
		Update(map[string]interface{}{
			"generated_image_count": imageCount,
			"production_status":     status,
		}, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update production: %w", err)
	}

	log.Printf("✅ Production %s updated: %d images, status: %s", productionID, imageCount, status)
	return nil
}

// downloadAndEncodeImage - 이미지 다운로드 및 Base64 인코딩
func (s *Service) downloadAndEncodeImage(url string) (string, string, error) {
	log.Printf("📥 Downloading image from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read image data: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/png"
	}

	base64Data := base64.StdEncoding.EncodeToString(imageData)
	log.Printf("✅ Image downloaded (size: %d bytes, type: %s)", len(imageData), mimeType)

	return base64Data, mimeType, nil
}

// Helper functions
func extractBase64Data(dataURL string) string {
	if strings.Contains(dataURL, ",") {
		parts := strings.SplitN(dataURL, ",", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return dataURL
}

func extractMimeType(dataURL string) string {
	if strings.HasPrefix(dataURL, "data:") {
		parts := strings.SplitN(dataURL, ";", 2)
		if len(parts) >= 1 {
			return strings.TrimPrefix(parts[0], "data:")
		}
	}
	return "image/png"
}

func mustDecodeBase64(encoded string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Printf("⚠️  Base64 decode error: %v", err)
		return []byte{}
	}
	return decoded
}

func boolPtr(b bool) *bool {
	return &b
}
