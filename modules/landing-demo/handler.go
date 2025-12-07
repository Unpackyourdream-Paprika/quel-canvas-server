package landingdemo

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
	return &Handler{
		service: NewService(),
	}
}

// HandleGenerate - POST /api/landing-demo/generate
// 랜딩 페이지 체험존 이미지 생성 (무제한)
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
		log.Println("❌ [LandingDemo] Service not initialized")
		json.NewEncoder(w).Encode(LandingDemoResponse{
			Success:      false,
			ErrorMessage: "Service unavailable",
		})
		return
	}

	// Request 파싱
	var req LandingDemoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [LandingDemo] Invalid request: %v", err)
		json.NewEncoder(w).Encode(LandingDemoResponse{
			Success:      false,
			ErrorMessage: "Invalid request format",
		})
		return
	}

	// 요청 검증
	if strings.TrimSpace(req.Prompt) == "" {
		json.NewEncoder(w).Encode(LandingDemoResponse{
			Success:      false,
			ErrorMessage: "Prompt is required",
		})
		return
	}

	log.Printf("🎨 [LandingDemo] Processing request: prompt=%s, images=%d, ratio=%s, qty=%d",
		truncateString(req.Prompt, 30), len(req.Images), req.AspectRatio, req.Quantity)

	ctx := r.Context()

	// 이미지 생성 (무제한 - 크레딧 차감 없음)
	response, err := h.service.GenerateImages(ctx, &req)
	if err != nil {
		log.Printf("❌ [LandingDemo] Generation failed: %v", err)
		json.NewEncoder(w).Encode(LandingDemoResponse{
			Success:      false,
			ErrorMessage: "Generation failed",
		})
		return
	}

	log.Printf("✅ [LandingDemo] Response sent: success=%v, images=%d",
		response.Success, len(response.Images))

	json.NewEncoder(w).Encode(response)
}
