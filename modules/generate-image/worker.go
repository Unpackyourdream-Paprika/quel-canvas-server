package generateimage

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// StartWorker - Redis Queue Worker 시작
func StartWorker() {
	log.Println("🔄 Redis Queue Worker starting...")

	config := GetConfig()

	// Service 초기화
	service := NewService()
	if service == nil {
		log.Fatal("❌ Failed to initialize Service")
		return
	}

	// 1단계: Redis 연결
	rdb := connectRedis(config)
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
	case "pipeline_stage":
		log.Printf("📌 Pipeline Stage Mode - Processing stage %v", job.StageIndex)
		processPipelineStage(ctx, service, job)

	default:
		log.Printf("⚠️  Job type %s not implemented yet", job.JobType)
		service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
	}
}

// processPipelineStage - Pipeline Stage 모드 처리 (Stage당 최대 3개 동시)
func processPipelineStage(ctx context.Context, service *Service, job *ProductionJob) {
	log.Printf("🚀 Starting Pipeline Stage processing for job: %s", job.JobID)

	// Phase 1: stages 배열 추출
	stages, ok := job.JobInputData["stages"].([]interface{})
	if !ok {
		log.Printf("❌ Failed to get stages array from job_input_data")
		service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
		return
	}

	userID, _ := job.JobInputData["userId"].(string)
	log.Printf("📦 Pipeline has %d stages, UserID=%s", len(stages), userID)

	// Phase 2: Job 상태 업데이트
	if err := service.UpdateJobStatus(ctx, job.JobID, StatusProcessing); err != nil {
		log.Printf("❌ Failed to update job status: %v", err)
		return
	}

	if job.ProductionID != nil {
		if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, StatusProcessing); err != nil {
			log.Printf("⚠️  Failed to update production status: %v", err)
		}
	}

	// Phase 3: Stage 동시 실행 제한 (최대 3개)
	type StageResult struct {
		StageIndex int
		AttachIDs  []int
		Success    int
	}

	results := make([]StageResult, len(stages))

	// Stage 동시 실행 제한 (최대 3개)
	maxConcurrentStages := 3
	stageSemaphore := make(chan struct{}, maxConcurrentStages)
	var wg sync.WaitGroup
	
	fmt.Printf("🎯 Starting stage processing: max %d concurrent stages, total %d stages\n", 
		maxConcurrentStages, len(stages))

	for stageIdx, stageData := range stages {
		wg.Add(1)

		go func(idx int, data interface{}) {
			defer wg.Done()
			
			// Stage 동시 실행 제한
			stageSemaphore <- struct{}{}
			fmt.Printf("🚀 Stage %d: Starting (concurrent slots used: %d/%d)\n", 
				idx, len(stageSemaphore), maxConcurrentStages)
			
			defer func() { 
				<-stageSemaphore // Stage 완료 후 슬롯 해제
				fmt.Printf("✅ Stage %d: Completed, releasing slot\n", idx)
			}()

			stage, ok := data.(map[string]interface{})
			if !ok {
				log.Printf("❌ Invalid stage data at index %d", idx)
				return
			}

			// Stage 데이터 추출
			stageIndex := int(stage["stage_index"].(float64))
			quantity := int(stage["quantity"].(float64))

			log.Printf("🎬 Stage %d: Processing %d images (stage pool limited)", stageIndex, quantity)

			// TODO: 실제 Stage 처리 로직 구현 예정
			// 현재는 임시로 빈 결과 저장
			results[stageIndex] = StageResult{
				StageIndex: stageIndex,
				AttachIDs:  []int{},
				Success:    0,
			}
		}(stageIdx, stageData)
	}

	// 모든 Stage 완료 대기
	wg.Wait()

	log.Printf("✅ All stages completed with stage pool")
	log.Printf("✅ Pipeline Stage processing completed for job: %s", job.JobID)
}

// connectRedis - Redis 연결 설정
func connectRedis(config *Config) *redis.Client {
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

// base64DecodeString - Base64 문자열을 바이트 배열로 디코딩
func base64DecodeString(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}