package klingmigration

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	appconfig "quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/database"
	redisClient "quel-canvas-server/modules/common/redis"
)

// Worker - Kling Video Worker
type Worker struct {
	rdb      *redis.Client
	dbClient *database.Client
	service  *Service
	config   *Config
}

// NewWorker - Worker 생성
func NewWorker() *Worker {
	cfg := appconfig.GetConfig()

	rdb := redisClient.Connect(cfg)
	if rdb == nil {
		log.Println("❌ [Kling Worker] Failed to connect to Redis")
		return nil
	}

	dbClient := database.NewClient()
	if dbClient == nil {
		log.Println("❌ [Kling Worker] Failed to initialize Database client")
		return nil
	}

	service := NewService()
	if service == nil {
		log.Println("❌ [Kling Worker] Failed to initialize Kling service")
		return nil
	}

	klingConfig := GetConfig()
	if klingConfig == nil {
		log.Println("❌ [Kling Worker] Failed to load Kling config")
		return nil
	}

	log.Println("✅ [Kling Worker] Initialized successfully")
	return &Worker{
		rdb:      rdb,
		dbClient: dbClient,
		service:  service,
		config:   klingConfig,
	}
}

// StartWorker - Redis 큐 감시 시작
func (w *Worker) StartWorker() {
	log.Println("🔄 [Kling Worker] Starting video queue worker...")
	log.Println("👀 [Kling Worker] Watching queue: jobs:video")

	ctx := context.Background()

	for {
		// Job 받기 (BRPOP - Blocking Right Pop)
		result, err := w.rdb.BRPop(ctx, 0, "jobs:video").Result()
		if err != nil {
			log.Printf("❌ [Kling Worker] Redis BRPOP error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// result[0]은 "jobs:video", result[1]이 실제 job_id
		jobID := result[1]
		log.Printf("🎯 [Kling Worker] Received video job: %s", jobID)

		// Job 처리 (동기 처리 - 비디오 생성은 시간이 오래 걸림)
		w.processVideoJob(ctx, jobID)
	}
}

// processVideoJob - 비디오 작업 처리
func (w *Worker) processVideoJob(ctx context.Context, jobID string) {
	log.Printf("🚀 [Kling Worker] Processing video job: %s", jobID)

	// 1. Supabase에서 Job 데이터 조회
	job, err := w.dbClient.FetchJobFromSupabase(jobID)
	if err != nil {
		log.Printf("❌ [Kling Worker] Failed to fetch job %s: %v", jobID, err)
		return
	}

	log.Printf("📦 [Kling Worker] Job Data - Type: %s, Status: %s", job.JobType, job.JobStatus)

	// 2. Job 상태를 processing으로 업데이트
	if err := w.dbClient.UpdateJobStatus(ctx, jobID, "processing"); err != nil {
		log.Printf("⚠️ [Kling Worker] Failed to update job status: %v", err)
	}

	// 3. Job 입력 데이터에서 imageBase64, prompt 추출
	imageBase64, _ := job.JobInputData["imageBase64"].(string)
	prompt, _ := job.JobInputData["prompt"].(string)
	userID, _ := job.JobInputData["userId"].(string)

	if imageBase64 == "" {
		log.Printf("❌ [Kling Worker] Missing imageBase64 in job input")
		w.dbClient.UpdateJobFailed(ctx, jobID, "Missing imageBase64")
		return
	}

	log.Printf("📝 [Kling Worker] Prompt: %s", prompt)
	log.Printf("👤 [Kling Worker] UserID: %s", userID)

	// 4. Kling AI API 호출 - 작업 생성
	taskID, err := w.service.CreateImageToVideoTask(imageBase64, prompt)
	if err != nil {
		log.Printf("❌ [Kling Worker] Failed to create Kling task: %v", err)
		w.dbClient.UpdateJobFailed(ctx, jobID, err.Error())
		return
	}

	log.Printf("✅ [Kling Worker] Kling task created: %s", taskID)

	// 5. Kling AI 작업 완료 대기 (최대 60회 시도 = 약 5분)
	status, err := w.service.WaitForCompletion(taskID, 60)
	if err != nil {
		log.Printf("❌ [Kling Worker] Task failed or timed out: %v", err)
		w.dbClient.UpdateJobFailed(ctx, jobID, err.Error())
		return
	}

	// 6. 비디오 URL 추출
	if len(status.Data.TaskResult.Videos) == 0 {
		log.Printf("❌ [Kling Worker] No videos in result")
		w.dbClient.UpdateJobFailed(ctx, jobID, "No videos in result")
		return
	}

	videoURL := status.Data.TaskResult.Videos[0].URL
	log.Printf("🎬 [Kling Worker] Video URL: %s", videoURL)

	// 7. 비디오를 Supabase Storage에 업로드 (TODO: UploadToStorage 메서드 필요)
	storedURL, err := w.uploadVideoToSupabase(ctx, videoURL, jobID, userID)
	if err != nil {
		log.Printf("⚠️ [Kling Worker] Failed to upload to Supabase, using original URL: %v", err)
		storedURL = videoURL // 실패 시 원본 URL 사용
	}

	// 8. 크레딧 차감 (TODO: DeductCredits 메서드 필요 - 현재는 로그만)
	if userID != "" {
		log.Printf("💰 [Kling Worker] Should deduct %d credits from user %s (not implemented)", w.config.ImagePrice, userID)
	}

	// 9. Job 완료 처리
	if err := w.updateJobWithVideo(ctx, jobID, storedURL); err != nil {
		log.Printf("⚠️ [Kling Worker] Failed to update job with video URL: %v", err)
	}

	log.Printf("✅ [Kling Worker] Video job %s completed successfully", jobID)
}

// uploadVideoToSupabase - 비디오를 Supabase Storage에 업로드
func (w *Worker) uploadVideoToSupabase(ctx context.Context, videoURL, jobID, userID string) (string, error) {
	// 비디오 다운로드
	resp, err := http.Get(videoURL)
	if err != nil {
		return "", fmt.Errorf("failed to download video: %w", err)
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read video data: %w", err)
	}

	// TODO: Supabase Storage 업로드 구현 필요
	// 현재는 원본 URL 반환
	log.Printf("⚠️ [Kling Worker] Storage upload not implemented, using original URL")
	return videoURL, nil
}

// updateJobWithVideo - Job에 비디오 URL 업데이트 및 완료 처리
func (w *Worker) updateJobWithVideo(ctx context.Context, jobID, videoURL string) error {
	// videoURL을 포함한 결과로 완료 처리
	// generated_attach_ids 대신 videoUrl을 job 결과에 저장
	generatedAttachIDs := []interface{}{videoURL}

	if err := w.dbClient.UpdateJobCompleted(ctx, jobID, generatedAttachIDs); err != nil {
		return err
	}

	log.Printf("📝 [Kling Worker] Job completed with video URL: %s", videoURL)
	return nil
}
