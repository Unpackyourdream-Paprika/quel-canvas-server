package cartoon

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"quel-canvas-server/modules/common/cancel"
	"quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/fallback"
	"quel-canvas-server/modules/common/model"
)

// StartWorker - Redis Queue Worker 시작
func StartWorker() {
	log.Println("🔄 Redis Queue Worker starting...")

	cfg := config.GetConfig()

	// 테스트
	// Service 초기화
	service := NewService()
	if service == nil {
		log.Fatal("❌ Failed to initialize Service")
		return
	}

	// 1단계: Redis 연결
	rdb := connectRedis(cfg)
	if rdb == nil {
		log.Fatal("❌ Failed to connect to Redis")
		return
	}
	log.Println("✅ Redis connected successfully")

	// 2단계: Queue 감시 시작
	log.Println("👀 Watching queue: jobs:queue")

	ctx := context.Background()

	// 무한 루프로 Queue 감시
	for {
		// 3단계: Job 받기 (BRPOP - Blocking Right Pop)
		result, err := rdb.BRPop(ctx, 0, "jobs:queue").Result()
		if err != nil {
			log.Printf("❌ Redis BRPOP error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// result[0]은 "jobs:queue", result[1]이 실제 job_id
		jobId := result[1]
		log.Printf("🎯 Received new job: %s", jobId)

		// 4단계: Job 처리 (goroutine으로 비동기)
		go processJob(ctx, service, jobId)
	}
}

// processJob - Job 처리 함수
func processJob(ctx context.Context, service *Service, jobID string) {
	log.Printf("🚀 Processing job: %s", jobID)

	// 4단계: Supabase에서 Job 데이터 조회
	job, err := service.FetchJobFromSupabase(jobID)
	if err != nil {
		log.Printf("❌ Failed to fetch job %s: %v", jobID, err)
		return
	}

	// Job 데이터 로그 출력 (디버깅)
	log.Printf("📦 Job Data:")
	log.Printf("   JobID: %s", job.JobID)
	log.Printf("   JobType: %s", job.JobType)
	log.Printf("   Status: %s", job.JobStatus)
	log.Printf("   TotalImages: %d", job.TotalImages)

	// ProductionID 값 출력 (포인터 처리)
	if job.ProductionID != nil {
		log.Printf("   ProductionID: %s", *job.ProductionID)
	} else {
		log.Printf("   ProductionID: null")
	}

	log.Printf("   JobInputData: %+v", job.JobInputData)

	// Job Type 확인 및 분기 처리
	log.Printf("🔍 Processing job_type: %s", job.JobType)

	switch job.JobType {
	case "single_batch":
		log.Printf("📌 Single Batch Mode - Processing %d images in one batch", job.TotalImages)
		processSingleBatch(ctx, service, job)
	case "pipeline_stage":
		log.Printf("📌 Pipeline Stage Mode - Processing stage %v", job.StageIndex)
		processPipelineStage(ctx, service, job)

	case "simple_general":
		log.Printf("📌 Simple General Mode - Processing %d images with multiple input images", job.TotalImages)
		processSimpleGeneral(ctx, service, job)

	case "simple_portrait":
		log.Printf("📌 Simple Portrait Mode - Processing %d images with merged images", job.TotalImages)
		processSimplePortrait(ctx, service, job)

	default:
		log.Printf("⚠️  Unknown job_type: %s, using default single_batch mode", job.JobType)
		processSingleBatch(ctx, service, job)
	}
}

// processSingleBatch - Single Batch 모드 처리 (다중 조합 지원)
func processSingleBatch(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🚀 Starting Single Batch processing for job: %s", job.JobID)

	// Phase 1: Input Data 추출
	individualImageAttachIds, ok := job.JobInputData["individualImageAttachIds"].([]interface{})
	if !ok || len(individualImageAttachIds) == 0 {
		log.Printf("❌ Failed to get individualImageAttachIds or empty array - using placeholders")
		individualImageAttachIds = []interface{}{}
	}

	basePrompt := fallback.SafeString(job.JobInputData["basePrompt"], "best quality, masterpiece")
	// Combinations 배열 추출
	combinations := fallback.NormalizeCombinations(job.JobInputData["combinations"], fallback.DefaultQuantity(job.TotalImages), "front", "full")

	// aspect-ratio 추출 (기본값: "16:9")
	aspectRatio := fallback.SafeAspectRatio(job.JobInputData["aspect-ratio"])

	userID := fallback.SafeString(job.JobInputData["userId"], "")

	// org_id가 없으면 유저의 조직 조회
	if job.OrgID == nil && userID != "" {
		orgID, err := service.GetUserOrganization(ctx, userID)
		if err == nil && orgID != "" {
			job.OrgID = &orgID
			log.Printf("🏢 Found organization for user %s: %s", userID, orgID)
		}
	}

	log.Printf("📦 Input Data: IndividualImages=%d, BasePrompt=%s, Combinations=%d, UserID=%s",
		len(individualImageAttachIds), basePrompt, len(combinations), userID)

	// Phase 2: Status 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusProcessing); err != nil {
		log.Printf("❌ Failed to update job status: %v", err)
		return
	}

	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusProcessing); err != nil {
			log.Printf("⚠️  Failed to update production status: %v", err)
		}
	}

	// Phase 3: 이미지 다운로드 및 카테고리별 분류
	categories := &ImageCategories{
		Character: [][]byte{},
		Prop:      [][]byte{},
	}

	// Cartoon 프론트 타입: none, character, face, prop, background

	for i, attachObj := range individualImageAttachIds {
		attachMap, ok := attachObj.(map[string]interface{})
		if !ok {
			log.Printf("⚠️  Invalid attach object at index %d", i)
			continue
		}

		attachIDFloat, ok := attachMap["attachId"].(float64)
		if !ok {
			log.Printf("⚠️  Failed to get attachId at index %d", i)
			continue
		}

		attachID := int(attachIDFloat)
		attachType, _ := attachMap["type"].(string)

		log.Printf("📥 [Cartoon] Downloading image %d/%d: AttachID=%d, Type=%s",
			i+1, len(individualImageAttachIds), attachID, attachType)

		imageData, err := service.DownloadImageFromStorage(attachID)
		if err != nil {
			log.Printf("❌ Failed to download image %d: %v", attachID, err)
			continue
		}

		// type에 따라 카테고리별로 분류 (Cartoon 전용)
		switch attachType {
		case "character":
			if len(categories.Character) < MaxModels {
				categories.Character = append(categories.Character, imageData)
				log.Printf("✅ [Cartoon] Character image added (%d/%d)", len(categories.Character), MaxModels)
			} else {
				log.Printf("⚠️ [Cartoon] Maximum characters reached (%d), skipping", MaxModels)
			}
		case "face":
			if len(categories.Character) < MaxModels {
				categories.Character = append(categories.Character, imageData)
				log.Printf("✅ [Cartoon] Face reference added (%d/%d)", len(categories.Character), MaxModels)
			} else {
				log.Printf("⚠️ [Cartoon] Maximum models reached (%d), skipping face", MaxModels)
			}
		case "background":
			categories.Background = imageData
			log.Printf("✅ [Cartoon] Background image added")
		case "prop":
			categories.Prop = append(categories.Prop, imageData)
			log.Printf("✅ [Cartoon] Prop image added")
		case "none":
			// none은 Prop(Accessories)로 처리
			categories.Prop = append(categories.Prop, imageData)
			log.Printf("✅ [Cartoon] None type → Prop image added")
		default:
			// 알 수 없는 타입은 Prop으로 처리
			categories.Prop = append(categories.Prop, imageData)
			log.Printf("⚠️  [Cartoon] Unknown type: %s → Prop image added", attachType)
		}
	}

	normalizeCartoonCategories(categories, &basePrompt)

	log.Printf("✅ Images classified - Character:%d, Prop:%d, BG:%v",
		len(categories.Character), len(categories.Prop), categories.Background != nil)

	// Phase 4: Combinations 병렬 처리
	var wg sync.WaitGroup
	var progressMutex sync.Mutex
	generatedAttachIds := []int{}
	completedCount := 0

	log.Printf("Starting parallel processing for %d combinations (max 2 concurrent)", len(combinations))

	// Semaphore: 최대 2개 조합만 동시 처리
	semaphore := make(chan struct{}, 2)

	for comboIdx, combo := range combinations {
		wg.Add(1)

		go func(idx int, combo map[string]interface{}) {
			defer wg.Done()

			// Semaphore 획득 (최대 2개까지만)
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // 완료 시 반환

			angle := fallback.SafeString(combo["angle"], "front")
			shot := fallback.SafeString(combo["shot"], "full")
			fx := fallback.SafeString(combo["fx"], "none")
			quantity := fallback.SafeInt(combo["quantity"], 1)

			log.Printf("Combination %d/%d: angle=%s, shot=%s, fx=%s, quantity=%d (parallel)",
				idx+1, len(combinations), angle, shot, fx, quantity)

			// 앵글/샷/FX에 대한 상세 설명 추가
			angleDesc := GetAngleDescription(angle)
			shotDesc := GetShotDescription(shot)
			fxDesc := GetFXDescription(fx)

			// 앵글을 프롬프트 맨 앞과 맨 뒤에 2번 강조
			var enhancedPrompt string
			if fxDesc != "" {
				enhancedPrompt = fmt.Sprintf("⚠️ MANDATORY CAMERA ANGLE: %s\n\n[FRAMING]: %s\n[VISUAL FX]: %s\n\n%s\n\n⚠️ REMINDER - CAMERA ANGLE: %s", angleDesc, shotDesc, fxDesc, basePrompt, angleDesc)
			} else {
				enhancedPrompt = fmt.Sprintf("⚠️ MANDATORY CAMERA ANGLE: %s\n\n[FRAMING]: %s\n\n%s\n\n⚠️ REMINDER - CAMERA ANGLE: %s", angleDesc, shotDesc, basePrompt, angleDesc)
			}

			log.Printf("📝 Combination %d Enhanced Prompt: %s", idx+1, enhancedPrompt[:minInt(100, len(enhancedPrompt))])

			// 해당 조합의 quantity만큼 생성
			for i := 0; i < quantity; i++ {
				// 🛑 취소 체크 - 새 이미지 생성 전에 확인
				if service.IsJobCancelled(job.JobID) {
					log.Printf("🛑 Combination %d: Job %s cancelled, stopping generation", idx+1, job.JobID)
					service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
					if job.ProductionID != nil {
						service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
					}
					return
				}

				log.Printf("🎨 Combination %d: Generating image %d/%d for [%s + %s]...",
					idx+1, i+1, quantity, angle, shot)

				// Gemini API 호출 (카테고리별 이미지 전달, aspect-ratio 포함)
				generatedBase64, err := service.GenerateImageWithGeminiMultiple(ctx, categories, enhancedPrompt, aspectRatio)
				if err != nil {
					log.Printf("❌ Combination %d: Gemini API failed for image %d: %v", idx+1, i+1, err)
					// 403 PERMISSION_DENIED 또는 429 RESOURCE_EXHAUSTED 에러 체크
					if (strings.Contains(err.Error(), "403") && strings.Contains(err.Error(), "PERMISSION_DENIED")) ||
						(strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
						log.Printf("🚨 API Error detected (403 PERMISSION_DENIED or 429 RESOURCE_EXHAUSTED) - Stopping job.")
						if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed); err != nil {
							log.Printf("❌ Failed to update job status to error: %v", err)
						}
						if job.ProductionID != nil {
							if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusFailed); err != nil {
								log.Printf("❌ Failed to update production status to error: %v", err)
							}
						}
						return
					}
					continue
				}

				// 🛑 Gemini 응답 후 취소 체크 - 취소됐으면 저장/차감 안 함
				if service.IsJobCancelled(job.JobID) {
					log.Printf("🛑 Combination %d: Job %s cancelled after generation, discarding image %d", idx+1, job.JobID, i+1)
					service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
					if job.ProductionID != nil {
						service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
					}
					return
				}

				// Base64 → []byte 변환
				generatedImageData, err := base64DecodeString(generatedBase64)
				if err != nil {
					log.Printf("❌ Combination %d: Failed to decode image %d: %v", idx+1, i+1, err)
					continue
				}

				// Storage 업로드
				filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
				if err != nil {
					log.Printf("❌ Combination %d: Failed to upload image %d: %v", idx+1, i+1, err)
					continue
				}

				// Attach 레코드 생성
				attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
				if err != nil {
					log.Printf("❌ Combination %d: Failed to create attach record %d: %v", idx+1, i+1, err)
					continue
				}

				// 크레딧 차감 (조직/개인 구분)
				if job.ProductionID != nil && userID != "" {
					go func(attachID int, prodID string, orgID *string) {
						if err := service.DeductCredits(context.Background(), userID, orgID, prodID, []int{attachID}, "gemini-banana"); err != nil {
							log.Printf("⚠️  Combination %d: Failed to deduct credits for attach %d: %v", idx+1, attachID, err)
						}
					}(attachID, *job.ProductionID, job.OrgID)
				}

				// 성공 카운트 및 ID 수집 (thread-safe)
				progressMutex.Lock()
				generatedAttachIds = append(generatedAttachIds, attachID)
				completedCount++
				currentProgress := completedCount
				currentAttachIds := make([]int, len(generatedAttachIds))
				copy(currentAttachIds, generatedAttachIds)
				progressMutex.Unlock()

				log.Printf("✅ Combination %d: Image %d/%d completed for [%s + %s]: AttachID=%d",
					idx+1, i+1, quantity, angle, shot, attachID)

				// 진행 상황 업데이트
				if err := service.UpdateJobProgress(ctx, job.JobID, currentProgress, currentAttachIds); err != nil {
					log.Printf("⚠️  Failed to update progress: %v", err)
				}
			}

			log.Printf("✅ Combination %d/%d completed: %d images generated",
				idx+1, len(combinations), quantity)
		}(comboIdx, combo)
	}

	// 모든 Combination 완료 대기
	log.Printf("⏳ Waiting for all %d combinations to complete...", len(combinations))
	wg.Wait()
	log.Printf("✅ All combinations completed in parallel")

	// Phase 5: 최종 완료 처리
	// 🛑 취소된 Job은 user_cancelled 상태 유지 (completed로 덮어쓰지 않음)
	if service.IsJobCancelled(job.JobID) {
		log.Printf("🛑 Job %s was cancelled, keeping user_cancelled status", job.JobID)
		// attach_ids만 업데이트 (이미 생성된 이미지들)
		if job.ProductionID != nil && len(generatedAttachIds) > 0 {
			if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, generatedAttachIds); err != nil {
				log.Printf("Failed to update production attach_ids: %v", err)
			}
		}
		log.Printf("Single Batch processing completed for job: %s (cancelled with %d images)", job.JobID, len(generatedAttachIds))
		return
	}

	finalStatus := model.StatusCompleted
	if completedCount == 0 {
		log.Printf("⚠️ No images generated; marking job as completed with fallbacks")
	}

	log.Printf("🏁 Job %s finished: %d/%d images completed", job.JobID, completedCount, job.TotalImages)

	// Job 상태 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, finalStatus); err != nil {
		log.Printf("❌ Failed to update final job status: %v", err)
	}

	// Production 업데이트
	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, finalStatus); err != nil {
			log.Printf("⚠️  Failed to update final production status: %v", err)
		}

		if len(generatedAttachIds) > 0 {
			if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, generatedAttachIds); err != nil {
				log.Printf("⚠️  Failed to update production attach_ids: %v", err)
			}
		}
	}

	log.Printf("✅ Single Batch processing completed for job: %s", job.JobID)
}

