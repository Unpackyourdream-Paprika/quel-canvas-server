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
	"quel-canvas-server/modules/submodule/nanobanana"
	"quel-canvas-server/modules/submodule/seedream"
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

// processLandingSimpleGeneral - Landing 이미지 생성 처리 (model_id 기반 라우팅)
func processLandingSimpleGeneral(ctx context.Context, service *Service, job *model.ProductionJob) {
	log.Printf("🚀 [Landing] Starting Simple General processing for job: %s", job.JobID)

	// Input Data 추출
	prompt := fallback.SafeString(job.JobInputData["prompt"], "")
	aspectRatio := fallback.SafeAspectRatio(job.JobInputData["aspect-ratio"])
	quantity := job.TotalImages
	if quantity <= 0 || quantity > 4 {
		quantity = 4
	}
	userID := fallback.SafeString(job.JobInputData["userId"], "")

	// 모델 관련 파라미터 추출
	modelID := fallback.SafeString(job.JobInputData["modelId"], "")
	templatePrompt := fallback.SafeString(job.JobInputData["templatePrompt"], "")
	negativePrompt := fallback.SafeString(job.JobInputData["negativePrompt"], "")
	modelSteps := fallback.SafeInt(job.JobInputData["modelSteps"], 4)
	modelCfgScale := fallback.SafeFloat(job.JobInputData["modelCfgScale"], 1.0)

	log.Printf("📦 [Landing] Input: Prompt=%s, AspectRatio=%s, Quantity=%d, UserID=%s",
		truncateString(prompt, 50), aspectRatio, quantity, userID)
	log.Printf("📦 [Landing] Model: ID=%s, Steps=%d, CFG=%.1f", modelID, modelSteps, modelCfgScale)

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

	// 이미지가 있고 프롬프트가 비어있으면 이미지 기반 생성용 기본 프롬프트 사용
	hasInputImages := false
	if uploadedIds, ok := job.JobInputData["uploadedAttachIds"].([]interface{}); ok && len(uploadedIds) > 0 {
		hasInputImages = true
	}

	// 프롬프트가 비어있을 때 기본값 설정
	if prompt == "" {
		if templatePrompt != "" {
			prompt = templatePrompt
		} else if hasInputImages {
			prompt = "Create a high quality product photo based on this image, professional studio lighting, clean background"
		} else {
			prompt = "best quality, masterpiece"
		}
	}

	// OpenAI로 프롬프트 정제
	refinedPrompt, err := service.RefinePromptWithOpenAI(ctx, prompt, templatePrompt)
	if err != nil {
		log.Printf("⚠️ [Landing] Prompt refinement failed: %v, using original", err)
		if templatePrompt != "" && prompt != templatePrompt {
			refinedPrompt = templatePrompt + ", " + prompt
		} else {
			refinedPrompt = prompt
		}
	}
	log.Printf("📝 [Landing] Refined prompt: %s", truncateString(refinedPrompt, 100))

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

	// 모델 타입 판별
	isSeedream := seedream.IsSeedreamModel(modelID)
	isNanobanana := IsNanobananaModel(modelID)
	isRunware := IsRunwareModel(modelID) && !isSeedream && !isNanobanana // Seedream, Nanobanana 별도 처리
	isMultiview := IsMultiviewModel(modelID)

	if isSeedream {
		log.Printf("🎨 [Landing] Using Seedream API (submodule): %s", modelID)
	} else if isNanobanana {
		log.Printf("🍌 [Landing] Using Nanobanana API (Gemini 2.5 Flash): %s", modelID)
	} else if isRunware {
		log.Printf("🎨 [Landing] Using Runware API: %s", modelID)
	} else if isMultiview {
		log.Printf("🌐 [Landing] Using Multiview API: %s", modelID)
	} else {
		log.Printf("🎨 [Landing] Using Gemini API (default)")
	}

	// 입력 이미지 base64 준비 (Runware용)
	var inputImageBase64 string
	if len(inputImages) > 0 {
		inputImageBase64 = base64.StdEncoding.EncodeToString(inputImages[0])
	}

	// 🚀 병렬 이미지 생성을 위한 결과 구조체
	type GenerationResult struct {
		Index     int
		ImageData []byte
		ImageURL  string // Runware URL (빠른 응답용)
		Error     error
	}

	// 결과 채널
	resultChan := make(chan GenerationResult, quantity)

	log.Printf("🚀 [Landing] Starting PARALLEL image generation: %d images", quantity)

	// 병렬로 이미지 생성 시작
	for i := 0; i < quantity; i++ {
		go func(idx int) {
			// 취소 체크
			if service.IsJobCancelled(job.JobID) {
				resultChan <- GenerationResult{Index: idx, Error: fmt.Errorf("job cancelled")}
				return
			}

			log.Printf("🎨 [Landing] [Parallel] Starting image %d/%d...", idx+1, quantity)

			var generatedImageData []byte
			var genErr error

			if isSeedream {
				// Seedream submodule 사용 - URL만 먼저 반환 (빠른 응답)
				seedreamService := seedream.NewService()
				if seedreamService == nil {
					genErr = fmt.Errorf("Seedream service not initialized")
					resultChan <- GenerationResult{Index: idx, Error: genErr}
					return
				}

				imageURL, err := seedreamService.GenerateWithURL(
					ctx,
					refinedPrompt,
					aspectRatio,
					inputImageBase64,
				)
				if err != nil {
					log.Printf("❌ [Landing] [Parallel] Seedream image %d failed: %v", idx+1, err)
					resultChan <- GenerationResult{Index: idx, Error: err}
					return
				}
				if imageURL == "" {
					log.Printf("❌ [Landing] [Parallel] Seedream image %d: empty URL", idx+1)
					resultChan <- GenerationResult{Index: idx, Error: fmt.Errorf("empty image URL")}
					return
				}
				log.Printf("✅ [Landing] [Parallel] Image %d URL received: %s", idx+1, truncateString(imageURL, 50))
				resultChan <- GenerationResult{Index: idx, ImageURL: imageURL}
				return

			} else if isNanobanana {
				// Nanobanana submodule 사용 (Gemini 2.5 Flash)
				nanobananaService := nanobanana.NewService()
				if nanobananaService == nil {
					genErr = fmt.Errorf("Nanobanana service not initialized")
					resultChan <- GenerationResult{Index: idx, Error: genErr}
					return
				}

				// 입력 이미지 준비
				var images []nanobanana.InputImage
				for _, imgData := range inputImages {
					images = append(images, nanobanana.InputImage{
						Data:     base64.StdEncoding.EncodeToString(imgData),
						MimeType: "image/jpeg",
					})
				}

				req := &nanobanana.GenerateRequest{
					Prompt: refinedPrompt,
					Model:  "", // config.GeminiModel 사용 (gemini-2.5-flash-image)
					Width:  1024,
					Height: 1024,
					Images: images,
				}

				resp, err := nanobananaService.Generate(ctx, req)
				if err != nil {
					log.Printf("❌ [Landing] [Parallel] Nanobanana image %d failed: %v", idx+1, err)
					resultChan <- GenerationResult{Index: idx, Error: err}
					return
				}
				if !resp.Success || resp.ImageBase64 == "" {
					errMsg := "empty image"
					if resp.ErrorMessage != "" {
						errMsg = resp.ErrorMessage
					}
					log.Printf("❌ [Landing] [Parallel] Nanobanana image %d: %s", idx+1, errMsg)
					resultChan <- GenerationResult{Index: idx, Error: fmt.Errorf(errMsg)}
					return
				}

				// Base64 디코딩
				generatedImageData, genErr = base64.StdEncoding.DecodeString(resp.ImageBase64)
				if genErr != nil {
					log.Printf("❌ [Landing] [Parallel] Nanobanana image %d decode failed: %v", idx+1, genErr)
					resultChan <- GenerationResult{Index: idx, Error: genErr}
					return
				}

				log.Printf("✅ [Landing] [Parallel] Nanobanana image %d generated: %d bytes", idx+1, len(generatedImageData))
				resultChan <- GenerationResult{Index: idx, ImageData: generatedImageData}
				return

			} else if isRunware {
				// Runware API 사용 - URL만 먼저 반환 (빠른 응답)
				imageURL, err := service.GenerateImageWithRunwareURL(
					ctx,
					refinedPrompt,
					modelID,
					aspectRatio,
					modelSteps,
					modelCfgScale,
					negativePrompt,
					inputImageBase64,
				)
				if err != nil {
					log.Printf("❌ [Landing] [Parallel] Runware image %d failed: %v", idx+1, err)
					resultChan <- GenerationResult{Index: idx, Error: err}
					return
				}
				if imageURL == "" {
					log.Printf("❌ [Landing] [Parallel] Runware image %d: empty URL", idx+1)
					resultChan <- GenerationResult{Index: idx, Error: fmt.Errorf("empty image URL")}
					return
				}
				log.Printf("✅ [Landing] [Parallel] Image %d URL received (Runware): %s", idx+1, truncateString(imageURL, 50))
				resultChan <- GenerationResult{Index: idx, ImageURL: imageURL}
				return

			} else if isMultiview {
				// Multiview는 일단 Gemini로 fallback (추후 구현)
				log.Printf("⚠️ [Landing] Multiview not implemented yet, using Gemini")
				var generatedBase64 string
				if len(inputImages) > 0 {
					categories := &ImageCategories{
						Clothing:    inputImages,
						Accessories: [][]byte{},
					}
					generatedBase64, genErr = service.GenerateImageWithGeminiMultiple(ctx, categories, refinedPrompt, aspectRatio)
				} else {
					generatedBase64, genErr = service.GenerateImageWithGeminiTextOnly(ctx, refinedPrompt, aspectRatio)
				}
				if genErr != nil {
					log.Printf("❌ [Landing] [Parallel] Multiview image %d failed: %v", idx+1, genErr)
					resultChan <- GenerationResult{Index: idx, Error: genErr}
					return
				}
				if generatedBase64 == "" {
					log.Printf("❌ [Landing] [Parallel] Multiview image %d: empty base64", idx+1)
					resultChan <- GenerationResult{Index: idx, Error: fmt.Errorf("empty image data")}
					return
				}
				generatedImageData, genErr = base64.StdEncoding.DecodeString(generatedBase64)
				if genErr != nil {
					log.Printf("❌ [Landing] [Parallel] Multiview image %d decode failed: %v", idx+1, genErr)
					resultChan <- GenerationResult{Index: idx, Error: genErr}
					return
				}

			} else {
				// Gemini API 사용 (기본)
				var generatedBase64 string
				if len(inputImages) > 0 {
					categories := &ImageCategories{
						Clothing:    inputImages,
						Accessories: [][]byte{},
					}
					generatedBase64, genErr = service.GenerateImageWithGeminiMultiple(ctx, categories, refinedPrompt, aspectRatio)
				} else {
					generatedBase64, genErr = service.GenerateImageWithGeminiTextOnly(ctx, refinedPrompt, aspectRatio)
				}
				if genErr != nil {
					log.Printf("❌ [Landing] [Parallel] Gemini image %d failed: %v", idx+1, genErr)
					resultChan <- GenerationResult{Index: idx, Error: genErr}
					return
				}
				if generatedBase64 == "" {
					log.Printf("❌ [Landing] [Parallel] Gemini image %d: empty base64", idx+1)
					resultChan <- GenerationResult{Index: idx, Error: fmt.Errorf("empty image data")}
					return
				}
				generatedImageData, genErr = base64.StdEncoding.DecodeString(generatedBase64)
				if genErr != nil {
					log.Printf("❌ [Landing] [Parallel] Gemini image %d decode failed: %v", idx+1, genErr)
					resultChan <- GenerationResult{Index: idx, Error: genErr}
					return
				}
			}

			log.Printf("✅ [Landing] [Parallel] Image %d generated successfully", idx+1)
			resultChan <- GenerationResult{Index: idx, ImageData: generatedImageData}
		}(i)
	}

	// 결과 수집 및 처리
	generatedAttachIds := []int{}
	generatedImageURLs := []string{} // 빠른 응답용 URL 저장
	completedCount := 0
	var apiError error

	for i := 0; i < quantity; i++ {
		result := <-resultChan

		if result.Error != nil {
			log.Printf("❌ [Landing] Image %d failed: %v", result.Index+1, result.Error)
			// API 에러 체크 (403, 429)
			if strings.Contains(result.Error.Error(), "403") || strings.Contains(result.Error.Error(), "429") {
				apiError = result.Error
			}
			continue
		}

		// URL이 있는 경우 (Seedream) - 즉시 완료 처리 후 백그라운드에서 Storage 업로드
		if result.ImageURL != "" {
			generatedImageURLs = append(generatedImageURLs, result.ImageURL)
			completedCount++
			log.Printf("✅ [Landing] Image %d/%d URL ready (will upload in background)", result.Index+1, quantity)

			// 즉시 Progress 업데이트 (프론트에서 URL로 이미지 표시)
			if err := service.UpdateJobProgressWithURLs(ctx, job.JobID, completedCount, generatedImageURLs); err != nil {
				log.Printf("⚠️ [Landing] Failed to update progress with URLs: %v", err)
			}

			// 백그라운드에서 다운로드 + 업로드 + Attach 생성
			go func(imageURL string, idx int) {
				bgCtx := context.Background()

				// 이미지 다운로드
				imageData, err := service.DownloadImageFromURL(bgCtx, imageURL)
				if err != nil {
					log.Printf("❌ [Landing] [Background] Failed to download image %d: %v", idx+1, err)
					return
				}

				// Storage 업로드
				filePath, webpSize, err := service.UploadImageToStorage(bgCtx, imageData, userID)
				if err != nil {
					log.Printf("❌ [Landing] [Background] Failed to upload image %d: %v", idx+1, err)
					return
				}

				// Attach 레코드 생성
				attachID, err := service.CreateAttachRecord(bgCtx, filePath, webpSize)
				if err != nil {
					log.Printf("❌ [Landing] [Background] Failed to create attach record %d: %v", idx+1, err)
					return
				}

				log.Printf("✅ [Landing] [Background] Image %d archived: AttachID=%d", idx+1, attachID)

				// Job에 attach_id 추가 (프론트엔드 폴링용)
				if err := service.AppendJobAttachId(bgCtx, job.JobID, attachID); err != nil {
					log.Printf("⚠️ [Landing] [Background] Failed to append attach_id to job: %v", err)
				}

				// 크레딧 차감
				if job.ProductionID != nil && userID != "" {
					if err := service.DeductCredits(bgCtx, userID, job.OrgID, *job.ProductionID, []int{attachID}); err != nil {
						log.Printf("⚠️ [Landing] [Background] Failed to deduct credits for attach %d: %v", attachID, err)
					}
				}

				// Production에 attach_id 추가 (기존 것에 append)
				if job.ProductionID != nil {
					if err := service.AppendProductionAttachId(bgCtx, *job.ProductionID, attachID); err != nil {
						log.Printf("⚠️ [Landing] [Background] Failed to append attach_id: %v", err)
					}
				}
			}(result.ImageURL, result.Index)

			continue
		}

		// ImageData가 있는 경우 (Gemini, Runware 등) - 기존 로직
		// Storage 업로드
		filePath, webpSize, err := service.UploadImageToStorage(ctx, result.ImageData, userID)
		if err != nil {
			log.Printf("❌ [Landing] Failed to upload image %d: %v", result.Index+1, err)
			continue
		}

		// Attach 레코드 생성
		attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
		if err != nil {
			log.Printf("❌ [Landing] Failed to create attach record %d: %v", result.Index+1, err)
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

		log.Printf("✅ [Landing] Image %d/%d completed: AttachID=%d", result.Index+1, quantity, attachID)

		// 진행 상황 업데이트 (병렬이라 완료될 때마다)
		if err := service.UpdateJobProgress(ctx, job.JobID, completedCount, generatedAttachIds); err != nil {
			log.Printf("⚠️ [Landing] Failed to update progress: %v", err)
		}
	}

	// API 에러로 인한 실패 처리
	if apiError != nil && completedCount == 0 {
		log.Printf("🚨 [Landing] All images failed due to API error")
		service.UpdateJobStatus(ctx, job.JobID, model.StatusFailed)
		if job.ProductionID != nil {
			service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusFailed)
		}
		return
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
