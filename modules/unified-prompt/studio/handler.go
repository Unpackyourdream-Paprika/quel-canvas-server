package studio

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"quel-canvas-server/modules/unified-prompt/common"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	return &Handler{
		service: NewService(),
	}
}

// HandleGenerate - POST /api/unified-prompt/studio/generate
// Visual Studio Sandbox에서 이미지 생성 요청 처리 (회원 전용)
func (h *Handler) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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
		log.Println("❌ [Studio] Service not initialized")
		json.NewEncoder(w).Encode(StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Service unavailable",
			ErrorCode:    common.ErrCodeInternalError,
		})
		return
	}

	// Request 파싱
	var req StudioGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Studio] Invalid request: %v", err)
		json.NewEncoder(w).Encode(StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Invalid request format",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	// 요청 검증
	if strings.TrimSpace(req.Prompt) == "" {
		json.NewEncoder(w).Encode(StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Prompt is required",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	if strings.TrimSpace(req.UserID) == "" {
		json.NewEncoder(w).Encode(StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "User ID is required. Please sign in.",
			ErrorCode:    common.ErrCodeUnauthorized,
		})
		return
	}

	if strings.TrimSpace(req.Category) == "" {
		json.NewEncoder(w).Encode(StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Category is required",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	// 카테고리 검증
	if !common.IsValidCategory(req.Category) {
		json.NewEncoder(w).Encode(StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Invalid category: " + req.Category,
			ErrorCode:    common.ErrCodeInvalidCategory,
		})
		return
	}

	// 이미지 개수 제한
	if len(req.ReferenceImages) > 3 {
		json.NewEncoder(w).Encode(StudioGenerateResponse{
			Success:      false,
			ErrorMessage: "Maximum 3 reference images allowed",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	ctx := r.Context()

	log.Printf("🎨 [Studio] Processing request: user=%s, category=%s, prompt=%s, images=%d",
		req.UserID, req.Category, truncateString(req.Prompt, 30), len(req.ReferenceImages))

	// 이미지 생성
	response, err := h.service.GenerateImage(ctx, &req)
	if err != nil {
		log.Printf("❌ [Studio] Generation failed: %v", err)
	}

	log.Printf("✅ [Studio] Response sent: success=%v, attachId=%d",
		response.Success, response.AttachID)

	json.NewEncoder(w).Encode(response)
}

// HandleCheckCredits - GET /api/unified-prompt/studio/check-credits
// 사용자 크레딧 확인
func (h *Handler) HandleCheckCredits(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Query parameter에서 userId 추출
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"errorCode":    common.ErrCodeInvalidRequest,
			"errorMessage": "User ID is required",
		})
		return
	}

	ctx := r.Context()

	credits, err := h.service.CheckUserCredits(ctx, userID)
	if err != nil {
		log.Printf("⚠️ [Studio] Failed to check credits: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"errorCode":    common.ErrCodeInternalError,
			"errorMessage": "Failed to check credits",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"credits": credits,
	})
}

// HandleAnalyze - POST /api/unified-prompt/studio/analyze
// 이미지 분석하여 레시피용 프롬프트 추출
func (h *Handler) HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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
		log.Println("❌ [Studio] Service not initialized")
		json.NewEncoder(w).Encode(StudioAnalyzeResponse{
			Success:      false,
			ErrorMessage: "Service unavailable",
			ErrorCode:    common.ErrCodeInternalError,
		})
		return
	}

	// Request 파싱
	var req StudioAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Studio] Invalid analyze request: %v", err)
		json.NewEncoder(w).Encode(StudioAnalyzeResponse{
			Success:      false,
			ErrorMessage: "Invalid request format",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	// 요청 검증
	if strings.TrimSpace(req.ImageURL) == "" {
		json.NewEncoder(w).Encode(StudioAnalyzeResponse{
			Success:      false,
			ErrorMessage: "Image URL is required",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	ctx := r.Context()

	log.Printf("🔍 [Studio] Processing analyze request: category=%s", req.Category)

	// 이미지 분석
	response, err := h.service.AnalyzeImage(ctx, &req)
	if err != nil {
		log.Printf("❌ [Studio] Analysis failed: %v", err)
	}

	log.Printf("✅ [Studio] Analyze response sent: success=%v", response.Success)

	json.NewEncoder(w).Encode(response)
}
