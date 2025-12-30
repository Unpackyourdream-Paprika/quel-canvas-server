package worker

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"quel-canvas-server/modules/common/config"
	redisClient "quel-canvas-server/modules/common/redis"
)

// EnqueueHandler - Redis Queue Enqueue Handler
type EnqueueHandler struct {
	rdb *redis.Client
}

// EnqueueRequest - Enqueue 요청
type EnqueueRequest struct {
	JobID string `json:"job_id"`
}

// EnqueueResponse - Enqueue 응답
type EnqueueResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message,omitempty"`
	Error         string `json:"error,omitempty"`
	JobID         string `json:"job_id,omitempty"`
	Queue         string `json:"queue,omitempty"`
	QueuePosition int64  `json:"queuePosition,omitempty"`
}

// NewEnqueueHandler - EnqueueHandler 생성
func NewEnqueueHandler() *EnqueueHandler {
	cfg := config.GetConfig()

	rdb := redisClient.Connect(cfg)
	if rdb == nil {
		log.Println("⚠️ [Enqueue] Failed to connect to Redis")
		return nil
	}

	log.Println("✅ [Enqueue] Handler initialized with Redis connection")
	return &EnqueueHandler{
		rdb: rdb,
	}
}

// RegisterRoutes - 라우트 등록
func (h *EnqueueHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/enqueue", h.HandleEnqueue).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/enqueue", h.HandleEnqueue).Methods("POST", "OPTIONS")
	log.Println("✅ Enqueue routes registered: /enqueue, /api/enqueue")
}

// HandleEnqueue - POST /enqueue
func (h *EnqueueHandler) HandleEnqueue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// OPTIONS 요청 처리
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Request 파싱
	var req EnqueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Enqueue] Invalid request: %v", err)
		json.NewEncoder(w).Encode(EnqueueResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// job_id 검증
	if req.JobID == "" {
		json.NewEncoder(w).Encode(EnqueueResponse{
			Success: false,
			Error:   "job_id is required",
		})
		return
	}

	log.Printf("📥 [Enqueue] Received job_id: %s", req.JobID)

	// Redis LPUSH
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.rdb.LPush(ctx, "jobs:queue", req.JobID).Result()
	if err != nil {
		log.Printf("❌ [Enqueue] Redis LPUSH failed: %v", err)
		json.NewEncoder(w).Encode(EnqueueResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Queue 길이 조회
	queueLen, _ := h.rdb.LLen(ctx, "jobs:queue").Result()

	log.Printf("✅ [Enqueue] Job %s enqueued successfully (position: %d)", req.JobID, queueLen)

	json.NewEncoder(w).Encode(EnqueueResponse{
		Success:       true,
		Message:       "Job enqueued successfully",
		JobID:         req.JobID,
		Queue:         "jobs:queue",
		QueuePosition: queueLen,
	})
}
