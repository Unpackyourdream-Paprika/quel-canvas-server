package multiview

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sync"

	"quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/database"
	"quel-canvas-server/modules/common/model"
	redisutil "quel-canvas-server/modules/common/redis"

	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
)

// ProcessJob - Multiview Job 처리 (다른 모듈과 동일한 패턴)
func ProcessJob(ctx context.Context, job *model.ProductionJob) {
	log.Printf("🌐 [Multiview] Starting job processing: %s", job.JobID)

	// Service 초기화
	service := NewService()
	if service == nil {
		log.Printf("❌ [Multiview] Failed to initialize service for job: %s", job.JobID)
		updateJobFailed(job.JobID, "Failed to initialize multiview service")
		return
	}

	// Job Type에 따라 분기
	switch job.JobType {
	case "multiview", "multiview_360":
		log.Printf("🔄 [Multiview] Processing multiview_360 job: %s", job.JobID)
		processMultiview360(ctx, service, job)
	default:
		log.Printf("⚠️ [Multiview] Unknown job_type: %s, treating as multiview_360", job.JobType)
		processMultiview360(ctx, service, job)
	}
}

// processMultiview360 - 360도 다각도 이미지 생성 처리
func processMultiview360(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🎯 [Multiview] Starting 360 processing for job: %s", job.JobID)

	cfg := config.GetConfig()
	dbClient := database.NewClient()

	// Phase 1: Input Data 추출
	inputData := job.JobInputData
	if inputData == nil {
		log.Printf("❌ [Multiview] Missing job_input_data")
		updateJobFailed(job.JobID, "Missing job input data")
		return
	}

	// sourceImageBase64 또는 sourceAttachId 추출 (원본 이미지)
	var sourceImageData []byte
	var err error

	// 우선 base64 데이터 확인
	if sourceBase64, ok := inputData["sourceImageBase64"].(string); ok && sourceBase64 != "" {
		log.Printf("📦 [Multiview] Using base64 source image")
		sourceImageData, err = base64.StdEncoding.DecodeString(sourceBase64)
		if err != nil {
			log.Printf("❌ [Multiview] Failed to decode base64 image: %v", err)
			updateJobFailed(job.JobID, "Failed to decode base64 image")
			return
		}
		log.Printf("✅ [Multiview] Base64 source image decoded: %d bytes", len(sourceImageData))
	} else if sourceAttachIDFloat, ok := inputData["sourceAttachId"].(float64); ok {
		// base64가 없으면 attachId로 다운로드
		sourceAttachID := int(sourceAttachIDFloat)
		log.Printf("📦 [Multiview] Using sourceAttachId: %d", sourceAttachID)
		dbClient := database.NewClient()
		sourceImageData, err = dbClient.DownloadImageFromStorage(sourceAttachID)
		if err != nil {
			log.Printf("❌ [Multiview] Failed to download source image: %v", err)
			updateJobFailed(job.JobID, "Failed to download source image")
			return
		}
		log.Printf("✅ [Multiview] Source image downloaded: %d bytes", len(sourceImageData))
	} else {
		log.Printf("❌ [Multiview] Missing both sourceImageBase64 and sourceAttachId")
		updateJobFailed(job.JobID, "Missing source image (base64 or attach ID)")
		return
	}

	// userId 추출
	userID, _ := inputData["userId"].(string)
	if userID == "" && job.QuelMemberID != nil {
		userID = *job.QuelMemberID
	}
	if userID == "" {
		log.Printf("❌ [Multiview] Missing userId")
		updateJobFailed(job.JobID, "Missing user ID")
		return
	}

	// angles 추출 (기본: DefaultAngles)
	angles := DefaultAngles
	if anglesInterface, ok := inputData["angles"].([]interface{}); ok && len(anglesInterface) > 0 {
		angles = make([]int, 0, len(anglesInterface))
		for _, a := range anglesInterface {
			if angleFloat, ok := a.(float64); ok {
				angles = append(angles, int(angleFloat))
			}
		}
	}

	// aspectRatio 추출 (기본: "1:1")
	aspectRatio := "1:1"
	if ar, ok := inputData["aspectRatio"].(string); ok && ar != "" {
		aspectRatio = ar
	}

	// category 추출
	category, _ := inputData["category"].(string)

	// originalPrompt 추출
	originalPrompt, _ := inputData["originalPrompt"].(string)

	// rotateBackground 추출 (배경도 회전할지 여부)
	rotateBackground, _ := inputData["rotateBackground"].(bool)

	log.Printf("📦 [Multiview] Input: userId=%s, angles=%v, aspectRatio=%s, rotateBackground=%v",
		userID, angles, aspectRatio, rotateBackground)

	// Phase 2: Status 업데이트 → processing
	dbClient = database.NewClient()
	if err = dbClient.UpdateJobStatus(ctx, job.JobID, model.StatusProcessing); err != nil {
		log.Printf("⚠️ [Multiview] Failed to update job status: %v", err)
	}

	// Phase 3: 크레딧 확인
	requiredCredits := len(angles) * cfg.ImagePerPrice
	credits, err := service.CheckUserCredits(ctx, userID)
	if err != nil {
		log.Printf("⚠️ [Multiview] Failed to check credits: %v", err)
	} else if credits < requiredCredits {
		log.Printf("❌ [Multiview] Insufficient credits: required=%d, available=%d", requiredCredits, credits)
		updateJobFailed(job.JobID, fmt.Sprintf("Insufficient credits. Required: %d, Available: %d", requiredCredits, credits))
		return
	}

	// Phase 4는 이미 위에서 처리됨 (sourceImageData가 이미 준비됨)

	// Phase 5: 레퍼런스 이미지 다운로드 (있는 경우)
	referenceMap := make(map[int][]byte)
	if refImagesInterface, ok := inputData["referenceImages"].([]interface{}); ok {
		for _, refInterface := range refImagesInterface {
			refMap, ok := refInterface.(map[string]interface{})
			if !ok {
				continue
			}
			refAttachIDFloat, ok := refMap["attachId"].(float64)
			if !ok {
				continue
			}
			refAngleFloat, ok := refMap["angle"].(float64)
			if !ok {
				continue
			}

			refAttachID := int(refAttachIDFloat)
			refAngle := int(refAngleFloat)

			refData, err := dbClient.DownloadImageFromStorage(refAttachID)
			if err != nil {
				log.Printf("⚠️ [Multiview] Failed to download reference image for angle %d: %v", refAngle, err)
				continue
			}
			referenceMap[refAngle] = refData
			log.Printf("📎 [Multiview] Reference image loaded for angle %d", refAngle)
		}
	}

	// Phase 6: 각 각도별 이미지 생성 (병렬 처리)
	var generatedImages []GeneratedAngleImage
	var generatedAttachIDs []interface{}
	var totalCreditsUsed int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Semaphore로 동시 처리 수 제한 (최대 2개)
	semaphore := make(chan struct{}, 2)

	for _, angle := range angles {
		wg.Add(1)
		go func(currentAngle int) {
			defer wg.Done()
			semaphore <- struct{}{}        // 세마포어 획득
			defer func() { <-semaphore }() // 완료 후 반환

			log.Printf("🎨 [Multiview] Generating angle %d (%s)...", currentAngle, GetAngleLabel(currentAngle))

			// Job 취소 확인
			rdb := redisutil.Connect(cfg)
			if rdb != nil && redisutil.IsJobCancelled(rdb, job.JobID) {
				log.Printf("⚠️ [Multiview] Job cancelled, skipping angle %d", currentAngle)
				return
			}

			var result GeneratedAngleImage

			// 0도는 원본 이미지 그대로 사용
			if currentAngle == 0 {
				filePath, fileSize, err := service.UploadImageToStorage(ctx, sourceImageData, userID, currentAngle)
				if err != nil {
					log.Printf("⚠️ [Multiview] Failed to upload source image: %v", err)
					result = GeneratedAngleImage{
						Angle:        currentAngle,
						AngleLabel:   GetAngleLabel(currentAngle),
						Success:      false,
						ErrorMessage: "Failed to save source image",
					}
				} else {
					attachID, _ := service.CreateAttachRecord(ctx, filePath, fileSize)
					imageURL := cfg.SupabaseStorageBaseURL + filePath

					result = GeneratedAngleImage{
						Angle:      currentAngle,
						AngleLabel: GetAngleLabel(currentAngle),
						ImageURL:   imageURL,
						AttachID:   attachID,
						Success:    true,
					}
				}
			} else {
				// Gemini API로 이미지 생성
				hasReference := false
				var refData []byte
				if rd, ok := referenceMap[currentAngle]; ok {
					refData = rd
					hasReference = true
				}

				imageData, err := service.GenerateSingleAngle(ctx, sourceImageData, refData, currentAngle, aspectRatio, category, originalPrompt, hasReference, rotateBackground)
				if err != nil {
					log.Printf("❌ [Multiview] Failed to generate angle %d: %v", currentAngle, err)
					result = GeneratedAngleImage{
						Angle:        currentAngle,
						AngleLabel:   GetAngleLabel(currentAngle),
						Success:      false,
						ErrorMessage: fmt.Sprintf("Generation failed: %v", err),
					}
				} else {
					// Storage에 업로드
					filePath, fileSize, err := service.UploadImageToStorage(ctx, imageData, userID, currentAngle)
					if err != nil {
						log.Printf("⚠️ [Multiview] Failed to upload image for angle %d: %v", currentAngle, err)
						result = GeneratedAngleImage{
							Angle:       currentAngle,
							AngleLabel:  GetAngleLabel(currentAngle),
							ImageBase64: base64.StdEncoding.EncodeToString(imageData),
							Success:     true,
						}
					} else {
						attachID, _ := service.CreateAttachRecord(ctx, filePath, fileSize)
						imageURL := cfg.SupabaseStorageBaseURL + filePath

						result = GeneratedAngleImage{
							Angle:      currentAngle,
							AngleLabel: GetAngleLabel(currentAngle),
							ImageURL:   imageURL,
							AttachID:   attachID,
							Success:    true,
						}

						// 성공한 이미지 크레딧 누적 (마지막에 한번에 차감)
						mu.Lock()
						totalCreditsUsed += cfg.ImagePerPrice
						mu.Unlock()
					}
				}
			}

			// 결과 저장
			mu.Lock()
			generatedImages = append(generatedImages, result)
			if result.Success && result.AttachID > 0 {
				generatedAttachIDs = append(generatedAttachIDs, result.AttachID)
			}
			mu.Unlock()

			// 진행 상황 업데이트
			mu.Lock()
			completedCount := len(generatedImages)
			currentAttachIDs := make([]int, 0, len(generatedAttachIDs))
			for _, id := range generatedAttachIDs {
				if intID, ok := id.(int); ok {
					currentAttachIDs = append(currentAttachIDs, intID)
				}
			}
			mu.Unlock()

			if err := dbClient.UpdateJobProgress(ctx, job.JobID, completedCount, currentAttachIDs); err != nil {
				log.Printf("⚠️ [Multiview] Failed to update progress: %v", err)
			}

		}(angle)
	}

	wg.Wait()

	// Phase 7: 크레딧 한번에 차감 (동시성 문제 방지)
	if totalCreditsUsed > 0 {
		log.Printf("💰 [Multiview] Deducting total credits: %d", totalCreditsUsed)
		productionID := ""
		if job.ProductionID != nil {
			productionID = *job.ProductionID
		}
		if err := service.DeductCredits(ctx, userID, totalCreditsUsed, productionID); err != nil {
			log.Printf("⚠️ [Multiview] Failed to deduct credits: %v", err)
		} else {
			log.Printf("✅ [Multiview] Successfully deducted %d credits", totalCreditsUsed)
		}
	}

	// Phase 8: Job 완료 상태 업데이트
	successCount := 0
	for _, img := range generatedImages {
		if img.Success {
			successCount++
		}
	}

	log.Printf("✅ [Multiview] Generation completed - JobID: %s, Success: %d/%d, CreditsUsed: %d",
		job.JobID, successCount, len(angles), totalCreditsUsed)

	// Job 상태 업데이트
	if successCount > 0 {
		if err := dbClient.UpdateJobCompleted(ctx, job.JobID, generatedAttachIDs); err != nil {
			log.Printf("⚠️ [Multiview] Failed to update job completed: %v", err)
		}

		// Phase 9: Production에 생성된 이미지 추가 (Archive에 표시되도록)
		if job.ProductionID != nil && *job.ProductionID != "" {
			attachIDsInt := make([]int, 0, len(generatedAttachIDs))
			for _, id := range generatedAttachIDs {
				if intID, ok := id.(int); ok {
					attachIDsInt = append(attachIDsInt, intID)
				}
			}
			if len(attachIDsInt) > 0 {
				if err := dbClient.UpdateProductionAttachIds(ctx, *job.ProductionID, attachIDsInt); err != nil {
					log.Printf("⚠️ [Multiview] Failed to update production attach_ids: %v", err)
				} else {
					log.Printf("✅ [Multiview] Production attach_ids updated: %d images", len(attachIDsInt))
				}
			}

			// Production 상태를 completed로 업데이트
			if err := dbClient.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusCompleted); err != nil {
				log.Printf("⚠️ [Multiview] Failed to update production status: %v", err)
			}
		}
	} else {
		updateJobFailed(job.JobID, "All image generations failed")
	}
}

