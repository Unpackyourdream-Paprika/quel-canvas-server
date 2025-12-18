package landingdemo

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/fallback"
	"quel-canvas-server/modules/common/model"
)

// ProcessJob - Landing Job 처리 함수 (Worker에서 호출)
func ProcessJob(ctx context.Context, job *model.ProductionJob) {
	log.Printf("🚀 [Landing] Starting job processing: %s", job.JobID)

	// Service 초기화
	service := NewServiceWithDB()
	if service == nil {
		log.Printf("❌ [Landing] Failed to initialize service")
		return
	}

	// Job 데이터 로그
	log.Printf("📦 [Landing] Job Data:")
	log.Printf("   JobID: %s", job.JobID)
	log.Printf("   JobType: %s", job.JobType)
	log.Printf("   Status: %s", job.JobStatus)
	log.Printf("   TotalImages: %d", job.TotalImages)

	if job.ProductionID != nil {
		log.Printf("   ProductionID: %s", *job.ProductionID)
	}

	// Job Type에 따른 처리
	switch job.JobType {
	case "simple_general":
		log.Printf("📌 [Landing] Simple General Mode")
		processLandingSimpleGeneral(ctx, service, job)
	default:
		log.Printf("📌 [Landing] Default Mode (simple_general)")
		processLandingSimpleGeneral(ctx, service, job)
	}
}

