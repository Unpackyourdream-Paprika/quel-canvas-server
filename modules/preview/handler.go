package preview

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// PreviewHandler handles lightweight slash-node previews without queueing.
type PreviewHandler struct{}

type PreviewRequest struct {
	CategoryID string      `json:"categoryId,omitempty"`
	SessionID  string      `json:"sessionId,omitempty"`
	UserID     string      `json:"userId,omitempty"`
	Payload    interface{} `json:"payload,omitempty"` // arbitrary node/solo composition data
}

type PreviewResponse struct {
	PreviewID  string      `json:"previewId"`
	PreviewURL string      `json:"previewUrl"`
	Echo       interface{} `json:"echo,omitempty"`
}

// NewPreviewHandler creates a handler instance.
func NewPreviewHandler() *PreviewHandler {
	return &PreviewHandler{}
}

// RegisterRoutes wires preview endpoints.
func (h *PreviewHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/preview/slash", h.handlePreview).Methods("POST", "OPTIONS")
}

// handlePreview returns a quick placeholder preview (transparent 1x1 PNG) and echoes the payload.
// This keeps the endpoint fast; replace the PreviewURL logic with actual generation if needed.
func (h *PreviewHandler) handlePreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp := PreviewResponse{
		PreviewID:  time.Now().Format("20060102T150405.000Z07"),
		PreviewURL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGMAAQAABQABDQottAAAAABJRU5ErkJggg==",
		Echo:       req.Payload,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