func normalizeCartoonCategories(categories *ImageCategories, prompt *string) {
	if categories == nil {
		return
	}

	// 이미지가 전혀 없는 경우 (텍스트만으로 생성) - placeholder 사용 안 함
	hasAnyImage := len(categories.Character) > 0 || len(categories.Prop) > 0 || categories.Background != nil
	if !hasAnyImage {
		log.Printf("🔧 [Cartoon] No images provided - will generate with text prompt only")
		if prompt != nil {
			*prompt = strings.TrimSpace(*prompt + "\nGenerate a completely new image based on the text description only.")
		}
		return
	}

	if len(categories.Character) == 0 {
		switch {
		case len(categories.Prop) > 0:
			categories.Character = append(categories.Character, categories.Prop[0])
			log.Printf("🔧 Using prop image as character placeholder")
		case categories.Background != nil:
			categories.Character = append(categories.Character, categories.Background)
			log.Printf("🔧 Using background image as character placeholder")
		default:
			// 🔧 더 이상 1x1 placeholder 사용 안 함
			log.Printf("🔧 [Cartoon] No character image available - will use text-only generation")
		}

		if prompt != nil {
			*prompt = strings.TrimSpace(*prompt + "\nIf no character is provided, keep the cartoon style consistent without a person.")
		}
	}

	// Character가 있을 때만 Prop 채우기
	if len(categories.Prop) == 0 && len(categories.Character) > 0 {
		categories.Prop = append(categories.Prop, categories.Character[0])
		log.Printf("🔧 No props provided; reusing character reference for stability")
	}
}

