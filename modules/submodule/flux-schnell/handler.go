package fluxschnell

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
		log.Println("⚠️ [FluxSchnell] Service initialization failed - check RUNWARE_API_KEY")
		return nil
	}
	return &Handler{
		service: service,
	}
}

// HandleGenerate - POST /api/flux-schnell/generate
// Dream 모드용 빠른 이미지 생성 (Flux Schnell via Runware)
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
		log.Println("❌ [FluxSchnell] Service not initialized")
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Service unavailable - check RUNWARE_API_KEY",
		})
		return
	}

	// Request 파싱
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [FluxSchnell] Invalid request: %v", err)
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

	log.Printf("🎨 [FluxSchnell] Processing request: prompt=%s, size=%dx%d, steps=%d, userID=%s, productionID=%s",
		truncateString(req.Prompt, 30), req.Width, req.Height, req.Steps, req.UserID, req.ProductionID)

	ctx := r.Context()

	// Production status를 processing으로 업데이트
	if req.ProductionID != "" {
		if err := h.service.UpdateProductionStatus(ctx, req.ProductionID, "processing"); err != nil {
			log.Printf("⚠️ [FluxSchnell] Failed to update production status: %v", err)
		}
	}

	// 이미지 생성
	response, err := h.service.Generate(ctx, &req)
	if err != nil {
		log.Printf("❌ [FluxSchnell] Generation failed: %v", err)
		// 실패 시 production status를 failed로 업데이트
		if req.ProductionID != "" {
			h.service.UpdateProductionStatus(ctx, req.ProductionID, "failed")
		}
		json.NewEncoder(w).Encode(GenerateResponse{
			Success:      false,
			ErrorMessage: "Generation failed",
		})
		return
	}

	// 이미지 생성 성공 시
	if response.Success && response.ImageURL != "" {
		// 1. 이미지를 Supabase Storage에 업로드하고 attach 레코드 생성
		if req.UserID != "" {
			attachIdx, err := h.service.UploadImageToStorage(ctx, response.ImageURL, req.UserID)
			if err != nil {
				log.Printf("⚠️ [FluxSchnell] Failed to upload image to storage: %v", err)
			} else {
				// 2. Production에 attach_idx 추가
				if req.ProductionID != "" {
					if err := h.service.UpdateProductionImageComplete(ctx, req.ProductionID, attachIdx); err != nil {
						log.Printf("⚠️ [FluxSchnell] Failed to update production attach_ids: %v", err)
					}
				}
			}

			// 3. 크레딧 차감
			var orgID *string
			if foundOrgID, err := h.service.GetUserOrganization(ctx, req.UserID); err == nil && foundOrgID != "" {
				orgID = &foundOrgID
				log.Printf("🏢 [FluxSchnell] Found organization for user %s: %s", req.UserID, foundOrgID)
			}

			// 크레딧 차감 (1개 이미지)
			if err := h.service.DeductCredits(ctx, req.UserID, orgID, req.ProductionID, 1); err != nil {
				log.Printf("⚠️ [FluxSchnell] Failed to deduct credits: %v", err)
			}
		}
	} else if !response.Success {
		// 생성 실패 시 production status를 failed로 업데이트
		if req.ProductionID != "" {
			h.service.UpdateProductionStatus(ctx, req.ProductionID, "failed")
		}
	}

	log.Printf("✅ [FluxSchnell] Response sent: success=%v", response.Success)

	json.NewEncoder(w).Encode(response)
}
