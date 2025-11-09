package beauty

import (
	"net/http"
)

type GenerateImageHandler struct {
	service *Service
}

func NewGenerateImageHandler() *GenerateImageHandler {
	return &GenerateImageHandler{
		service: NewService(),
	}
}

// 사용하지 않음 (Worker가 Redis Queue에서 처리)
func (h *GenerateImageHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"message": "Use Redis Queue for job submission"}`))
}

// 사용하지 않음 (Frontend가 직접 Supabase 조회)
func (h *GenerateImageHandler) GetImageStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"message": "Query Supabase directly for job status"}`))
}