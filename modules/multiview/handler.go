package multiview

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"quel-canvas-server/modules/common/config"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	return &Handler{
		service: NewService(),
	}
}

// RegisterRoutes - 라우트 등록
func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/multiview/generate", h.HandleGenerate).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/multiview/check-credits", h.HandleCheckCredits).Methods("GET", "OPTIONS")
	log.Println("✅ Multiview 360 routes registered")
}

// HandleGenerate - POST /api/multiview/generate
// 360도 다각도 이미지 생성 요청 처리
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
		log.Println("❌ [Multiview] Service not initialized")
		json.NewEncoder(w).Encode(MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: "Service unavailable",
			ErrorCode:    ErrCodeInternalError,
		})
		return
	}

	// Request 파싱
	var req MultiviewGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [Multiview] Invalid request: %v", err)
		json.NewEncoder(w).Encode(MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: "Invalid request format",
			ErrorCode:    ErrCodeInvalidRequest,
		})
		return
	}

	// 요청 검증 - 원본 이미지 필수
	if strings.TrimSpace(req.SourceImage) == "" {
		json.NewEncoder(w).Encode(MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: "Source image is required",
			ErrorCode:    ErrCodeImageRequired,
		})
		return
	}

	// 사용자 ID 필수
	if strings.TrimSpace(req.UserID) == "" {
		json.NewEncoder(w).Encode(MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: "User ID is required. Please sign in.",
			ErrorCode:    ErrCodeUnauthorized,
		})
		return
	}

	// 레퍼런스 이미지 개수 제한 (최대 3개)
	if len(req.ReferenceImages) > 3 {
		json.NewEncoder(w).Encode(MultiviewGenerateResponse{
			Success:      false,
			ErrorMessage: "Maximum 3 reference images allowed",
			ErrorCode:    ErrCodeInvalidRequest,
		})
		return
	}

	// 각도 유효성 검사
	for _, angle := range req.Angles {
		if !IsValidAngle(angle) {
			json.NewEncoder(w).Encode(MultiviewGenerateResponse{
				Success:      false,
				ErrorMessage: "Invalid angle value. Angles must be between 0 and 359.",
				ErrorCode:    ErrCodeInvalidAngle,
			})
			return
		}
	}

	// 레퍼런스 이미지 각도 검사
	for _, ref := range req.ReferenceImages {
		if !IsValidAngle(ref.Angle) {
			json.NewEncoder(w).Encode(MultiviewGenerateResponse{
				Success:      false,
				ErrorMessage: "Invalid reference image angle. Angles must be between 0 and 359.",
				ErrorCode:    ErrCodeInvalidAngle,
			})
			return
		}
	}

	ctx := r.Context()

	// 각도 정보 로깅
	angleCount := len(req.Angles)
	if angleCount == 0 {
		angleCount = len(DefaultAngles)
	}

	log.Printf("🔄 [Multiview] Processing request: user=%s, angles=%d, refs=%d, category=%s",
		req.UserID, angleCount, len(req.ReferenceImages), req.Category)

	// 이미지 생성
	response, err := h.service.GenerateMultiview(ctx, &req)
	if err != nil {
		log.Printf("❌ [Multiview] Generation failed: %v", err)
	}

	// 성공/실패 로깅
	if response.Success {
		successCount := 0
		for _, img := range response.GeneratedImages {
			if img.Success {
				successCount++
			}
		}
		log.Printf("✅ [Multiview] Response sent: success=%v, generated=%d/%d, credits=%d",
			response.Success, successCount, response.TotalImages, response.CreditsUsed)
	} else {
		log.Printf("❌ [Multiview] Response sent: success=%v, error=%s",
			response.Success, response.ErrorMessage)
	}

	json.NewEncoder(w).Encode(response)
}

// HandleCheckCredits - GET /api/multiview/check-credits
// 사용자 크레딧 확인 및 가능한 각도 수 계산
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

	// Service 확인
	if h.service == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"errorCode":    ErrCodeInternalError,
			"errorMessage": "Service unavailable",
		})
		return
	}

	// Query parameter에서 userId 추출
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"errorCode":    ErrCodeInvalidRequest,
			"errorMessage": "User ID is required",
		})
		return
	}

	// 요청된 각도 수 (기본: 8개)
	angleCountStr := r.URL.Query().Get("angleCount")
	angleCount := 8 // 기본값
	if angleCountStr != "" {
		var count int
		if _, err := json.Marshal(angleCountStr); err == nil {
			if n, _ := json.Number(angleCountStr).Int64(); n > 0 {
				count = int(n)
				if count > 0 && count <= 360 {
					angleCount = count
				}
			}
		}
	}

	ctx := r.Context()

	creditResult, err := h.service.CheckUserCreditsDetailed(ctx, userID)
	if err != nil {
		log.Printf("⚠️ [Multiview] Failed to check credits: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"errorCode":    ErrCodeInternalError,
			"errorMessage": "Failed to check credits",
		})
		return
	}

	// 크레딧 당 이미지 가격 (config에서 가져오기)
	cfg := config.GetConfig()
	pricePerImage := cfg.ImagePerPrice

	// 가능한 최대 각도 수 계산
	maxAngles := creditResult.AvailableCredits / pricePerImage
	if maxAngles > 360 {
		maxAngles = 360
	}

	// 요청된 각도 수에 필요한 크레딧
	requiredCredits := angleCount * pricePerImage
	canGenerate := creditResult.AvailableCredits >= requiredCredits

	response := map[string]interface{}{
		"success":           true,
		"creditSource":      creditResult.CreditSource,
		"availableCredits":  creditResult.AvailableCredits,
		"personalCredits":   creditResult.PersonalCredits,
		"canFallback":       creditResult.CanFallback,
		"pricePerImage":     pricePerImage,
		"maxAngles":         maxAngles,
		"requestedAngles":   angleCount,
		"requiredCredits":   requiredCredits,
		"canGenerate":       canGenerate,
		"defaultAngles":     DefaultAngles,
	}

	// org 크레딧이 있는 경우 추가
	if creditResult.CreditSource == "organization" {
		response["orgCredits"] = creditResult.OrgCredits
	}

	json.NewEncoder(w).Encode(response)
}
