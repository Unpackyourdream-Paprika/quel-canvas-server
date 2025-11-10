package beauty

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"quel-canvas-server/modules/common/config"
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
		log.Printf("❌ Failed to get individualImageAttachIds or empty array")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	basePrompt, ok := job.JobInputData["basePrompt"].(string)
	if !ok {
		log.Printf("❌ Failed to get basePrompt")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	// Combinations 배열 추출
	combinationsRaw, ok := job.JobInputData["combinations"].([]interface{})
	if !ok {
		log.Printf("❌ Failed to get combinations array")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	// aspect-ratio 추출 (기본값: "16:9")
	aspectRatio := "16:9"
	if ar, ok := job.JobInputData["aspect-ratio"].(string); ok && ar != "" {
		aspectRatio = ar
	}

	userID, _ := job.JobInputData["userId"].(string)

	log.Printf("📦 Input Data: IndividualImages=%d, BasePrompt=%s, Combinations=%d, UserID=%s",
		len(individualImageAttachIds), basePrompt, len(combinationsRaw), userID)

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
		Clothing:    [][]byte{},
		Accessories: [][]byte{},
	}

	clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
	accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true}

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

		log.Printf("📥 Downloading image %d/%d: AttachID=%d, Type=%s",
			i+1, len(individualImageAttachIds), attachID, attachType)

		imageData, err := service.DownloadImageFromStorage(attachID)
		if err != nil {
			log.Printf("❌ Failed to download image %d: %v", attachID, err)
			continue
		}

		// type에 따라 카테고리별로 분류
		switch attachType {
		case "model":
			categories.Model = imageData
			log.Printf("✅ Model image added")
		case "background", "bg":
			categories.Background = imageData
			log.Printf("✅ Background image added")
		default:
			if clothingTypes[attachType] {
				categories.Clothing = append(categories.Clothing, imageData)
				log.Printf("✅ Clothing image added (type: %s)", attachType)
			} else if accessoryTypes[attachType] {
				categories.Accessories = append(categories.Accessories, imageData)
				log.Printf("✅ Accessory image added (type: %s)", attachType)
			} else if attachType != "none" {
				log.Printf("⚠️  Unknown type: %s, skipping", attachType)
			}
		}
	}

	// 최소한 의류 이미지는 있어야 함
	if len(categories.Clothing) == 0 && categories.Model == nil {
		log.Printf("❌ No clothing or model images found")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	log.Printf("✅ Images classified - Model:%v, Clothing:%d, Accessories:%d, BG:%v",
		categories.Model != nil, len(categories.Clothing), len(categories.Accessories), categories.Background != nil)

	// Phase 4: Combinations 병렬 처리
	var wg sync.WaitGroup
	var progressMutex sync.Mutex
	generatedAttachIds := []int{}
	completedCount := 0

	// Camera Angle 매핑 (시네마틱 톤 + Beauty 전용 앵글)
	cameraAngleTextMap := map[string]string{
		// Fashion 기본 앵글
		"front":   "Cinematic front-facing angle, direct eye contact with camera, film photography composition",
		"side":    "Cinematic side profile angle, 90-degree perspective, film photography composition",
		"profile": "Professional cinematic portrait, formal front-facing composition with confident posture, clean elegant background, polished film aesthetic",
		"back":    "Cinematic rear angle, back view composition, film photography aesthetic",

		// Beauty 전용 앵글
		"3/4": "Three-quarter angle, face turned slightly showing both frontal and side features, classic beauty portrait composition, elegant and flattering perspective",
	}

	// Shot Type 매핑 (시네마틱 톤 + Beauty 전용 샷)
	shotTypeTextMap := map[string]string{
		// Fashion 기본 샷
		"tight":  "Cinematic tight shot, film camera close-up framing from shoulders up, fill frame naturally with subject's face and upper body, intimate cinematic composition",
		"middle": "Cinematic medium shot, film camera framing from waist up, balanced composition showing upper body and outfit details, editorial fashion film style",
		"full":   "Cinematic full body shot, film camera capturing head to toe, complete outfit visible with environmental context, wide fashion film composition",

		// Beauty 전용 샷 타입
		"closeup": "Beauty closeup shot, tight framing on face highlighting makeup and skin texture, professional beauty photography, emphasizes facial features and cosmetics",
		"upper":   "Upper body shot, framing from chest up, beauty editorial composition, shows face, neck, and shoulders with makeup and styling details",
		"detail":  "Extreme detail shot, macro focus on specific feature (eyes, lips, skin), high-end beauty photography, showcases makeup artistry and product details",
	}

	log.Printf("🚀 Starting parallel processing for %d combinations (max 2 concurrent)", len(combinationsRaw))

	// Semaphore: 최대 2개 조합만 동시 처리
	semaphore := make(chan struct{}, 2)

	for comboIdx, comboRaw := range combinationsRaw {
		wg.Add(1)

		go func(idx int, data interface{}) {
			defer wg.Done()

			// Semaphore 획득 (최대 2개까지만)
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // 완료 시 반환

			combo := data.(map[string]interface{})
			angle := combo["angle"].(string)
			shot := combo["shot"].(string)
			quantity := int(combo["quantity"].(float64))

			log.Printf("🎯 Combination %d/%d: angle=%s, shot=%s, quantity=%d (parallel)",
				idx+1, len(combinationsRaw), angle, shot, quantity)

			// 조합별 프롬프트 생성
			cameraAngleText := cameraAngleTextMap[angle]
			if cameraAngleText == "" {
				cameraAngleText = "Front view" // 기본값
			}

			shotTypeText := shotTypeTextMap[shot]
			if shotTypeText == "" {
				shotTypeText = "full body shot" // 기본값
			}

			enhancedPrompt := cameraAngleText + ", " + shotTypeText + ". " + basePrompt +
				". Create a single unified photorealistic cinematic composition where the model wears all clothing and accessories together in one complete outfit. " +
				"Film photography aesthetic with natural storytelling composition."

			log.Printf("📝 Combination %d Enhanced Prompt: %s", idx+1, enhancedPrompt[:minInt(100, len(enhancedPrompt))])

			// 해당 조합의 quantity만큼 생성
			for i := 0; i < quantity; i++ {
				log.Printf("🎨 Combination %d: Generating image %d/%d for [%s + %s]...",
					idx+1, i+1, quantity, angle, shot)

				// Gemini API 호출 (카테고리별 이미지 전달, aspect-ratio 포함)
				generatedBase64, err := service.GenerateImageWithGeminiMultiple(ctx, categories, enhancedPrompt, aspectRatio)
				if err != nil {
					log.Printf("❌ Combination %d: Gemini API failed for image %d: %v", idx+1, i+1, err)
					continue
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

				// 크레딧 차감
				if job.ProductionID != nil && userID != "" {
					go func(attachID int, prodID string) {
						if err := service.DeductCredits(context.Background(), userID, prodID, []int{attachID}); err != nil {
							log.Printf("⚠️  Combination %d: Failed to deduct credits for attach %d: %v", idx+1, attachID, err)
						}
					}(attachID, *job.ProductionID)
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
				idx+1, len(combinationsRaw), quantity)
		}(comboIdx, comboRaw)
	}

	// 모든 Combination 완료 대기
	log.Printf("⏳ Waiting for all %d combinations to complete...", len(combinationsRaw))
	wg.Wait()
	log.Printf("✅ All combinations completed in parallel")

	// Phase 5: 최종 완료 처리
	finalStatus := model.StatusCompleted
	if completedCount == 0 {
		finalStatus = model.StatusFailed
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

// minInt - Helper function for minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// processPipelineStage - Pipeline Stage 모드 처리 (여러 stage 순차 실행)
func processPipelineStage(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🚀 Starting Pipeline Stage processing for job: %s", job.JobID)

	// Phase 1: stages 배열 추출
	stages, ok := job.JobInputData["stages"].([]interface{})
	if !ok {
		log.Printf("❌ Failed to get stages array from job_input_data")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	userID, _ := job.JobInputData["userId"].(string)
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
	type StageResult struct {
		StageIndex int
		AttachIDs  []int
		Success    int
	}

	results := make([]StageResult, len(stages))
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
				log.Printf("❌ Invalid stage data at index %d", idx)
				return
			}

			// Stage 데이터 추출
			stageIndex := int(stage["stage_index"].(float64))
			prompt := stage["prompt"].(string)
			quantity := int(stage["quantity"].(float64))

			// aspect-ratio 추출 (기본값: "16:9")
			aspectRatio := "16:9"
			if ar, ok := stage["aspect-ratio"].(string); ok && ar != "" {
				aspectRatio = ar
			}

			log.Printf("🎬 Stage %d/%d: Processing %d images with aspect-ratio %s (parallel)", stageIndex+1, len(stages), quantity, aspectRatio)

			// individualImageAttachIds 또는 mergedImageAttachId 지원
			var stageCategories *ImageCategories

			if individualIds, ok := stage["individualImageAttachIds"].([]interface{}); ok && len(individualIds) > 0 {
				// 새 방식: individualImageAttachIds로 카테고리별 분류
				log.Printf("🔍 Stage %d: Using individualImageAttachIds (%d images)", stageIndex, len(individualIds))

				stageCategories = &ImageCategories{
					Clothing:    [][]byte{},
					Accessories: [][]byte{},
				}

				clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
				accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true}

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
					case "model":
						stageCategories.Model = imageData
						log.Printf("✅ Stage %d: Model image added", stageIndex)
					case "bg":
						stageCategories.Background = imageData
						log.Printf("✅ Stage %d: Background image added", stageIndex)
					default:
						if clothingTypes[attachType] {
							stageCategories.Clothing = append(stageCategories.Clothing, imageData)
							log.Printf("✅ Stage %d: Clothing image added (type: %s)", stageIndex, attachType)
						} else if accessoryTypes[attachType] {
							stageCategories.Accessories = append(stageCategories.Accessories, imageData)
							log.Printf("✅ Stage %d: Accessory image added (type: %s)", stageIndex, attachType)
						} else {
							log.Printf("⚠️  Stage %d: Unknown type: %s, skipping", stageIndex, attachType)
						}
					}
				}


				log.Printf("✅ Stage %d: Images classified - Model:%v, Clothing:%d, Accessories:%d, BG:%v",
					stageIndex, stageCategories.Model != nil, len(stageCategories.Clothing),
					len(stageCategories.Accessories), stageCategories.Background != nil)

			} else if mergedID, ok := stage["mergedImageAttachId"].(float64); ok {
				// 레거시 방식: mergedImageAttachId
				log.Printf("⚠️  Stage %d: Using legacy mergedImageAttachId (deprecated)", stageIndex)
				mergedImageAttachID := int(mergedID)

				imageData, err := service.DownloadImageFromStorage(mergedImageAttachID)
				if err != nil {
					log.Printf("❌ Stage %d: Failed to download merged image: %v", stageIndex, err)
					return
				}

				// 레거시 이미지를 Clothing 카테고리로 처리
				stageCategories = &ImageCategories{
					Clothing:    [][]byte{imageData},
					Accessories: [][]byte{},
				}
			} else {
				log.Printf("❌ Stage %d: No individualImageAttachIds or mergedImageAttachId found", stageIndex)
				return
			}

			// Stage별 이미지 생성 루프
			stageGeneratedIds := []int{}

			for i := 0; i < quantity; i++ {
				log.Printf("🎨 Stage %d: Generating image %d/%d...", stageIndex, i+1, quantity)

				// Gemini API 호출 (카테고리별 이미지 전달, aspect-ratio 포함)
				generatedBase64, err := service.GenerateImageWithGeminiMultiple(ctx, stageCategories, prompt, aspectRatio)
				if err != nil {
					log.Printf("❌ Stage %d: Gemini API failed for image %d: %v", stageIndex, i+1, err)
					continue
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

				// 크레딧 차감 (Attach 성공 직후)
				if job.ProductionID != nil && userID != "" {
					go func(attachID int, prodID string) {
						if err := service.DeductCredits(context.Background(), userID, prodID, []int{attachID}); err != nil {
							log.Printf("⚠️  Stage %d: Failed to deduct credits for attach %d: %v", stageIndex, attachID, err)
						}
					}(attachID, *job.ProductionID)
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
			results[stageIndex] = StageResult{
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
		expectedQuantity := int(stage["quantity"].(float64))
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
		stage := stageData.(map[string]interface{})
		expectedQuantity := int(stage["quantity"].(float64))
		actualQuantity := len(results[stageIdx].AttachIDs)
		missing := expectedQuantity - actualQuantity

		if missing <= 0 {
			continue
		}

		log.Printf("🔄 Stage %d: Starting retry for %d missing images...", stageIdx, missing)

		// Stage 데이터 재추출
		prompt := stage["prompt"].(string)
		aspectRatio := "16:9"
		if ar, ok := stage["aspect-ratio"].(string); ok && ar != "" {
			aspectRatio = ar
		}

		// individualImageAttachIds 또는 mergedImageAttachId 지원
		var retryCategories *ImageCategories

		if individualIds, ok := stage["individualImageAttachIds"].([]interface{}); ok && len(individualIds) > 0 {
			// 새 방식: individualImageAttachIds로 카테고리별 분류
			retryCategories = &ImageCategories{
				Clothing:    [][]byte{},
				Accessories: [][]byte{},
			}

			clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
			accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true, "acce": true}

			for _, attachObj := range individualIds {
				attachMap := attachObj.(map[string]interface{})
				attachID := int(attachMap["attachId"].(float64))
				attachType, _ := attachMap["type"].(string)

				imageData, err := service.DownloadImageFromStorage(attachID)
				if err != nil {
					log.Printf("❌ Stage %d retry: Failed to download image %d", stageIdx, attachID)
					continue
				}

				switch attachType {
				case "model":
					retryCategories.Model = imageData
				case "bg":
					retryCategories.Background = imageData
				default:
					if clothingTypes[attachType] {
						retryCategories.Clothing = append(retryCategories.Clothing, imageData)
					} else if accessoryTypes[attachType] {
						retryCategories.Accessories = append(retryCategories.Accessories, imageData)
					}
				}
			}
		} else if mergedID, ok := stage["mergedImageAttachId"].(float64); ok {
			// 레거시 방식
			mergedImageAttachID := int(mergedID)
			imageData, err := service.DownloadImageFromStorage(mergedImageAttachID)
			if err != nil {
				log.Printf("❌ Stage %d: Failed to download input image for retry: %v", stageIdx, err)
				continue
			}
			retryCategories = &ImageCategories{
				Clothing:    [][]byte{imageData},
				Accessories: [][]byte{},
			}
		} else {
			log.Printf("❌ Stage %d: No image data for retry", stageIdx)
			continue
		}

		// 재시도 루프
		retrySuccess := 0
		for i := 0; i < missing; i++ {
			log.Printf("🔄 Stage %d: Retry generating image %d/%d...", stageIdx, i+1, missing)

			// Gemini API 호출 (카테고리별 이미지 전달)
			generatedBase64, err := service.GenerateImageWithGeminiMultiple(ctx, retryCategories, prompt, aspectRatio)
			if err != nil {
				log.Printf("❌ Stage %d: Retry %d failed: %v", stageIdx, i+1, err)
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

			// 크레딧 차감
			if job.ProductionID != nil && userID != "" {
				go func(aID int, prodID string) {
					if err := service.DeductCredits(context.Background(), userID, prodID, []int{aID}); err != nil {
						log.Printf("⚠️  Stage %d: Failed to deduct credits for retry attach %d: %v", stageIdx, aID, err)
					}
				}(attachID, *job.ProductionID)
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
	finalStatus := model.StatusCompleted
	if len(allGeneratedAttachIds) == 0 {
		finalStatus = model.StatusFailed
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
		DB:           0,              // 기본 DB
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
		log.Printf("❌ Failed to get uploadedAttachIds or empty array")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	prompt, ok := job.JobInputData["prompt"].(string)
	if !ok {
		log.Printf("❌ Failed to get prompt")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	// aspect-ratio 추출 (기본값: "16:9")
	aspectRatio := "16:9"
	if ar, ok := job.JobInputData["aspect-ratio"].(string); ok && ar != "" {
		aspectRatio = ar
	}

	quantity := job.TotalImages
	userID, _ := job.JobInputData["userId"].(string)

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
		log.Printf("❌ No input images downloaded successfully")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	log.Printf("✅ All %d input images prepared", len(base64Images))

	// Phase 4: 이미지 생성 루프
	generatedAttachIds := []int{}
	completedCount := 0

	for i := 0; i < quantity; i++ {
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

		// 4.5: 크레딧 차감 (Attach 성공 직후 즉시 처리)
		if job.ProductionID != nil && userID != "" {
			go func(attachID int, prodID string) {
				if err := service.DeductCredits(context.Background(), userID, prodID, []int{attachID}); err != nil {
					log.Printf("⚠️  Failed to deduct credits for attach %d: %v", attachID, err)
				}
			}(attachID, *job.ProductionID)
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
		finalStatus = model.StatusFailed
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
		log.Printf("❌ Failed to get mergedImages or empty array")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		return
	}

	// aspect-ratio 추출 (기본값: "16:9")
	aspectRatio := "16:9"
	if ar, ok := job.JobInputData["aspect-ratio"].(string); ok && ar != "" {
		aspectRatio = ar
	}

	userID, _ := job.JobInputData["userId"].(string)

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
			log.Printf("⚠️  Invalid mergedImage object at index %d", i)
			continue
		}

		// mergedAttachId 추출
		mergedAttachIDFloat, ok := mergedImageMap["mergedAttachId"].(float64)
		if !ok {
			log.Printf("⚠️  Invalid mergedAttachId at index %d", i)
			continue
		}
		mergedAttachID := int(mergedAttachIDFloat)

		// wrappingPrompt 추출
		wrappingPrompt, ok := mergedImageMap["wrappingPrompt"].(string)
		if !ok {
			log.Printf("⚠️  Invalid wrappingPrompt at index %d", i)
			continue
		}

		photoIndex, _ := mergedImageMap["photoIndex"].(float64)

		log.Printf("🎨 Generating image %d/%d (PhotoIndex=%d, MergedAttachID=%d)...",
			i+1, len(mergedImages), int(photoIndex), mergedAttachID)

		// 3.1: 입력 이미지 다운로드
		imageData, err := service.DownloadImageFromStorage(mergedAttachID)
		if err != nil {
			log.Printf("❌ Failed to download merged image %d: %v", mergedAttachID, err)
			continue
		}

		base64Image := service.ConvertImageToBase64(imageData)
		log.Printf("✅ Merged image prepared (Base64 length: %d)", len(base64Image))

		// 3.2: Gemini API 호출 (단일 이미지 + wrappingPrompt, aspect-ratio 포함)
		generatedBase64, err := service.GenerateImageWithGemini(ctx, base64Image, wrappingPrompt, aspectRatio)
		if err != nil {
			log.Printf("❌ Gemini API failed for image %d: %v", i+1, err)
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

		// 3.6: 크레딧 차감 (Attach 성공 직후 즉시 처리)
		if job.ProductionID != nil && userID != "" {
			go func(attachID int, prodID string) {
				if err := service.DeductCredits(context.Background(), userID, prodID, []int{attachID}); err != nil {
					log.Printf("⚠️  Failed to deduct credits for attach %d: %v", attachID, err)
				}
			}(attachID, *job.ProductionID)
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
		finalStatus = model.StatusFailed
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
