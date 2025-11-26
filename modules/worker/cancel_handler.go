package worker

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/model"
	redisutil "quel-canvas-server/modules/common/redis"

	"github.com/redis/go-redis/v9"
	supa "github.com/supabase-community/supabase-go"
)

// CancelHandler - Job 취소 API 핸들러
type CancelHandler struct {
	rdb      *redis.Client
	supabase *supa.Client
}

// NewCancelHandler - 핸들러 생성
func NewCancelHandler() *CancelHandler {
	cfg := config.GetConfig()
	if cfg == nil {
		log.Println("❌ [CancelHandler] Failed to get config")
		return nil
	}

	// Redis 연결
	rdb := redisutil.Connect(cfg)
	if rdb == nil {
		log.Println("❌ [CancelHandler] Failed to connect to Redis")
		return nil
	}

	// Supabase 연결
	supabase, err := supa.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, nil)
	if err != nil {
		log.Printf("❌ [CancelHandler] Failed to connect to Supabase: %v", err)
		return nil
	}

	return &CancelHandler{
		rdb:      rdb,
		supabase: supabase,
	}
}

// RegisterRoutes - 라우트 등록
func (h *CancelHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/jobs/{jobId}/cancel", h.CancelJob).Methods("POST", "OPTIONS")
	log.Println("✅ [CancelHandler] Routes registered: POST /api/jobs/{jobId}/cancel")
}

// CancelJob - Job 취소 처리
func (h *CancelHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	vars := mux.Vars(r)
	jobID := vars["jobId"]

	if jobID == "" {
		http.Error(w, `{"error": "jobId is required"}`, http.StatusBadRequest)
		return
	}

	log.Printf("🛑 [CancelHandler] Cancel requested for job: %s", jobID)

	// 1. Redis에 취소 플래그 설정
	if err := redisutil.SetJobCancelled(h.rdb, jobID); err != nil {
		log.Printf("❌ [CancelHandler] Failed to set cancel flag: %v", err)
		http.Error(w, `{"error": "Failed to set cancel flag"}`, http.StatusInternalServerError)
		return
	}

	// 2. DB에서 현재 job 상태 조회
	var jobs []model.ProductionJob
	_, err := h.supabase.From("quel_production_jobs").
		Select("*", "", false).
		Eq("job_id", jobID).
		ExecuteTo(&jobs)

	if err != nil || len(jobs) == 0 {
		log.Printf("❌ [CancelHandler] Job not found: %s", jobID)
		http.Error(w, `{"error": "Job not found"}`, http.StatusNotFound)
		return
	}

	job := jobs[0]

	// 이미 완료/취소된 job은 취소 불가
	if job.JobStatus == model.StatusCompleted || job.JobStatus == model.StatusUserCancelled {
		log.Printf("⚠️ [CancelHandler] Job already %s: %s", job.JobStatus, jobID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":         false,
			"message":         "Job already " + job.JobStatus,
			"job_id":          jobID,
			"job_status":      job.JobStatus,
			"completed_images": job.CompletedImages,
		})
		return
	}

	log.Printf("✅ [CancelHandler] Cancel flag set for job: %s (current status: %s, completed: %d)",
		jobID, job.JobStatus, job.CompletedImages)

	// 응답
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"message":          "Cancel request sent. Job will stop after current image.",
		"job_id":           jobID,
		"current_status":   job.JobStatus,
		"completed_images": job.CompletedImages,
		"total_images":     job.TotalImages,
	})
}
