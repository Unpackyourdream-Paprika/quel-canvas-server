package klingmigration

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

// Handler - Kling Migration HTTP Handler
type Handler struct {
	rdb     *redis.Client
	service *Service
}

// NewHandler - Handler 생성
func NewHandler() *Handler {
	cfg := config.GetConfig()

	rdb := redisClient.Connect(cfg)
	if rdb == nil {
		log.Println("⚠️ [Kling] Failed to connect to Redis")
		return nil
	}

	service := NewService()
	if service == nil {
		log.Println("⚠️ [Kling] Failed to initialize service (check KLING_AI_ACCESS_KEY)")
		return nil
	}

	log.Println("✅ [Kling] Handler initialized with Redis and Kling AI service")
	return &Handler{
		rdb:     rdb,
		service: service,
	}
}

// RegisterRoutes - 라우트 등록
func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/enqueue-video", h.HandleEnqueueVideo).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/enqueue-video", h.HandleEnqueueVideo).Methods("POST", "OPTIONS")
	log.Println("✅ [Kling] Routes registered: /enqueue-video, /api/enqueue-video")
}

// HandleEnqueueVideo - POST /enqueue-video
func (h *Handler) HandleEnqueueVideo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// OPTIONS 요청 처리
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Request 파싱
	var req EnqueueVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Kling] Invalid request: %v", err)
		json.NewEncoder(w).Encode(EnqueueVideoResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// job_id 검증
	if req.JobID == "" {
		json.NewEncoder(w).Encode(EnqueueVideoResponse{
			Success: false,
			Error:   "job_id is required",
		})
		return
	}

	log.Printf("📥 [Kling] Received video job: %s", req.JobID)

	// Redis LPUSH (jobs:video 큐에 추가)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.rdb.LPush(ctx, "jobs:video", req.JobID).Result()
	if err != nil {
		log.Printf("❌ [Kling] Redis LPUSH failed: %v", err)
		json.NewEncoder(w).Encode(EnqueueVideoResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Queue 길이 조회
	queueLen, _ := h.rdb.LLen(ctx, "jobs:video").Result()

	log.Printf("✅ [Kling] Video job %s enqueued successfully (position: %d)", req.JobID, queueLen)

	json.NewEncoder(w).Encode(EnqueueVideoResponse{
		Success:       true,
		Message:       "Video job enqueued successfully",
		JobID:         req.JobID,
		Queue:         "jobs:video",
		QueuePosition: queueLen,
	})
}