// GenerateSingleAngle - 단일 각도 이미지 생성
func (s *Service) GenerateSingleAngle(ctx context.Context, sourceImage, refImage []byte, angle int, aspectRatio, category, originalPrompt string, hasReference, rotateBackground bool) ([]byte, error) {
	cfg := config.GetConfig()

	// Gemini API 호출 준비
	var parts []*genai.Part

	// 원본 이미지 추가
	parts = append(parts, &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: "image/png",
			Data:     sourceImage,
		},
	})

	// 레퍼런스 이미지가 있으면 추가
	if hasReference && len(refImage) > 0 {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     refImage,
			},
		})
	}

	// 프롬프트 생성
	prompt := BuildMultiviewPrompt(0, angle, category, originalPrompt, hasReference, rotateBackground)
	parts = append(parts, genai.NewPartFromText(prompt))

	// Content 생성
	content := &genai.Content{
		Parts: parts,
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
			Temperature: floatPtr(0.5),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	// 응답에서 이미지 추출
	if len(result.Candidates) > 0 {
		for _, candidate := range result.Candidates {
			if candidate.Content == nil {
				continue
			}
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil && len(part.InlineData.Data) > 0 {
					return part.InlineData.Data, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no image in API response")
}

// updateJobFailed - Job 실패 상태 업데이트
func updateJobFailed(jobID, errorMsg string) {
	dbClient := database.NewClient()
	if dbClient == nil {
		log.Printf("❌ [Multiview] Failed to create DB client for error update")
		return
	}

	ctx := context.Background()
	if err := dbClient.UpdateJobFailed(ctx, jobID, errorMsg); err != nil {
		log.Printf("❌ [Multiview] Failed to update job failed status: %v", err)
	}
}

// Redis 관련 헬퍼 (redis 패키지 import를 위해)
var _ = redis.Nil