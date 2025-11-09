package beauty

import (
	"context"
	"log"

	"quel-canvas-server/modules/common/model"
)

// ProcessJob - Beauty 모듈의 Job 처리 진입점
func ProcessJob(ctx context.Context, job *model.ProductionJob) {
	log.Printf("💄 [BEAUTY MODULE] Job %s started (quel_production_path: %s)", job.JobID, job.QuelProductionPath)

	// Service 초기화
	service := NewService()
	if service == nil {
		log.Printf("❌ [BEAUTY MODULE] Failed to initialize service")
		return
	}

	// Job Type에 따라 분기 처리
	switch job.JobType {
	case "single_batch":
		processSingleBatch(ctx, service, job)
	case "pipeline_stage":
		processPipelineStage(ctx, service, job)
	case "simple_general":
		processSimpleGeneral(ctx, service, job)
	case "simple_portrait":
		processSimplePortrait(ctx, service, job)
	default:
		processSingleBatch(ctx, service, job)
	}
}
