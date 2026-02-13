package modify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
	geminiretry "quel-canvas-server/modules/common/gemini"
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
			inputData.Layers,
			referenceBase64,
			referenceMimeType,
			inputData.AspectRatio,
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
	if v, ok := data["aspect-ratio"].(string); ok && v != "" {
		inputData.AspectRatio = v
	} else {
		inputData.AspectRatio = "16:9" // default
	}
	if v, ok := data["userId"].(string); ok {
		inputData.UserID = v
		inputData.QuelMemberID = v
	}

	// layers 파싱
	if v, ok := data["layers"].([]interface{}); ok {
		for _, item := range v {
			if layerMap, ok := item.(map[string]interface{}); ok {
				layer := Layer{}
				if color, ok := layerMap["color"].(string); ok {
					layer.Color = color
				}
				if prompt, ok := layerMap["prompt"].(string); ok {
					layer.Prompt = prompt
				}
				if refImg, ok := layerMap["referenceImage"].(string); ok && refImg != "" {
					layer.ReferenceImage = &refImg
				}
				// Color만 있으면 layer 추가 (prompt나 referenceImage 중 하나만 있어도 됨)
				if layer.Color != "" {
					inputData.Layers = append(inputData.Layers, layer)
					log.Printf("  - Layer %s: prompt='%s', hasRefImg=%v", layer.Color, layer.Prompt, layer.ReferenceImage != nil)
				}
			}
		}
		log.Printf("📋 Parsed %d layers", len(inputData.Layers))
	}

	return inputData, nil
}

// overlayMaskOnImage - 원본 이미지 위에 마스크를 합성
func (s *Service) overlayMaskOnImage(imageData []byte, maskData []byte) ([]byte, error) {
	log.Printf("🎨 Overlaying mask on original image...")

	// 원본 이미지 디코딩
	origImg, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode original image: %w", err)
	}

	// 마스크 이미지 디코딩
	maskImg, _, err := image.Decode(bytes.NewReader(maskData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode mask image: %w", err)
	}

	// 결과 이미지 생성 (원본 크기)
	bounds := origImg.Bounds()
	result := image.NewRGBA(bounds)

	// 원본 이미지 복사
	draw.Draw(result, bounds, origImg, image.Point{}, draw.Src)

	// 마스크 이미지 오버레이 (마스크 크기가 다를 수 있으므로 조정)
	maskBounds := maskImg.Bounds()
	if maskBounds.Dx() != bounds.Dx() || maskBounds.Dy() != bounds.Dy() {
		log.Printf("⚠️  Mask size (%dx%d) differs from image size (%dx%d), drawing as-is",
			maskBounds.Dx(), maskBounds.Dy(), bounds.Dx(), bounds.Dy())
	}

	// 마스크를 원본 위에 오버레이 (Over 모드 - 투명 부분은 원본 유지)
	draw.Draw(result, bounds, maskImg, image.Point{}, draw.Over)

	// PNG로 인코딩
	var buf bytes.Buffer
	if err := png.Encode(&buf, result); err != nil {
		return nil, fmt.Errorf("failed to encode merged image: %w", err)
	}

	log.Printf("✅ Mask overlayed successfully (merged size: %d bytes)", buf.Len())
	return buf.Bytes(), nil
}

