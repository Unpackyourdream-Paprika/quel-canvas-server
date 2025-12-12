package cancel

import (
	"context"
	"log"

	"quel-canvas-server/modules/common/model"
)

// StageResult - Stage 결과 저장용 (각 모듈에서 사용)
type StageResult struct {
	StageIndex int
	AttachIDs  []int
	Success    int
}

// StatusUpdater - 상태 업데이트 인터페이스
type StatusUpdater interface {
	IsJobCancelled(jobID string) bool
	UpdateJobStatus(ctx context.Context, jobID string, status string) error
	UpdateProductionPhotoStatus(ctx context.Context, productionID string, status string) error
	UpdateProductionAttachIds(ctx context.Context, productionID string, attachIds []int) error
}

// CheckAndHandleCancelBeforeGeneration - 이미지 생성 전 취소 체크
// 취소됐으면 results에 현재까지 생성된 이미지 저장하고 true 반환
func CheckAndHandleCancelBeforeGeneration(
	ctx context.Context,
	service StatusUpdater,
	job *model.ProductionJob,
	stageIndex int,
	stageGeneratedIds []int,
	results []StageResult,
) bool {
	if !service.IsJobCancelled(job.JobID) {
		return false
	}

	log.Printf("🛑 Stage %d: Job %s cancelled, stopping generation", stageIndex, job.JobID)

	// 지금까지 생성된 이미지는 results에 저장
	results[stageIndex] = StageResult{
		StageIndex: stageIndex,
		AttachIDs:  stageGeneratedIds,
		Success:    len(stageGeneratedIds),
	}

	service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
	if job.ProductionID != nil {
		service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
	}

	return true
}

// CheckAndHandleCancelAfterGeneration - 이미지 생성 후 취소 체크 (저장/차감 전)
// 취소됐으면 results에 현재까지 생성된 이미지 저장하고 true 반환
func CheckAndHandleCancelAfterGeneration(
	ctx context.Context,
	service StatusUpdater,
	job *model.ProductionJob,
	stageIndex int,
	imageIndex int,
	stageGeneratedIds []int,
	results []StageResult,
) bool {
	if !service.IsJobCancelled(job.JobID) {
		return false
	}

	log.Printf("🛑 Stage %d: Job %s cancelled after generation, discarding image %d", stageIndex, job.JobID, imageIndex+1)

	// 지금까지 생성된 이미지는 results에 저장
	results[stageIndex] = StageResult{
		StageIndex: stageIndex,
		AttachIDs:  stageGeneratedIds,
		Success:    len(stageGeneratedIds),
	}

	service.UpdateJobStatus(ctx, job.JobID, model.StatusUserCancelled)
	if job.ProductionID != nil {
		service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, model.StatusUserCancelled)
	}

	return true
}

// CheckCancelForRetryPhase - 재시도 단계 진입 전 취소 체크
func CheckCancelForRetryPhase(service StatusUpdater, jobID string) bool {
	if service.IsJobCancelled(jobID) {
		log.Printf("🛑 Job %s cancelled, skipping retry phase", jobID)
		return true
	}
	return false
}

// CheckCancelDuringRetry - 재시도 루프 중 취소 체크
func CheckCancelDuringRetry(service StatusUpdater, jobID string, stageIdx int) bool {
	if service.IsJobCancelled(jobID) {
		log.Printf("🛑 Stage %d: Job %s cancelled during retry", stageIdx, jobID)
		return true
	}
	return false
}

// HandleFinalStatus - 최종 상태 처리 (취소된 경우 상태 유지)
// 취소됐으면 true 반환 (completed로 덮어쓰지 않음)
func HandleFinalStatus(
	ctx context.Context,
	service StatusUpdater,
	job *model.ProductionJob,
	allGeneratedAttachIds []int,
) bool {
	if !service.IsJobCancelled(job.JobID) {
		return false
	}

	log.Printf("🛑 Job %s was cancelled, keeping user_cancelled status", job.JobID)

	// attach_ids만 업데이트 (이미 생성된 이미지들)
	if job.ProductionID != nil && len(allGeneratedAttachIds) > 0 {
		if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, allGeneratedAttachIds); err != nil {
			log.Printf("Failed to update production attach_ids: %v", err)
		}
	}

	log.Printf("Pipeline Stage processing completed for job: %s (cancelled with %d images)", job.JobID, len(allGeneratedAttachIds))
	return true
}
