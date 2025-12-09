package veo3

// VideoGenerationRequest represents the request to generate a video
type VideoGenerationRequest struct {
	ImageURL       string `json:"imageUrl"`
	AttachID       int    `json:"attachId"`
	Prompt         string `json:"prompt"`
	Duration       int    `json:"duration"` // 5-10 seconds
	FPS            int    `json:"fps"`
	UserID         string `json:"userId"`
	GenerationMode string `json:"generationMode"` // "single", "start-end", "multi-reference"
	StartImageURL  string `json:"startImageUrl,omitempty"`
	EndImageURL    string `json:"endImageUrl,omitempty"`
	ReferenceURLs  []string `json:"referenceUrls,omitempty"`
}

// VideoGenerationJob represents a video generation job in the queue
type VideoGenerationJob struct {
	JobID          string `json:"jobId"`
	UserID         string `json:"userId"`
	ImageURL       string `json:"imageUrl"`
	AttachID       int    `json:"attachId"`
	Prompt         string `json:"prompt"`
	Duration       int    `json:"duration"`
	FPS            int    `json:"fps"`
	GenerationMode string `json:"generationMode"`
	StartImageURL  string `json:"startImageUrl,omitempty"`
	EndImageURL    string `json:"endImageUrl,omitempty"`
	ReferenceURLs  []string `json:"referenceUrls,omitempty"`
	Status         string `json:"status"` // "pending", "processing", "completed", "failed"
	VideoURL       string `json:"videoUrl,omitempty"`
	ErrorMessage   string `json:"errorMessage,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// Veo3Response represents the response from Veo3 API
type Veo3Response struct {
	VideoURL string `json:"videoUrl"`
	JobID    string `json:"jobId"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}
