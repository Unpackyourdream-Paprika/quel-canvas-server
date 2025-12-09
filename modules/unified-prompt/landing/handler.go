package landing

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

// HandleGenerate - POST /api/unified-prompt/landing/generate
// 랜딩 페이지에서 이미지 생성 요청 처리 (비회원 2회 제한)
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
		log.Println("❌ [Landing] Service not initialized")
		json.NewEncoder(w).Encode(LandingGenerateResponse{
			Success:      false,
			ErrorMessage: "Service unavailable",
			ErrorCode:    common.ErrCodeInternalError,
		})
		return
	}

	// Request 파싱
	var req LandingGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Landing] Invalid request: %v", err)
		json.NewEncoder(w).Encode(LandingGenerateResponse{
			Success:      false,
			ErrorMessage: "Invalid request format",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	// 요청 검증
	if strings.TrimSpace(req.Prompt) == "" {
		json.NewEncoder(w).Encode(LandingGenerateResponse{
			Success:      false,
			ErrorMessage: "Prompt is required",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	if strings.TrimSpace(req.SessionID) == "" {
		json.NewEncoder(w).Encode(LandingGenerateResponse{
			Success:      false,
			ErrorMessage: "Session ID is required",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	// 이미지 개수 제한
	if len(req.ReferenceImages) > 3 {
		json.NewEncoder(w).Encode(LandingGenerateResponse{
			Success:      false,
			ErrorMessage: "Maximum 3 reference images allowed",
			ErrorCode:    common.ErrCodeInvalidRequest,
		})
		return
	}

	ctx := r.Context()

	// 비회원 제한 확인
	usage, limitReached, err := h.service.CheckGuestLimit(ctx, req.SessionID)
	if err != nil {
		log.Printf("⚠️ [Landing] Failed to check guest limit: %v", err)
		// Redis 오류 시에도 계속 진행 (제한 없이)
	}

	// 제한 도달 시
	if limitReached {
		log.Printf("🚫 [Landing] Guest limit reached: session=%s, count=%d", req.SessionID, usage.UsedCount)
		json.NewEncoder(w).Encode(LandingGenerateResponse{
			Success:         false,
			ErrorMessage:    "You've reached the free generation limit. Please sign in to continue.",
			ErrorCode:       common.ErrCodeGuestLimitReached,
			UsedCount:       usage.UsedCount,
			MaxCount:        common.MaxGuestGenerations,
			LimitReached:    true,
			RedirectToLogin: true,
		})
		return
	}

	log.Printf("🎨 [Landing] Processing request: session=%s, prompt=%s, images=%d",
		req.SessionID, truncateString(req.Prompt, 30), len(req.ReferenceImages))

	// 이미지 생성
	response, err := h.service.GenerateImage(ctx, &req)
	if err != nil {
		log.Printf("❌ [Landing] Generation failed: %v", err)
		// 에러 응답은 이미 response에 설정됨
		json.NewEncoder(w).Encode(response)
		return
	}

	// 성공 시 사용 횟수 증가
	if response.Success {
		updatedUsage, err := h.service.IncrementGuestUsage(ctx, req.SessionID)
		if err != nil {
			log.Printf("⚠️ [Landing] Failed to increment usage: %v", err)
		} else {
			response.UsedCount = updatedUsage.UsedCount
			response.MaxCount = common.MaxGuestGenerations
			response.LimitReached = updatedUsage.UsedCount >= common.MaxGuestGenerations

			// 마지막 사용인 경우 알림
			if response.LimitReached {
				response.RedirectToLogin = true
				log.Printf("📢 [Landing] Last free generation used: session=%s", req.SessionID)
			}
		}
	}

	log.Printf("✅ [Landing] Response sent: success=%v, usedCount=%d/%d",
		response.Success, response.UsedCount, response.MaxCount)

	json.NewEncoder(w).Encode(response)
}

// HandleCheckLimit - GET /api/unified-prompt/landing/check-limit
// 비회원 사용 제한 확인
func (h *Handler) HandleCheckLimit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Query parameter에서 sessionId 추출
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		json.NewEncoder(w).Encode(common.GuestLimitResponse{
			Success:   false,
			ErrorCode: common.ErrCodeInvalidRequest,
		})
		return
	}

	ctx := r.Context()

	// 제한 확인
	usage, limitReached, err := h.service.CheckGuestLimit(ctx, sessionID)
	if err != nil {
		log.Printf("⚠️ [Landing] Failed to check limit: %v", err)
		json.NewEncoder(w).Encode(common.GuestLimitResponse{
			Success:      true,
			UsedCount:    0,
			MaxCount:     common.MaxGuestGenerations,
			LimitReached: false,
		})
		return
	}

	json.NewEncoder(w).Encode(common.GuestLimitResponse{
		Success:         true,
		UsedCount:       usage.UsedCount,
		MaxCount:        common.MaxGuestGenerations,
		LimitReached:    limitReached,
		RedirectToLogin: limitReached,
	})
}
