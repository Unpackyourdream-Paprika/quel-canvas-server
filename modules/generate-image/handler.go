package generateimage

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

func (h *GenerateImageHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	result := h.service.GenerateImage()
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(result))
}

func (h *GenerateImageHandler) GetImageStatus(w http.ResponseWriter, r *http.Request) {
	result := h.service.GetImageStatus()
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(result))
}