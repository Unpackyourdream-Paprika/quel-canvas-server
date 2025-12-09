package veo3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	config *Config
	client *http.Client
}

func NewService() *Service {
	return &Service{
		config: LoadConfig(),
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SubmitJob submits a video generation job to Redis queue
func (s *Service) SubmitJob(req *VideoGenerationRequest) (string, error) {
	jobID := uuid.New().String()

	job := &VideoGenerationJob{
		JobID:          jobID,
		UserID:         req.UserID,
		ImageURL:       req.ImageURL,
		AttachID:       req.AttachID,
		Prompt:         req.Prompt,
		Duration:       req.Duration,
		FPS:            req.FPS,
		GenerationMode: req.GenerationMode,
		StartImageURL:  req.StartImageURL,
		EndImageURL:    req.EndImageURL,
		ReferenceURLs:  req.ReferenceURLs,
		Status:         "pending",
		CreatedAt:      time.Now().Format(time.RFC3339),
		UpdatedAt:      time.Now().Format(time.RFC3339),
	}

	// TODO: Add to Redis queue
	// For now, just store in Supabase
	if err := s.storeJobInSupabase(job); err != nil {
		return "", err
	}

	return jobID, nil
}

// GetJobStatus retrieves job status from Supabase
func (s *Service) GetJobStatus(jobID string) (*VideoGenerationJob, error) {
	// TODO: Query Supabase for job status
	// This is a placeholder implementation
	return &VideoGenerationJob{
		JobID:  jobID,
		Status: "pending",
	}, nil
}

// ProcessJob processes a video generation job using Veo3 API
func (s *Service) ProcessJob(job *VideoGenerationJob) error {
	// Update status to processing
	job.Status = "processing"
	job.UpdatedAt = time.Now().Format(time.RFC3339)
	s.updateJobInSupabase(job)

	// Prepare Veo3 API request
	veo3Req := map[string]interface{}{
		"prompt":   job.Prompt,
		"duration": job.Duration,
		"fps":      job.FPS,
		"mode":     job.GenerationMode,
	}

	// Add image(s) based on generation mode
	switch job.GenerationMode {
	case "single":
		veo3Req["image_url"] = job.ImageURL
	case "start-end":
		veo3Req["start_image_url"] = job.StartImageURL
		veo3Req["end_image_url"] = job.EndImageURL
	case "multi-reference":
		veo3Req["reference_urls"] = job.ReferenceURLs
	default:
		veo3Req["image_url"] = job.ImageURL
	}

	reqBody, err := json.Marshal(veo3Req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call Veo3 API
	req, err := http.NewRequest("POST", s.config.Veo3APIEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.Veo3APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Veo3 API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Veo3 API error: %s", string(body))
	}

	// Parse response
	var veo3Resp Veo3Response
	if err := json.NewDecoder(resp.Body).Decode(&veo3Resp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Update job with video URL
	job.Status = "completed"
	job.VideoURL = veo3Resp.VideoURL
	job.UpdatedAt = time.Now().Format(time.RFC3339)

	return s.updateJobInSupabase(job)
}

// storeJobInSupabase stores a new job in Supabase
func (s *Service) storeJobInSupabase(job *VideoGenerationJob) error {
	// TODO: Implement Supabase storage
	// This is a placeholder
	fmt.Printf("Storing job %s in Supabase\n", job.JobID)
	return nil
}

// updateJobInSupabase updates an existing job in Supabase
func (s *Service) updateJobInSupabase(job *VideoGenerationJob) error {
	// TODO: Implement Supabase update
	// This is a placeholder
	fmt.Printf("Updating job %s in Supabase: status=%s\n", job.JobID, job.Status)
	return nil
}
