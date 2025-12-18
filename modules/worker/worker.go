package worker

import (
	"context"
	"log"
	"time"

	"quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/database"
	redisClient "quel-canvas-server/modules/common/redis"

	"quel-canvas-server/modules/beauty"
	"quel-canvas-server/modules/cartoon"
	"quel-canvas-server/modules/cinema"
	"quel-canvas-server/modules/eats"
	"quel-canvas-server/modules/fashion"
	landingdemo "quel-canvas-server/modules/landing-demo"
	"quel-canvas-server/modules/modify"
	"quel-canvas-server/modules/multiview"
)

// StartWorker - Redis Queue Worker 시작
func StartWorker() {
	log.Println("🔄 Redis Queue Worker starting...")

	cfg := config.GetConfig()

	// Redis 연결
	rdb := redisClient.Connect(cfg)
	if rdb == nil {
		log.Fatal("❌ Failed to connect to Redis")
		return
	}
	log.Println("✅ Redis connected successfully")

	// Database 클라이언트 초기화
	dbClient := database.NewClient()
	if dbClient == nil {
		log.Fatal("❌ Failed to initialize Database client")
		return
	}

	// Queue 감시 시작
	log.Println("👀 Watching queue: jobs:queue")

	ctx := context.Background()

	// 무한 루프로 Queue 감시
	for {
		// Job 받기 (BRPOP - Blocking Right Pop)
		result, err := rdb.BRPop(ctx, 0, "jobs:queue").Result()
		if err != nil {
			log.Printf("❌ Redis BRPOP error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// result[0]은 "jobs:queue", result[1]이 실제 job_id
		jobID := result[1]
		log.Printf("🎯 Received new job: %s", jobID)

		// Job 처리 (goroutine으로 비동기)
		go processJob(ctx, dbClient, jobID)
	}
}

// processJob - Job 처리 함수 (quel_production_path 기반 라우팅)
func processJob(ctx context.Context, dbClient *database.Client, jobID string) {
	log.Printf("🚀 Processing job: %s", jobID)

	// Supabase에서 Job 데이터 조회
	job, err := dbClient.FetchJobFromSupabase(jobID)
	if err != nil {
		log.Printf("❌ Failed to fetch job %s: %v", jobID, err)
		return
	}

	// Job 데이터 로그 출력
	log.Printf("📦 Job Data:")
	log.Printf("   JobID: %s", job.JobID)
	log.Printf("   JobType: %s", job.JobType)
	log.Printf("   QuelProductionPath: %s", job.QuelProductionPath)
	log.Printf("   Status: %s", job.JobStatus)
	log.Printf("   TotalImages: %d", job.TotalImages)

	if job.ProductionID != nil {
		log.Printf("   ProductionID: %s", *job.ProductionID)
	} else {
		log.Printf("   ProductionID: null")
	}

	// job_type이 "modify"이거나 job_input_data에 maskDataUrl이 있으면 modify 모듈로 라우팅
	// (DB 제약으로 인해 job_type은 simple_general로 저장되지만 maskDataUrl로 modify job 식별)
	isModifyJob := job.JobType == "modify"
	if !isModifyJob && job.JobInputData != nil {
		if _, hasMask := job.JobInputData["maskDataUrl"]; hasMask {
			isModifyJob = true
		}
	}

	if isModifyJob {
		log.Printf("🎨 Routing to Modify module (detected via maskDataUrl)")
		modifyService := modify.NewService()
		if modifyService != nil {
			if err := modifyService.ProcessModifyJob(ctx, jobID); err != nil {
				log.Printf("❌ Modify job failed: %v", err)
			}
		} else {
			log.Printf("❌ Failed to initialize Modify service")
		}
		return
	}

	// quel_production_path 기반 라우팅 (기존 로직)
	path := job.QuelProductionPath

	// NULL 또는 빈 문자열은 fashion으로 처리
	if path == "" {
		path = "fashion"
	}

	log.Printf("🔀 Routing to module: %s", path)

	switch path {
	case "fashion":
		log.Printf("👗 Routing to Fashion module")
		fashion.ProcessJob(ctx, job)

	case "beauty":
		log.Printf("💄 Routing to Beauty module")
		beauty.ProcessJob(ctx, job)

	case "eats":
		log.Printf("🍔 Routing to Eats module")
		eats.ProcessJob(ctx, job)

	case "cinema":
		log.Printf("🎬 Routing to Cinema module")
		cinema.ProcessJob(ctx, job)

	case "cartoon":
		log.Printf("🎨 Routing to Cartoon module")
		cartoon.ProcessJob(ctx, job)

	case "multiview":
		log.Printf("🌐 Routing to Multiview module")
		multiview.ProcessJob(ctx, job)

	case "landing":
		log.Printf("🎨 Routing to Landing module")
		landingdemo.ProcessJob(ctx, job)

	default:
		log.Printf("⚠️  Unknown quel_production_path: %s, using Fashion as default", path)
		fashion.ProcessJob(ctx, job)
	}

	log.Printf("✅ Job %s processing completed", jobID)
}
