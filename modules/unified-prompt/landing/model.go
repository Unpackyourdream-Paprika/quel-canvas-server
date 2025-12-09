package landing

import "time"

// LandingGenerateRequest - 랜딩 페이지 이미지 생성 요청
type LandingGenerateRequest struct {
	Prompt          string   `json:"prompt"`
	ReferenceImages []string `json:"referenceImages"` // Base64 인코딩된 이미지 (최대 3개)
	AspectRatio     string   `json:"aspectRatio"`     // 기본값: "1:1"
	SessionID       string   `json:"sessionId"`       // 브라우저 세션 ID (비회원 제한용)
}

// LandingGenerateResponse - 랜딩 페이지 이미지 생성 응답
type LandingGenerateResponse struct {
	Success       bool   `json:"success"`
	JobID         string `json:"jobId,omitempty"`
	ImageURL      string `json:"imageUrl,omitempty"`      // 생성된 이미지 URL (동기 응답 시)
	ImageBase64   string `json:"imageBase64,omitempty"`   // 생성된 이미지 Base64 (동기 응답 시)
	ErrorMessage  string `json:"errorMessage,omitempty"`
	ErrorCode     string `json:"errorCode,omitempty"`

	// 비회원 제한 정보
	UsedCount       int  `json:"usedCount"`       // 현재까지 사용한 횟수
	MaxCount        int  `json:"maxCount"`        // 최대 허용 횟수
	LimitReached    bool `json:"limitReached"`    // 제한 도달 여부
	RedirectToLogin bool `json:"redirectToLogin"` // 로그인 페이지로 리다이렉트 필요 여부
}

// GuestUsage - 비회원 사용 기록 (Redis에 저장)
type GuestUsage struct {
	SessionID   string    `json:"sessionId"`
	UsedCount   int       `json:"usedCount"`
	FirstUsedAt time.Time `json:"firstUsedAt"`
	LastUsedAt  time.Time `json:"lastUsedAt"`
}

// LandingJob - 랜딩 페이지 전용 Job 데이터
type LandingJob struct {
	JobID           string   `json:"jobId"`
	SessionID       string   `json:"sessionId"`
	Prompt          string   `json:"prompt"`
	ReferenceImages []string `json:"referenceImages"`
	AspectRatio     string   `json:"aspectRatio"`
	Status          string   `json:"status"` // pending, processing, completed, failed
	ResultImageURL  string   `json:"resultImageUrl,omitempty"`
	ErrorMessage    string   `json:"errorMessage,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

// Job status constants
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
)