// performInpaint - Gemini API를 사용한 이미지 인페인팅
func (s *Service) performInpaint(
	ctx context.Context,
	imageBase64 string,
	imageMimeType string,
	maskBase64 string,
	prompt string,
	layers []Layer,
	referenceBase64 string,
	referenceMimeType string,
	aspectRatio string,
) (string, string, error) {

	log.Printf("🤖 Starting inpaint with Gemini API...")

	// 프롬프트 구성 - layers가 있으면 색상별 지시사항 통합
	var inpaintPrompt string

	// 강화된 공통 지시사항: 마스킹된 영역만 수정하고 나머지는 절대 건드리지 않도록
	strictInpaintInstruction := `⚠️ ABSOLUTE PRIORITY - PRESERVE ORIGINAL IMAGE QUALITY:

1. COLOR PRESERVATION (CRITICAL):
   - Maintain EXACT color tone, saturation, vibrancy, and richness of the original image
   - DO NOT wash out, desaturate, fade, or flatten colors in ANY part of the image
   - Preserve the original color contrast, brightness, and visual impact
   - Keep the same color temperature and color grading

2. DEPTH & PERSPECTIVE PRESERVATION (CRITICAL):
   - Maintain the original 3D depth, dimensional quality, and spatial relationships
   - Preserve perspective, distance, and sense of space exactly as in the original
   - DO NOT flatten or make the image look more 2D or less realistic
   - Keep the same foreground/background separation and depth cues

3. LIGHTING & ATMOSPHERE PRESERVATION (CRITICAL):
   - Maintain EXACT same lighting conditions, light direction, and intensity
   - Preserve all shadows, highlights, reflections, and light interactions
   - Keep the original atmosphere, mood, and photographic quality
   - DO NOT change the overall lighting balance or create new light sources

4. FRAMING & COMPOSITION PRESERVATION (CRITICAL):
   - Maintain the EXACT same framing, composition, and zoom level as the original image
   - DO NOT zoom in, zoom out, crop, or reframe the image in any way
   - Keep the exact same subject positioning, size, and placement within the frame
   - Preserve the original camera angle, distance, and field of view
   - The overall composition and layout must remain identical to the original

CRITICAL INPAINTING RULES:
1. ONLY modify the areas marked with colored paint strokes. These colored areas are the ONLY parts you should change.
2. DO NOT modify, alter, change, or regenerate ANY other part of the image outside the painted areas.
3. The unpainted areas must remain PIXEL-PERFECT identical to the original - same colors, same textures, same lighting, same everything.
4. Remove all paint stroke markings from the final output - no trace of the colored markers should remain.
5. The modification should blend naturally with the surrounding unchanged areas while preserving all qualities above.
6. Even if other parts of the image look similar to the marked area, DO NOT change them.`

	if len(layers) > 0 {
		// layers에서 색상별 프롬프트 추출하여 통합
		var layerInstructions []string
		for _, layer := range layers {
			instruction := fmt.Sprintf("%s colored area: %s", layer.Color, layer.Prompt)
			layerInstructions = append(layerInstructions, instruction)
		}
		combinedInstructions := strings.Join(layerInstructions, " | ")
		inpaintPrompt = fmt.Sprintf(`You are performing a PRECISE inpainting task.

%s

TASK: Modify ONLY the colored paint stroke areas with these instructions:
%s

Remember: Areas WITHOUT paint strokes must stay EXACTLY as they are in the original image. Do not touch them at all.`, strictInpaintInstruction, combinedInstructions)
		log.Printf("📝 Using layers prompt: %s", combinedInstructions)
	} else if prompt != "" {
		// 기존 prompt 사용
		inpaintPrompt = fmt.Sprintf(`You are performing a PRECISE inpainting task.

%s

TASK: Modify ONLY the areas marked with colored paint strokes according to this instruction: %s

Remember: Areas WITHOUT paint strokes must stay EXACTLY as they are in the original image. Do not touch them at all.`, strictInpaintInstruction, prompt)
	} else {
		// 기본 프롬프트
		inpaintPrompt = fmt.Sprintf(`You are performing a PRECISE inpainting task.

%s

TASK: Seamlessly fill in ONLY the areas marked with colored paint strokes with natural content that matches the surrounding context.

Remember: Areas WITHOUT paint strokes must stay EXACTLY as they are in the original image. Do not touch them at all.`, strictInpaintInstruction)
	}

	// Reference 이미지 수집 (전역 + 레이어별)
	var referenceImages []struct {
		base64   string
		mimeType string
		desc     string
	}

	// 전역 참조 이미지
	if referenceBase64 != "" {
		referenceImages = append(referenceImages, struct {
			base64   string
			mimeType string
			desc     string
		}{referenceBase64, referenceMimeType, "global style"})
	}

	// 레이어별 참조 이미지
	for _, layer := range layers {
		if layer.ReferenceImage != nil && *layer.ReferenceImage != "" {
			refBase64 := extractBase64Data(*layer.ReferenceImage)
			refMimeType := extractMimeType(*layer.ReferenceImage)
			referenceImages = append(referenceImages, struct {
				base64   string
				mimeType string
				desc     string
			}{refBase64, refMimeType, fmt.Sprintf("reference for %s area", layer.Color)})
			log.Printf("📷 Layer %s has reference image", layer.Color)
		}
	}

	// Reference 이미지가 있는 경우 프롬프트에 추가
	if len(referenceImages) > 0 {
		inpaintPrompt += "\n\nUse the reference image(s) as a style guide for the modification."
	}

	// Base64 디코딩
	imageData := mustDecodeBase64(imageBase64)
	maskData := mustDecodeBase64(maskBase64)

	if len(imageData) == 0 || len(maskData) == 0 {
		return "", "", fmt.Errorf("failed to decode image or mask data")
	}

	// 원본 이미지 + 마스크 합성
	mergedImageData, err := s.overlayMaskOnImage(imageData, maskData)
	if err != nil {
		return "", "", fmt.Errorf("failed to overlay mask: %w", err)
	}

	log.Printf("📤 Sending inpaint request to Gemini...")
	log.Printf("  - Prompt: %s", inpaintPrompt)
	log.Printf("  - Merged image size: %d bytes", len(mergedImageData))

	// Content 생성 - 합성된 이미지만 전달 (마스크 따로 안 보냄)
	parts := []*genai.Part{
		genai.NewPartFromText(inpaintPrompt),
		genai.NewPartFromBytes(mergedImageData, "image/png"), // 합성된 이미지
	}

	// Reference 이미지들 추가 (전역 + 레이어별)
	for _, refImg := range referenceImages {
		referenceData := mustDecodeBase64(refImg.base64)
		if len(referenceData) > 0 {
			parts = append(parts, genai.NewPartFromBytes(referenceData, refImg.mimeType))
			log.Printf("  - Reference image (%s): %d bytes", refImg.desc, len(referenceData))
		}
	}

	content := &genai.Content{
		Parts: parts,
	}

	// Gemini API 호출 (gemini-2.5-flash-image 모델 사용)
	cfg := config.GetConfig()

	log.Printf("📐 Using aspect ratio: %s", aspectRatio)

	result, err := geminiretry.GenerateContentWithRetry(
		ctx,
		cfg.GeminiAPIKeys,
		cfg.GeminiModel, // "gemini-2.5-flash-image"
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			ImageConfig: &genai.ImageConfig{
				AspectRatio: aspectRatio,
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
		// FinishReason 먼저 확인 (차단 여부 체크)
		if candidate.FinishReason != "" {
			log.Printf("⚠️ Gemini finish reason: %s", candidate.FinishReason)
		}

		// SafetyRatings 확인
		if len(candidate.SafetyRatings) > 0 {
			for _, rating := range candidate.SafetyRatings {
				if rating.Blocked {
					log.Printf("🚫 Gemini blocked by safety: category=%s, probability=%s",
						rating.Category, rating.Probability)
				}
			}
		}

		if candidate.Content == nil {
			log.Printf("⚠️ Gemini candidate has nil content (FinishReason: %s)", candidate.FinishReason)
			continue
		}

		for _, part := range candidate.Content.Parts {
			// 텍스트 응답 확인 (거부 메시지일 수 있음)
			if part.Text != "" {
				log.Printf("📝 Gemini returned text response: %s", part.Text)
			}

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

	// attach_ids는 UpdateJobProgress에서 quel_production_photo.attach_ids 배열로 업데이트됨
	log.Printf("✅ Image saved to quel_attach (attach_id: %d)", attachID)
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
