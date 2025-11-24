package veo3

import (
	"encoding/json"
	"net/http"
)

type Veo3Handler struct {
	service *Service
}

func NewVeo3Handler() *Veo3Handler {
	return &Veo3Handler{
		service: NewService(),
	}
}

// GenerateVideo handles video generation requests
// This endpoint adds the job to Redis queue for processing
func (h *Veo3Handler) GenerateVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VideoGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.ImageURL == "" || req.Prompt == "" || req.UserID == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Validate duration (5-10 seconds)
	if req.Duration < 5 || req.Duration > 10 {
		http.Error(w, "Duration must be between 5 and 10 seconds", http.StatusBadRequest)
		return
	}

	// Submit job to queue
	jobID, err := h.service.SubmitJob(&req)
	if err != nil {
		http.Error(w, "Failed to submit job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"jobId":  jobID,
		"status": "pending",
	})
}

// GetJobStatus returns the status of a video generation job
func (h *Veo3Handler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		http.Error(w, "Missing jobId parameter", http.StatusBadRequest)
		return
	}

	job, err := h.service.GetJobStatus(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}
