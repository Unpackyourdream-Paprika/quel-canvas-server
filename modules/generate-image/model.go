package generateimage

import "time"

type ImageGenerationRequest struct {
	Prompt      string            `json:"prompt"`
	Style       string            `json:"style,omitempty"`
	Size        string            `json:"size,omitempty"`
	Quality     string            `json:"quality,omitempty"`
	NumImages   int               `json:"num_images,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ImageGenerationResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ImageURL  string    `json:"image_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Error     string    `json:"error,omitempty"`
}

type ImageStatusResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ImageURL  string    `json:"image_url,omitempty"`
	Progress  int       `json:"progress,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	Error     string    `json:"error,omitempty"`
}

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)