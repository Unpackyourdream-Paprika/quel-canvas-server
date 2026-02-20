package klingmigration

import "time"

// VideoJobRequest - 비디오 생성 요청 (클라이언트에서 받는 데이터)
type VideoJobRequest struct {
	JobID       string `json:"job_id"`
	ImageBase64 string `json:"imageBase64"`
	Prompt      string `json:"prompt"`
	UserID      string `json:"userId"`
}

// EnqueueVideoRequest - /enqueue-video API 요청
type EnqueueVideoRequest struct {
	JobID       string `json:"job_id"`
	ImageBase64 string `json:"imageBase64,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	UserID      string `json:"userId,omitempty"`
}

// EnqueueVideoResponse - /enqueue-video API 응답
type EnqueueVideoResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message,omitempty"`
	Error         string `json:"error,omitempty"`
	JobID         string `json:"job_id,omitempty"`
	Queue         string `json:"queue,omitempty"`
	QueuePosition int64  `json:"queuePosition,omitempty"`
}

// KlingCreateTaskRequest - Kling AI API 요청 (Image to Video)
type KlingCreateTaskRequest struct {
	Model     string `json:"model"`      // kling-v1
	TaskType  string `json:"task_type"`  // image2video
	Input     KlingInput `json:"input"`
}

// KlingInput - Kling API 입력 데이터
type KlingInput struct {
	ImageURL    string `json:"image_url,omitempty"`    // URL 방식
	ImageBase64 string `json:"image_base64,omitempty"` // Base64 방식
	Prompt      string `json:"prompt"`
	Duration    int    `json:"duration,omitempty"`     // 5 or 10 초
	AspectRatio string `json:"aspect_ratio,omitempty"` // 16:9, 9:16, 1:1
}

// KlingCreateTaskResponse - Kling AI 작업 생성 응답
type KlingCreateTaskResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

// KlingTaskStatusResponse - Kling AI 작업 상태 조회 응답
type KlingTaskStatusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"` // submitted, processing, succeed, failed
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
		TaskResult struct {
			Videos []KlingVideoResult `json:"videos"`
		} `json:"task_result"`
	} `json:"data"`
}

// KlingVideoResult - Kling AI 비디오 결과
type KlingVideoResult struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Duration string `json:"duration"`
}

// VideoJob - 내부 비디오 작업 데이터
type VideoJob struct {
	JobID          string                 `json:"job_id"`
	ProductionID   *string                `json:"production_id"`
	UserID         string                 `json:"user_id"`
	JobType        string                 `json:"job_type"`
	JobStatus      string                 `json:"job_status"`
	TotalImages    int                    `json:"total_images"`
	CompletedImages int                   `json:"completed_images"`
	EstimatedCredits int                  `json:"estimated_credits"`
	JobInputData   map[string]interface{} `json:"job_input_data"`
	KlingTaskID    string                 `json:"kling_task_id,omitempty"`
	VideoURL       string                 `json:"video_url,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}