// processLandingSimpleGeneral - Landing 이미지 생성 처리
func processLandingSimpleGeneral(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🚀 [Landing] Starting Simple General processing for job: %s", job.JobID)

	// Input Data 추출
	prompt := fallback.SafeString(job.JobInputData["prompt"], "best quality, masterpiece")
	aspectRatio := fallback.SafeAspectRatio(job.JobInputData["aspect-ratio"])
	quantity := job.TotalImages
	if quantity <= 0 || quantity > 4 {
		quantity = 4
	}
	userID := fallback.SafeString(job.JobInputData["userId"], "")

	log.Printf("📦 [Landing] Input: Prompt=%s, AspectRatio=%s, Quantity=%d, UserID=%s",
		truncateString(prompt, 50), aspectRatio, quantity, userID)

	// Status 업데이트 - processing
	if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusProcessing); err != nil {
		log.Printf("❌ [Landing] Failed to update job status: %v", err)
		return
	}

	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusProcessing); err != nil {
			log.Printf("⚠️ [Landing] Failed to update production status: %v", err)
		}
	}

	// 입력 이미지 다운로드 (있는 경우)
	var inputImages [][]byte
	if uploadedIds, ok := job.JobInputData["uploadedAttachIds"].([]interface{}); ok && len(uploadedIds) > 0 {
		for i, attachObj := range uploadedIds {
			attachMap, ok := attachObj.(map[string]interface{})
			if !ok {
				continue
			}
			attachIDFloat, ok := attachMap["attachId"].(float64)
			if !ok {
				continue
			}
			attachID := int(attachIDFloat)

			log.Printf("📥 [Landing] Downloading input image %d: AttachID=%d", i+1, attachID)
			imageData, err := service.DownloadImageFromStorage(attachID)
			if err != nil {
				log.Printf("❌ [Landing] Failed to download image %d: %v", attachID, err)
				continue
			}
			inputImages = append(inputImages, imageData)
		}
	}

	log.Printf("✅ [Landing] %d input images prepared", len(inputImages))

	// 이미지 생성 루프
	generatedAttachIds := []int{}
	completedCount := 0

	for i := 0; i < quantity; i++ {
		// 취소 체크
		if service.IsJobCancelled(job.JobID) {
			log.Printf("🛑 [Landing] Job %s cancelled", job.JobID)
			service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
			if job.ProductionID != nil {
				service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
			}
			return
		}

		log.Printf("🎨 [Landing] Generating image %d/%d...", i+1, quantity)

		// Gemini API 호출
		var generatedBase64 string
		var err error

		if len(inputImages) > 0 {
			// 입력 이미지가 있는 경우 - 카테고리 분류 후 생성
			categories := &ImageCategories{
				Clothing:    inputImages,
				Accessories: [][]byte{},
			}
			generatedBase64, err = service.GenerateImageWithGeminiMultiple(ctx, categories, prompt, aspectRatio)
		} else {
			// 입력 이미지가 없는 경우 - 텍스트만으로 생성
			generatedBase64, err = service.GenerateImageWithGeminiTextOnly(ctx, prompt, aspectRatio)
		}

		if err != nil {
			log.Printf("❌ [Landing] Gemini API failed for image %d: %v", i+1, err)
			if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "429") {
				log.Printf("🚨 [Landing] API error detected - stopping job")
				service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
				if job.ProductionID != nil {
					service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusFailed)
				}
				return
			}
			continue
		}

		// Base64 → []byte 변환
		generatedImageData, err := base64.StdEncoding.DecodeString(generatedBase64)
		if err != nil {
			log.Printf("❌ [Landing] Failed to decode image %d: %v", i+1, err)
			continue
		}

		// Storage 업로드
		filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
		if err != nil {
			log.Printf("❌ [Landing] Failed to upload image %d: %v", i+1, err)
			continue
		}

		// Attach 레코드 생성
		attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
		if err != nil {
			log.Printf("❌ [Landing] Failed to create attach record %d: %v", i+1, err)
			continue
		}

		// 크레딧 차감
		if job.ProductionID != nil && userID != "" {
			go func(aID int, prodID string, orgID *string) {
				if err := service.DeductCredits(context.Background(), userID, orgID, prodID, []int{aID}); err != nil {
					log.Printf("⚠️ [Landing] Failed to deduct credits for attach %d: %v", aID, err)
				}
			}(attachID, *job.ProductionID, job.OrgID)
		}

		generatedAttachIds = append(generatedAttachIds, attachID)
		completedCount++

		log.Printf("✅ [Landing] Image %d/%d completed: AttachID=%d", i+1, quantity, attachID)

		// 진행 상황 업데이트
		if err := service.UpdateJobProgress(ctx, job.JobID, completedCount, generatedAttachIds); err != nil {
			log.Printf("⚠️ [Landing] Failed to update progress: %v", err)
		}
	}

	// 최종 완료 처리
	finalStatus := model.StatusCompleted
	if completedCount == 0 {
		log.Printf("⚠️ [Landing] No images generated")
		finalStatus = model.StatusFailed
	}

	log.Printf("🏁 [Landing] Job %s finished: %d/%d images", job.JobID, completedCount, quantity)

	// Job 상태 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, finalStatus); err != nil {
		log.Printf("❌ [Landing] Failed to update final status: %v", err)
	}

	// Production 업데이트
	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, finalStatus); err != nil {
			log.Printf("⚠️ [Landing] Failed to update production status: %v", err)
		}

		if len(generatedAttachIds) > 0 {
			if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, generatedAttachIds); err != nil {
				log.Printf("⚠️ [Landing] Failed to update production attach_ids: %v", err)
			}
		}
	}

	log.Printf("✅ [Landing] Processing completed for job: %s", job.JobID)
}

// GenerateImageWithGeminiTextOnly - 텍스트만으로 이미지 생성
func (s *Service) GenerateImageWithGeminiTextOnly(ctx context.Context, prompt string, aspectRatio string) (string, error) {
	cfg := config.GetConfig()

	if aspectRatio == "" {
		aspectRatio = "1:1"
	}

	log.Printf("🎨 [Landing] Calling Gemini API (text-only) - prompt: %s, ratio: %s",
		truncateString(prompt, 50), aspectRatio)

	// Content 생성 (텍스트만)
	content := &genai.Content{
		Parts: []*genai.Part{
			genai.NewPartFromText(prompt),
		},
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
			Temperature: floatPtr(0.45),
		},
	)
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
	}

	// 응답에서 이미지 추출
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				imageBase64 := base64.StdEncoding.EncodeToString(part.InlineData.Data)
				log.Printf("✅ [Landing] Image generated: %d bytes", len(part.InlineData.Data))
				return imageBase64, nil
			}
		}
	}

	return "", fmt.Errorf("no image in response")
}