// minInt - Helper function for minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getIntFromInterface - Helper function to extract int from interface{} (supports both float64 and string)
func getIntFromInterface(value interface{}, defaultValue int) int {
	if f, ok := value.(float64); ok {
		return int(f)
	}
	if s, ok := value.(string); ok {
		var result int
		if _, err := fmt.Sscanf(s, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

// processPipelineStage - Pipeline Stage 모드 처리 (여러 stage 순차 실행)
func processPipelineStage(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🚀 Starting Pipeline Stage processing for job: %s", job.JobID)

	// Phase 1: stages 배열 추출
	defaultPrompt := fallback.SafeString(job.JobInputData["basePrompt"], "best quality, masterpiece")
	stages, ok := job.JobInputData["stages"].([]interface{})
	if !ok || len(stages) == 0 {
		log.Printf("❌ Failed to get stages array from job_input_data - creating default stage")
		stages = []interface{}{
			map[string]interface{}{
				"stage_index": 0,
				"prompt":      defaultPrompt,
				"quantity":    fallback.DefaultQuantity(job.TotalImages),
			},
		}
	}

	userID := fallback.SafeString(job.JobInputData["userId"], "")
	log.Printf("📦 Pipeline has %d stages, UserID=%s", len(stages), userID)

	// Phase 2: Job 상태 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusProcessing); err != nil {
		log.Printf("❌ Failed to update job status: %v", err)
		return
	}

	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusProcessing); err != nil {
			log.Printf("⚠️  Failed to update production status: %v", err)
		}
	}

	// Phase 3: 모든 Stage 병렬 처리 (최종 배열은 순서 보장)
	results := make([]cancel.StageResult, len(stages))
	var wg sync.WaitGroup
	var progressMutex sync.Mutex
	totalCompleted := 0
	tempAttachIds := []int{} // 실시간 진행용 임시 배열 (순서 무관)

	for stageIdx, stageData := range stages {
		wg.Add(1)

		go func(idx int, data interface{}) {
			defer wg.Done()

			stage, ok := data.(map[string]interface{})
			if !ok {
				log.Printf("❌ Invalid stage data at index %d - using empty stage", idx)
				stage = map[string]interface{}{}
			}

			// Stage 데이터 추출
			stageIndex := getIntFromInterface(stage["stage_index"], idx)
			prompt := fallback.SafeString(stage["prompt"], defaultPrompt)
			quantity := getIntFromInterface(stage["quantity"], fallback.DefaultQuantity(job.TotalImages))

			// aspect-ratio 추출 (기본값: "16:9")
			aspectRatio := fallback.SafeAspectRatio(stage["aspect-ratio"])

			log.Printf("🎬 Stage %d/%d: Processing %d images with aspect-ratio %s (parallel)", stageIndex+1, len(stages), quantity, aspectRatio)

			// individualImageAttachIds 또는 mergedImageAttachId 지원
			stageCategories := &ImageCategories{
				Character: [][]byte{},
				Prop:      [][]byte{},
			}

			if individualIds, ok := stage["individualImageAttachIds"].([]interface{}); ok && len(individualIds) > 0 {
				// 새 방식: individualImageAttachIds로 카테고리별 분류
				log.Printf("🔍 Stage %d: Using individualImageAttachIds (%d images)", stageIndex, len(individualIds))

				clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
				accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true, "prop": true}

				for i, attachObj := range individualIds {
					attachMap, ok := attachObj.(map[string]interface{})
					if !ok {
						log.Printf("⚠️  Stage %d: Invalid attach object at index %d", stageIndex, i)
						continue
					}

					attachIDFloat, ok := attachMap["attachId"].(float64)
					if !ok {
						log.Printf("⚠️  Stage %d: Failed to get attachId at index %d", stageIndex, i)
						continue
					}

					attachID := int(attachIDFloat)
					attachType, _ := attachMap["type"].(string)

					imageData, err := service.DownloadImageFromStorage(attachID)
					if err != nil {
						log.Printf("❌ Stage %d: Failed to download image %d: %v", stageIndex, attachID, err)
						continue
					}

					// type에 따라 카테고리별로 분류
					switch attachType {
					case "model", "character", "face":
						if len(stageCategories.Character) < MaxModels {
							stageCategories.Character = append(stageCategories.Character, imageData)
							log.Printf("✅ Stage %d: Character/Face image added (%d/%d) [type: %s]", stageIndex, len(stageCategories.Character), MaxModels, attachType)
						} else {
							log.Printf("⚠️ Stage %d: Maximum characters reached (%d), skipping", stageIndex, MaxModels)
						}
					case "bg", "background":
						stageCategories.Background = imageData
						log.Printf("✅ Stage %d: Background image added", stageIndex)
					default:
						if clothingTypes[attachType] {
							stageCategories.Prop = append(stageCategories.Prop, imageData)
							log.Printf("✅ Stage %d: Clothing image added (type: %s)", stageIndex, attachType)
						} else if accessoryTypes[attachType] {
							stageCategories.Prop = append(stageCategories.Prop, imageData)
							log.Printf("✅ Stage %d: Accessory image added (type: %s)", stageIndex, attachType)
						} else {
							log.Printf("⚠️  Stage %d: Unknown type: %s, skipping", stageIndex, attachType)
						}
					}
				}

				log.Printf("✅ Stage %d: Images classified - Character:%d, Prop:%d, BG:%v",
					stageIndex, len(stageCategories.Character), len(stageCategories.Prop), stageCategories.Background != nil)

			} else if mergedID, ok := stage["mergedImageAttachId"].(float64); ok {
				// 레거시 방식: mergedImageAttachId
				log.Printf("⚠️  Stage %d: Using legacy mergedImageAttachId (deprecated)", stageIndex)
				mergedImageAttachID := int(mergedID)

				imageData, err := service.DownloadImageFromStorage(mergedImageAttachID)
				if err != nil {
					log.Printf("❌ Stage %d: Failed to download merged image: %v - using placeholder", stageIndex, err)
					imageData = fallback.PlaceholderBytes()
				}

				// 레거시 이미지를 Prop 카테고리로 처리
				stageCategories = &ImageCategories{
					Character: [][]byte{},
					Prop:      [][]byte{imageData},
				}
			} else {
				log.Printf("❌ Stage %d: No individualImageAttachIds or mergedImageAttachId found - using placeholder", stageIndex)
				stageCategories.Prop = append(stageCategories.Prop, fallback.PlaceholderBytes())
			}

			normalizeCartoonCategories(stageCategories, &prompt)

			// Cartoon 모듈 - 캐릭터(Model) 검증
			if len(stageCategories.Character) == 0 {
				log.Printf("❌ CRITICAL: Stage %d - Cartoon module requires CHARACTER (Model) image", stageIndex)
				log.Printf("❌ GLOBAL node must include character appearance reference")
				log.Printf("❌ Cannot generate webtoon/cartoon without character - Stage skipped")
				return
			}

			// Stage별 이미지 생성 루프
			stageGeneratedIds := []int{}

			for i := 0; i < quantity; i++ {
				// 🛑 취소 체크 - 새 이미지 생성 전에 확인
				if service.IsJobCancelled(job.JobID) {
					log.Printf("🛑 Stage %d: Job %s cancelled, stopping generation", stageIndex, job.JobID)
					results[stageIndex] = cancel.StageResult{
						StageIndex: stageIndex,
						AttachIDs:  stageGeneratedIds,
						Success:    len(stageGeneratedIds),
					}
					service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
					if job.ProductionID != nil {
						service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
					}
					return
				}

				log.Printf("🎨 Stage %d: Generating image %d/%d...", stageIndex, i+1, quantity)

				// Gemini API 호출 (카테고리별 이미지 전달, aspect-ratio 포함)
				generatedBase64, err := service.GenerateImageWithGeminiMultiple(ctx, stageCategories, prompt, aspectRatio)
				if err != nil {
					log.Printf("❌ Stage %d: Gemini API failed for image %d: %v", stageIndex, i+1, err)
					// 403 PERMISSION_DENIED 또는 429 RESOURCE_EXHAUSTED 에러 체크
					if (strings.Contains(err.Error(), "403") && strings.Contains(err.Error(), "PERMISSION_DENIED")) ||
						(strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
						log.Printf("🚨 API Error detected (403 PERMISSION_DENIED or 429 RESOURCE_EXHAUSTED) - Stopping job.")
						if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed); err != nil {
							log.Printf("❌ Failed to update job status to error: %v", err)
						}
						if job.ProductionID != nil {
							if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusFailed); err != nil {
								log.Printf("❌ Failed to update production status to error: %v", err)
							}
						}
						return
					}
					continue
				}

				// 🛑 Gemini 응답 후 취소 체크 - 취소됐으면 저장/차감 안 함
				if service.IsJobCancelled(job.JobID) {
					log.Printf("🛑 Stage %d: Job %s cancelled after generation, discarding image %d", stageIndex, job.JobID, i+1)
					results[stageIndex] = cancel.StageResult{
						StageIndex: stageIndex,
						AttachIDs:  stageGeneratedIds,
						Success:    len(stageGeneratedIds),
					}
					service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
					if job.ProductionID != nil {
						service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
					}
					return
				}

				// Base64 → []byte 변환
				generatedImageData, err := base64DecodeString(generatedBase64)
				if err != nil {
					log.Printf("❌ Stage %d: Failed to decode image %d: %v", stageIndex, i+1, err)
					continue
				}

				// Storage 업로드
				filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
				if err != nil {
					log.Printf("❌ Stage %d: Failed to upload image %d: %v", stageIndex, i+1, err)
					continue
				}

				// Attach 레코드 생성
				attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
				if err != nil {
					log.Printf("❌ Stage %d: Failed to create attach record %d: %v", stageIndex, i+1, err)
					continue
				}

				// 크레딧 차감 (조직/개인 구분)
				if job.ProductionID != nil && userID != "" {
					go func(attachID int, prodID string, orgID *string) {
						if err := service.DeductCredits(context.Background(), userID, orgID, prodID, []int{attachID}, "gemini-banana"); err != nil {
							log.Printf("⚠️  Stage %d: Failed to deduct credits for attach %d: %v", stageIndex, attachID, err)
						}
					}(attachID, *job.ProductionID, job.OrgID)
				}

				// Stage별 배열에 추가
				stageGeneratedIds = append(stageGeneratedIds, attachID)

				log.Printf("✅ Stage %d: Image %d/%d completed: AttachID=%d", stageIndex, i+1, quantity, attachID)

				// 전체 진행 상황 카운트 (thread-safe)
				progressMutex.Lock()
				totalCompleted++
				currentProgress := totalCompleted
				progressMutex.Unlock()

				log.Printf("📊 Overall progress: %d/%d images completed", currentProgress, job.TotalImages)

				// 실시간 DB 업데이트 (순서 무관, 빠른 업데이트)
				progressMutex.Lock()
				tempAttachIds = append(tempAttachIds, attachID)
				currentTempIds := make([]int, len(tempAttachIds))
				copy(currentTempIds, tempAttachIds)
				progressMutex.Unlock()

				// DB 업데이트 (순서는 나중에 최종 정렬)
				if err := service.UpdateJobProgress(ctx, job.JobID, currentProgress, currentTempIds); err != nil {
					log.Printf("⚠️  Failed to update progress: %v", err)
				}
			}

			// Stage 결과 저장 (stage_index 기반으로 올바른 위치에 저장)
			results[stageIndex] = cancel.StageResult{
				StageIndex: stageIndex,
				AttachIDs:  stageGeneratedIds,
				Success:    len(stageGeneratedIds),
			}

			log.Printf("🎬 Stage %d completed: %d/%d images generated", stageIndex, len(stageGeneratedIds), quantity)
		}(stageIdx, stageData)
	}

	// 모든 Stage 완료 대기
	log.Printf("⏳ Waiting for all stages to complete...")
	wg.Wait()
	log.Printf("✅ All stages completed in parallel")

	// ========== 재시도 로직 시작 ==========
	log.Printf("🔍 Checking missing images for each stage...")

	// Step 1: 각 Stage별 부족 갯수 확인
	for stageIdx, stageData := range stages {
		stage := stageData.(map[string]interface{})
		expectedQuantity := getIntFromInterface(stage["quantity"], 1)
		actualQuantity := len(results[stageIdx].AttachIDs)
		missing := expectedQuantity - actualQuantity

		if missing > 0 {
			log.Printf("⚠️  Stage %d: Missing %d images (expected: %d, got: %d)",
				stageIdx, missing, expectedQuantity, actualQuantity)
		} else {
			log.Printf("✅ Stage %d: Complete (expected: %d, got: %d)",
				stageIdx, expectedQuantity, actualQuantity)
		}
	}

	// Step 2: 부족한 Stage만 재시도
	for stageIdx, stageData := range stages {
		// 🛑 재시도 전에 취소 체크
		if service.IsJobCancelled(job.JobID) {
			log.Printf("🛑 Job %s cancelled, skipping retry phase", job.JobID)
			break
		}

		stage := stageData.(map[string]interface{})
		expectedQuantity := getIntFromInterface(stage["quantity"], 1)
		actualQuantity := len(results[stageIdx].AttachIDs)
		missing := expectedQuantity - actualQuantity

		if missing <= 0 {
			continue
		}

		log.Printf("🔄 Stage %d: Starting retry for %d missing images...", stageIdx, missing)

		// Stage 데이터 재추출
		prompt := fallback.SafeString(stage["prompt"], defaultPrompt)
		aspectRatio := fallback.SafeAspectRatio(stage["aspect-ratio"])

		// individualImageAttachIds 또는 mergedImageAttachId 지원
		retryCategories := &ImageCategories{
			Character: [][]byte{},
			Prop:      [][]byte{},
		}

		if individualIds, ok := stage["individualImageAttachIds"].([]interface{}); ok && len(individualIds) > 0 {
			// 새 방식: individualImageAttachIds로 카테고리별 분류
			clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
			accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true, "prop": true}

			for _, attachObj := range individualIds {
				attachMap := attachObj.(map[string]interface{})
				attachID := int(attachMap["attachId"].(float64))
				attachType, _ := attachMap["type"].(string)

				imageData := fallback.PlaceholderBytes()
				if downloaded, err := service.DownloadImageFromStorage(attachID); err == nil {
					imageData = downloaded
				} else {
					log.Printf("❌ Stage %d retry: Failed to download image %d: %v", stageIdx, attachID, err)
				}

				switch attachType {
				case "model", "character", "face":
					if len(retryCategories.Character) < MaxModels {
						retryCategories.Character = append(retryCategories.Character, imageData)
					}
				case "bg", "background":
					retryCategories.Background = imageData
				default:
					if clothingTypes[attachType] {
						retryCategories.Prop = append(retryCategories.Prop, imageData)
					} else if accessoryTypes[attachType] {
						retryCategories.Prop = append(retryCategories.Prop, imageData)
					}
				}
			}
		} else if mergedID, ok := stage["mergedImageAttachId"].(float64); ok {
			// 레거시 방식
			mergedImageAttachID := int(mergedID)
			imageData, err := service.DownloadImageFromStorage(mergedImageAttachID)
			if err != nil {
				log.Printf("❌ Stage %d: Failed to download input image for retry: %v - using placeholder", stageIdx, err)
				imageData = fallback.PlaceholderBytes()
			}
			retryCategories = &ImageCategories{
				Character: [][]byte{},
				Prop:      [][]byte{imageData},
			}
		} else {
			log.Printf("❌ Stage %d: No image data for retry - using placeholder", stageIdx)
			retryCategories.Prop = append(retryCategories.Prop, fallback.PlaceholderBytes())
		}

		normalizeCartoonCategories(retryCategories, &prompt)

		// 재시도 루프
		retrySuccess := 0
		for i := 0; i < missing; i++ {
			// 🛑 재시도 중 취소 체크
			if service.IsJobCancelled(job.JobID) {
				log.Printf("🛑 Stage %d: Job %s cancelled during retry", stageIdx, job.JobID)
				service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
				if job.ProductionID != nil {
					service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
				}
				return
			}

			log.Printf("🔄 Stage %d: Retry generating image %d/%d...", stageIdx, i+1, missing)

			// Gemini API 호출 (카테고리별 이미지 전달)
			generatedBase64, err := service.GenerateImageWithGeminiMultiple(ctx, retryCategories, prompt, aspectRatio)
			if err != nil {
				log.Printf("❌ Stage %d: Retry %d failed: %v", stageIdx, i+1, err)
				// 403 PERMISSION_DENIED 또는 429 RESOURCE_EXHAUSTED 에러 체크
				if (strings.Contains(err.Error(), "403") && strings.Contains(err.Error(), "PERMISSION_DENIED")) ||
					(strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
					log.Printf("🚨 API Error detected (403 PERMISSION_DENIED or 429 RESOURCE_EXHAUSTED) - Stopping retry.")
					if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed); err != nil {
						log.Printf("❌ Failed to update job status to error: %v", err)
					}
					if job.ProductionID != nil {
						if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusFailed); err != nil {
							log.Printf("❌ Failed to update production status to error: %v", err)
						}
					}
					return
				}
				continue
			}

			// Base64 → []byte 변환
			generatedImageData, err := base64DecodeString(generatedBase64)
			if err != nil {
				log.Printf("❌ Stage %d: Failed to decode retry image %d: %v", stageIdx, i+1, err)
				continue
			}

			// Storage 업로드
			filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
			if err != nil {
				log.Printf("❌ Stage %d: Failed to upload retry image %d: %v", stageIdx, i+1, err)
				continue
			}

			// Attach 레코드 생성
			attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
			if err != nil {
				log.Printf("❌ Stage %d: Failed to create attach record for retry %d: %v", stageIdx, i+1, err)
				continue
			}

			// 크레딧 차감 (조직/개인 구분)
			if job.ProductionID != nil && userID != "" {
				go func(aID int, prodID string, orgID *string) {
					if err := service.DeductCredits(context.Background(), userID, orgID, prodID, []int{aID}, "gemini-banana"); err != nil {
						log.Printf("⚠️  Stage %d: Failed to deduct credits for retry attach %d: %v", stageIdx, aID, err)
					}
				}(attachID, *job.ProductionID, job.OrgID)
			}

			// results에 추가
			results[stageIdx].AttachIDs = append(results[stageIdx].AttachIDs, attachID)
			retrySuccess++

			// 전체 진행 상황 업데이트
			progressMutex.Lock()
			totalCompleted++
			currentProgress := totalCompleted
			tempAttachIds = append(tempAttachIds, attachID)
			currentTempIds := make([]int, len(tempAttachIds))
			copy(currentTempIds, tempAttachIds)
			progressMutex.Unlock()

			log.Printf("✅ Stage %d: Retry image %d/%d completed: AttachID=%d", stageIdx, i+1, missing, attachID)
			log.Printf("📊 Overall progress: %d/%d images completed", currentProgress, job.TotalImages)

			// DB 업데이트
			if err := service.UpdateJobProgress(ctx, job.JobID, currentProgress, currentTempIds); err != nil {
				log.Printf("⚠️  Failed to update progress: %v", err)
			}
		}

		log.Printf("✅ Stage %d retry completed: %d/%d images recovered", stageIdx, retrySuccess, missing)
		log.Printf("📊 Stage %d final count: %d/%d images", stageIdx, len(results[stageIdx].AttachIDs), expectedQuantity)
	}

	log.Printf("🔍 All retry attempts completed")
	// ========== 재시도 로직 끝 ==========

	// 배열 합치기 전 각 Stage 결과 출력
	log.Printf("🔍 ===== Stage Results Before Merge =====")
	for i := 0; i < len(results); i++ {
		if results[i].AttachIDs != nil {
			log.Printf("📦 Stage %d: %v (total: %d)", i, results[i].AttachIDs, len(results[i].AttachIDs))
		} else {
			log.Printf("📦 Stage %d: [] (empty)", i)
		}
	}
	log.Printf("🔍 ========================================")

	// Stage 순서대로 AttachID 합치기 (stage_index 기준 정렬하여 순서 보장)
	allGeneratedAttachIds := []int{}
	for i := 0; i < len(results); i++ {
		if results[i].AttachIDs != nil {
			allGeneratedAttachIds = append(allGeneratedAttachIds, results[i].AttachIDs...)
			log.Printf("📎 Stage %d: Added %d attach IDs in order", i, len(results[i].AttachIDs))
		}
	}

	log.Printf("🎯 Final merged array: %v (total: %d)", allGeneratedAttachIds, len(allGeneratedAttachIds))

	// 최종 Job 진행 상황 업데이트
	if len(allGeneratedAttachIds) > 0 {
		if err := service.UpdateJobProgress(ctx, job.JobID, len(allGeneratedAttachIds), allGeneratedAttachIds); err != nil {
			log.Printf("⚠️  Failed to update final progress: %v", err)
		}
	}

	// Phase 4: 최종 완료 처리
	// 🛑 취소된 Job은 user_cancelled 상태 유지 (completed로 덮어쓰지 않음)
	if service.IsJobCancelled(job.JobID) {
		log.Printf("🛑 Job %s was cancelled, keeping user_cancelled status", job.JobID)
		// attach_ids만 업데이트 (이미 생성된 이미지들)
		if job.ProductionID != nil && len(allGeneratedAttachIds) > 0 {
			if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, allGeneratedAttachIds); err != nil {
				log.Printf("Failed to update production attach_ids: %v", err)
			}
		}
		log.Printf("Pipeline Stage processing completed for job: %s (cancelled with %d images)", job.JobID, len(allGeneratedAttachIds))
		return
	}

	finalStatus := model.StatusCompleted
	if len(allGeneratedAttachIds) == 0 {
		log.Printf("⚠️ No images generated in pipeline; marking job as completed with fallbacks")
	}

	log.Printf("🏁 Pipeline Job %s finished: %d/%d images completed", job.JobID, len(allGeneratedAttachIds), job.TotalImages)

	// Job 상태 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, finalStatus); err != nil {
		log.Printf("❌ Failed to update final job status: %v", err)
	}

	// Production 업데이트
	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, finalStatus); err != nil {
			log.Printf("⚠️  Failed to update final production status: %v", err)
		}

		if len(allGeneratedAttachIds) > 0 {
			if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, allGeneratedAttachIds); err != nil {
				log.Printf("⚠️  Failed to update production attach_ids: %v", err)
			}
		}
	}

	log.Printf("✅ Pipeline Stage processing completed for job: %s", job.JobID)
}

