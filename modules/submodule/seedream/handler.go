package seedream

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	service := NewService()
	if service == nil {
		log.Println("⚠️ [Seedream] Service initialization failed - check RUNWARE_API_KEY")
		return nil
	}
	return &Handler{
		service: service,
	}
}

// HandleGenerate - POST /api/seedream/generate
// 랜딩 페이지용 고품질 이미지 생성 (Seedream 3.0 via Runware)
func (h *Handler) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// OPTIONS 요청 처리
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// POST만 허용
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Service 확인
	if h.service == nil {
		log.Println("❌ [Seedream] Service not initialized")
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Service unavailable - check RUNWARE_API_KEY",
		})
		return
	}

	// Request 파싱
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Seedream] Invalid request: %v", err)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Invalid request format",
		})
		return
	}

	// 요청 검증
	if strings.TrimSpace(req.Prompt) == "" {
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Prompt is required",
		})
		return
	}

	log.Printf("🎨 [Seedream] Processing request: prompt=%s, aspectRatio=%s, images=%d",
		truncateString(req.Prompt, 30), req.AspectRatio, len(req.Images))

	ctx := r.Context()

	// 이미지 생성
	response, err := h.service.Generate(ctx, &req)
	if err != nil {
		log.Printf("❌ [Seedream] Generation failed: %v", err)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Generation failed",
		})
		return
	}

	log.Printf("✅ [Seedream] Response sent: success=%v", response.Success)

	json.NewEncoder(w).Encode(response)
}

// GetService - 외부에서 Service 접근용
func (h *Handler) GetService() *Service {
	return h.service
}
