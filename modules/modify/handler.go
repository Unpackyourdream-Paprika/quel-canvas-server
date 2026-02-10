package modify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"quel-canvas-server/modules/common/config"
)

type ModifyHandler struct {
	service *Service
}

func NewModifyHandler() *ModifyHandler {
	return &ModifyHandler{
		service: NewService(),
	}
}

// RegisterRoutes - 라우터에 Modify 엔드포인트 등록
func (h *ModifyHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/modify/submit", h.SubmitModifyJob).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/modify/status/{jobId}", h.GetJobStatus).Methods("GET", "OPTIONS")
	log.Println("✅ Modify routes registered: /api/modify/submit, /api/modify/status/{jobId}")
}

// SubmitModifyJob - Modify 작업 제출 (Redis Queue에 추가)
func (h *ModifyHandler) SubmitModifyJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// OPTIONS 요청 처리 (CORS preflight)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 요청 파싱
	var req ModifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Failed to parse request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid request format",
		})
		return
	}

	// 입력 검증
	if req.ImageURL == "" || req.MaskDataURL == "" || req.AttachID == 0 || req.UserID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Missing required fields: imageUrl, maskDataUrl, attachId, userId",
		})
		return
	}

	// Quantity 검증 (1-10)
	if req.Quantity < 1 || req.Quantity > 10 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Quantity must be between 1 and 10",
		})
		return
	}

	// AspectRatio 기본값 설정
	if req.AspectRatio == "" {
		req.AspectRatio = "16:9"
	}

	log.Printf("🎨 Modify job submission:")
	log.Printf("  - User: %s", req.UserID)
	log.Printf("  - Original Attach ID: %d", req.AttachID)
	log.Printf("  - Quantity: %d", req.Quantity)
	log.Printf("  - Prompt: %s", req.Prompt)
	log.Printf("  - Layers: %d", len(req.Layers))
	log.Printf("  - Aspect Ratio: %s", req.AspectRatio)
	log.Printf("  - Has Reference Image: %v", req.ReferenceImage != nil)

	// 1. 크레딧 확인
	cfg := config.GetConfig()
	creditCostPerImage := cfg.ImagePerPrice
	totalCost := creditCostPerImage * req.Quantity
	hasCredits, err := h.service.CheckUserCredits(req.UserID, totalCost)
	if err != nil {
		log.Printf("❌ Failed to check credits: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to verify user credits",
		})
		return
	}

	if !hasCredits {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Insufficient credits. Required: %d, Cost per image: %d", totalCost, creditCostPerImage),
		})
		return
	}

	// 2. Production 생성
	productionID, err := h.service.CreateModifyProduction(req)
	if err != nil {
		log.Printf("❌ Failed to create production: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to create production",
		})
		return
	}

	log.Printf("✅ Production created: %s", productionID)

	// 3. Job 생성 및 Redis Queue에 추가
	jobID := uuid.New().String()
	inputData := ModifyInputData{
		OriginalImageURL:      req.ImageURL,
		OriginalAttachID:      req.AttachID,
		OriginalProductionID:  stringValue(req.OriginalProductionID),
		MaskDataURL:           req.MaskDataURL,
		Prompt:                req.Prompt,
		Layers:                req.Layers, // 색상별 inpaint 지시사항
		ReferenceImageDataURL: req.ReferenceImage,
		Quantity:              req.Quantity,
		AspectRatio:           req.AspectRatio,
		UserID:                req.UserID,
		QuelMemberID:          req.UserID, // userId가 곧 quel_member_id
	}

	err = h.service.CreateJobAndEnqueue(jobID, productionID, inputData)
	if err != nil {
		log.Printf("❌ Failed to create job: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to create modify job",
		})
		return
	}

	log.Printf("✅ Job created and enqueued: %s", jobID)

	// 4. 크레딧 차감 (attachIds는 worker에서 생성되므로 빈 배열 전달)
	err = h.service.DeductCredits(context.Background(), req.UserID, totalCost, productionID, []int64{})
	if err != nil {
		log.Printf("⚠️  Failed to deduct credits (job will still process): %v", err)
	}

	// 성공 응답
	response := ModifyResponse{
		Success:      true,
		JobID:        jobID,
		ProductionID: productionID,
		Message:      fmt.Sprintf("Modify job submitted successfully. %d images will be generated.", req.Quantity),
		TotalImages:  req.Quantity,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetJobStatus - Job 상태 조회
func (h *ModifyHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	jobID := vars["jobId"]

	if jobID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "jobId is required",
		})
		return
	}

	job, err := h.service.FetchJobFromSupabase(jobID)
	if err != nil {
		log.Printf("❌ Failed to fetch job: %v", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Job not found",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}

// Helper function
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