// base64DecodeString - Base64 문자열을 바이트 배열로 디코딩
func base64DecodeString(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// connectRedis - Redis 연결 설정
func connectRedis(config *config.Config) *redis.Client {
	log.Printf("🔌 Connecting to Redis: %s", config.GetRedisAddr())

	// TLS 설정 (InsecureSkipVerify 추가)
	var tlsConfig *tls.Config
	if config.RedisUseTLS {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, // Render.com Redis용
		}
	}

	// Redis 클라이언트 생성
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.GetRedisAddr(),
		Username:     config.RedisUsername,
		Password:     config.RedisPassword,
		TLSConfig:    tlsConfig,
		DB:           0,                // 기본 DB
		DialTimeout:  10 * time.Second, // 타임아웃 늘림
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	// 연결 테스트
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("🔍 Testing Redis connection...")
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("❌ Redis ping failed: %v", err)
		return nil
	}

	return rdb
}

// processSimpleGeneral - Simple General 모드 처리 (여러 입력 이미지 기반)
func processSimpleGeneral(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🚀 Starting Simple General processing for job: %s", job.JobID)

	// Phase 1: Input Data 추출
	uploadedAttachIds, ok := job.JobInputData["uploadedAttachIds"].([]interface{})
	if !ok || len(uploadedAttachIds) == 0 {
		log.Printf("❌ Failed to get uploadedAttachIds or empty array - using placeholder")
		uploadedAttachIds = []interface{}{}
	}

	prompt := fallback.SafeString(job.JobInputData["prompt"], "best quality, masterpiece")

	// aspect-ratio 추출 (기본값: "16:9")
	aspectRatio := fallback.SafeAspectRatio(job.JobInputData["aspect-ratio"])

	quantity := job.TotalImages
	if quantity <= 0 {
		quantity = 1
	}
	userID := fallback.SafeString(job.JobInputData["userId"], "")

	log.Printf("📦 Input Data: UploadedImages=%d, Prompt=%s, Quantity=%d, AspectRatio=%s, UserID=%s",
		len(uploadedAttachIds), prompt, quantity, aspectRatio, userID)

	// Phase 2: Status 업데이트 - Job & Production → "processing"
	if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusProcessing); err != nil {
		log.Printf("❌ Failed to update job status: %v", err)
		return
	}

	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusProcessing); err != nil {
			log.Printf("⚠️  Failed to update production status: %v", err)
		}
	}

	// Phase 3: 모든 입력 이미지 다운로드 및 Base64 변환
	var base64Images []string

	for i, attachObj := range uploadedAttachIds {
		attachMap, ok := attachObj.(map[string]interface{})
		if !ok {
			log.Printf("⚠️  Invalid attach object at index %d", i)
			continue
		}

		attachIDFloat, ok := attachMap["attachId"].(float64)
		if !ok {
			log.Printf("⚠️  Invalid attachId at index %d", i)
			continue
		}
		attachID := int(attachIDFloat)

		attachType, _ := attachMap["type"].(string)
		log.Printf("📥 Downloading input image %d/%d: AttachID=%d, Type=%s",
			i+1, len(uploadedAttachIds), attachID, attachType)

		imageData, err := service.DownloadImageFromStorage(attachID)
		if err != nil {
			log.Printf("❌ Failed to download image %d: %v", attachID, err)
			continue
		}

		base64Image := service.ConvertImageToBase64(imageData)
		base64Images = append(base64Images, base64Image)
		log.Printf("✅ Input image %d prepared (Base64 length: %d)", i+1, len(base64Image))
	}

	if len(base64Images) == 0 {
		log.Printf("❌ No input images downloaded successfully - using placeholder")
		base64Images = []string{fallback.PlaceholderBase64()}
	}

	log.Printf("✅ All %d input images prepared", len(base64Images))

	// Phase 4: 이미지 생성 루프
	generatedAttachIds := []int{}
	completedCount := 0

	for i := 0; i < quantity; i++ {
		// 🛑 취소 체크
		if service.IsJobCancelled(job.JobID) {
			log.Printf("🛑 Job %s cancelled, stopping generation", job.JobID)
			service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
			if job.ProductionID != nil {
				service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
			}
			return
		}

		log.Printf("🎨 Generating image %d/%d...", i+1, quantity)

		// 4.1: Gemini API 호출 (단일 이미지 전달, aspect-ratio 포함)
		// ⚠️  simple_general은 레거시 모드 - 첫 번째 이미지만 사용
		if len(base64Images) == 0 {
			log.Printf("❌ No base64 images available")
			continue
		}
		generatedBase64, err := service.GenerateImageWithGemini(ctx, base64Images[0], prompt, aspectRatio)
		if err != nil {
			log.Printf("❌ Gemini API failed for image %d: %v", i+1, err)
			// 403 PERMISSION_DENIED 또는 429 RESOURCE_EXHAUSTED 에러 체크
			if (strings.Contains(err.Error(), "403") && strings.Contains(err.Error(), "PERMISSION_DENIED")) ||
				(strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
				log.Printf("🚨 API Error detected (403 PERMISSION_DENIED or 429 RESOURCE_EXHAUSTED) - Stopping job.")
				if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed); err != nil {
					log.Printf("❌ Failed to update job status to error: %v", err)
				}
				if job.ProductionID != nil {
					if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusFailed); err != nil {
						log.Printf("❌ Failed to update production status to error: %v", err)
					}
				}
				return
			}
			continue
		}

		// 4.2: Base64 → []byte 변환
		generatedImageData, err := base64DecodeString(generatedBase64)
		if err != nil {
			log.Printf("❌ Failed to decode generated image %d: %v", i+1, err)
			continue
		}

		// 4.3: Storage 업로드
		filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
		if err != nil {
			log.Printf("❌ Failed to upload image %d: %v", i+1, err)
			continue
		}

		// 4.4: Attach 레코드 생성
		attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
		if err != nil {
			log.Printf("❌ Failed to create attach record %d: %v", i+1, err)
			continue
		}

		// 4.5: 크레딧 차감 (조직/개인 구분)
		if job.ProductionID != nil && userID != "" {
			go func(attachID int, prodID string, orgID *string) {
				if err := service.DeductCredits(context.Background(), userID, orgID, prodID, []int{attachID}, "gemini-banana"); err != nil {
					log.Printf("⚠️  Failed to deduct credits for attach %d: %v", attachID, err)
				}
			}(attachID, *job.ProductionID, job.OrgID)
		}

		// 4.6: 성공 카운트 및 ID 수집
		generatedAttachIds = append(generatedAttachIds, attachID)
		completedCount++

		log.Printf("✅ Image %d/%d completed: AttachID=%d", i+1, quantity, attachID)

		// 4.7: 진행 상황 업데이트
		if err := service.UpdateJobProgress(ctx, job.JobID, completedCount, generatedAttachIds); err != nil {
			log.Printf("⚠️  Failed to update progress: %v", err)
		}
	}

	// Phase 5: 최종 완료 처리
	finalStatus := model.StatusCompleted
	if completedCount == 0 {
		log.Printf("⚠️ No images generated; marking job as completed with fallbacks")
	}

	log.Printf("🏁 Job %s finished: %d/%d images completed", job.JobID, completedCount, quantity)

	// Job 상태 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, finalStatus); err != nil {
		log.Printf("❌ Failed to update final job status: %v", err)
	}

	// Production 업데이트 (상태 + attach_ids 배열)
	if job.ProductionID != nil {
		// Production 상태 업데이트
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, finalStatus); err != nil {
			log.Printf("⚠️  Failed to update final production status: %v", err)
		}

		// Production attach_ids 배열에 생성된 이미지 ID 추가
		if len(generatedAttachIds) > 0 {
			if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, generatedAttachIds); err != nil {
				log.Printf("⚠️  Failed to update production attach_ids: %v", err)
			}
		}
	}

	log.Printf("✅ Simple General processing completed for job: %s", job.JobID)
}

