package nanobanana

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

// HandleGenerate - POST /api/nanobanana/generate
// 랜딩 페이지용 단순 이미지 생성 (Gemini)
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
		log.Println("❌ [Nanobanana] Service not initialized")
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Service unavailable",
		})
		return
	}

	// Request 파싱
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Nanobanana] Invalid request: %v", err)
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

	log.Printf("🎨 [Nanobanana] Processing request: prompt=%s, model=%s, size=%dx%d",
		truncateString(req.Prompt, 30), req.Model, req.Width, req.Height)

	ctx := r.Context()

	// 이미지 생성
	response, err := h.service.Generate(ctx, &req)
	if err != nil {
		log.Printf("❌ [Nanobanana] Generation failed: %v", err)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Generation failed",
		})
		return
	}

	log.Printf("✅ [Nanobanana] Response sent: success=%v", response.Success)

	json.NewEncoder(w).Encode(response)
}