// processSimplePortrait - Simple Portrait 모드 처리 (mergedImages 기반)
func processSimplePortrait(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🚀 Starting Simple Portrait processing for job: %s", job.JobID)

	// Phase 1: Input Data 추출
	mergedImages, ok := job.JobInputData["mergedImages"].([]interface{})
	if !ok || len(mergedImages) == 0 {
		log.Printf("❌ Failed to get mergedImages or empty array - using placeholder entry")
		mergedImages = []interface{}{map[string]interface{}{}}
	}

	// aspect-ratio 추출 (기본값: "16:9")
	aspectRatio := fallback.SafeAspectRatio(job.JobInputData["aspect-ratio"])

	userID := fallback.SafeString(job.JobInputData["userId"], "")
	defaultPrompt := fallback.SafeString(job.JobInputData["basePrompt"], "best quality, masterpiece")

	log.Printf("📦 Input Data: MergedImages=%d, AspectRatio=%s, UserID=%s", len(mergedImages), aspectRatio, userID)

	// Phase 2: Status 업데이트 - Job & Production → "processing"
	if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusProcessing); err != nil {
		log.Printf("❌ Failed to update job status: %v", err)
		return
	}

	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusProcessing); err != nil {
			log.Printf("⚠️  Failed to update production status: %v", err)
		}
	}

	// Phase 3: 이미지 생성 루프 (각 mergedImage마다 처리)
	generatedAttachIds := []int{}
	completedCount := 0

	for i, mergedImageObj := range mergedImages {
		mergedImageMap, ok := mergedImageObj.(map[string]interface{})
		if !ok {
			log.Printf("⚠️  Invalid mergedImage object at index %d - using placeholder", i)
			mergedImageMap = map[string]interface{}{}
		}

		// mergedAttachId 추출
		mergedAttachID := getIntFromInterface(mergedImageMap["mergedAttachId"], 0)

		// wrappingPrompt 추출
		wrappingPrompt := fallback.SafeString(mergedImageMap["wrappingPrompt"], defaultPrompt)

		photoIndex := getIntFromInterface(mergedImageMap["photoIndex"], i)

		log.Printf("🎨 Generating image %d/%d (PhotoIndex=%d, MergedAttachID=%d)...",
			i+1, len(mergedImages), int(photoIndex), mergedAttachID)

		// 3.1: 입력 이미지 다운로드
		imageData := fallback.PlaceholderBytes()
		if mergedAttachID > 0 {
			if downloaded, err := service.DownloadImageFromStorage(mergedAttachID); err == nil {
				imageData = downloaded
			} else {
				log.Printf("❌ Failed to download merged image %d: %v - using placeholder", mergedAttachID, err)
			}
		} else {
			log.Printf("⚠️ No mergedAttachId provided for index %d - using placeholder", i)
		}

		base64Image := service.ConvertImageToBase64(imageData)
		log.Printf("✅ Merged image prepared (Base64 length: %d)", len(base64Image))

		// 3.2: Gemini API 호출 (단일 이미지 + wrappingPrompt, aspect-ratio 포함)
		generatedBase64, err := service.GenerateImageWithGemini(ctx, base64Image, wrappingPrompt, aspectRatio)
		if err != nil {
			log.Printf("❌ Gemini API failed for image %d: %v", i+1, err)
			// 403 PERMISSION_DENIED 또는 429 RESOURCE_EXHAUSTED 에러 체크
			if (strings.Contains(err.Error(), "403") && strings.Contains(err.Error(), "PERMISSION_DENIED")) ||
				(strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
				log.Printf("🚨 API Error detected (403 PERMISSION_DENIED or 429 RESOURCE_EXHAUSTED) - Stopping job.")
				if err := service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed); err != nil {
					log.Printf("❌ Failed to update job status to error: %v", err)
				}
				if job.ProductionID != nil {
					if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusFailed); err != nil {
						log.Printf("❌ Failed to update production status to error: %v", err)
					}
				}
				return
			}
			continue
		}

		// 3.3: Base64 → []byte 변환
		generatedImageData, err := base64DecodeString(generatedBase64)
		if err != nil {
			log.Printf("❌ Failed to decode generated image %d: %v", i+1, err)
			continue
		}

		// 3.4: Storage 업로드
		filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
		if err != nil {
			log.Printf("❌ Failed to upload image %d: %v", i+1, err)
			continue
		}

		// 3.5: Attach 레코드 생성
		attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
		if err != nil {
			log.Printf("❌ Failed to create attach record %d: %v", i+1, err)
			continue
		}

		// 3.6: 크레딧 차감 (조직/개인 구분)
		if job.ProductionID != nil && userID != "" {
			go func(attachID int, prodID string, orgID *string) {
				if err := service.DeductCredits(context.Background(), userID, orgID, prodID, []int{attachID}, "gemini-banana"); err != nil {
					log.Printf("⚠️  Failed to deduct credits for attach %d: %v", attachID, err)
				}
			}(attachID, *job.ProductionID, job.OrgID)
		}

		// 3.7: 성공 카운트 및 ID 수집
		generatedAttachIds = append(generatedAttachIds, attachID)
		completedCount++

		log.Printf("✅ Image %d/%d completed: AttachID=%d", i+1, len(mergedImages), attachID)

		// 3.8: 진행 상황 업데이트
		if err := service.UpdateJobProgress(ctx, job.JobID, completedCount, generatedAttachIds); err != nil {
			log.Printf("⚠️  Failed to update progress: %v", err)
		}
	}

	// Phase 4: 최종 완료 처리
	finalStatus := model.StatusCompleted
	if completedCount == 0 {
		log.Printf("⚠️ No images generated; marking job as completed with fallbacks")
	}

	log.Printf("🏁 Job %s finished: %d/%d images completed", job.JobID, completedCount, len(mergedImages))

	// Job 상태 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, finalStatus); err != nil {
		log.Printf("❌ Failed to update final job status: %v", err)
	}

	// Production 업데이트 (상태 + attach_ids 배열)
	if job.ProductionID != nil {
		// Production 상태 업데이트
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, finalStatus); err != nil {
			log.Printf("⚠️  Failed to update final production status: %v", err)
		}

		// Production attach_ids 배열에 생성된 이미지 ID 추가
		if len(generatedAttachIds) > 0 {
			if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, generatedAttachIds); err != nil {
				log.Printf("⚠️  Failed to update production attach_ids: %v", err)
			}
		}
	}

	log.Printf("✅ Simple Portrait processing completed for job: %s", job.JobID)
}
