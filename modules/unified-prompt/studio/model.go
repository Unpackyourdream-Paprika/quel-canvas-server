package studio

import "time"

// StudioGenerateRequest - Visual Studio 이미지 생성 요청
type StudioGenerateRequest struct {
	Prompt          string   `json:"prompt"`
	ReferenceImages []string `json:"referenceImages"` // Base64 인코딩된 이미지 (최대 3개)
	AspectRatio     string   `json:"aspectRatio"`     // 기본값: "1:1"
	Category        string   `json:"category"`        // fashion, beauty, eats, cinema, cartoon
	UserID          string   `json:"userId"`          // 필수 - 회원 전용
}

// StudioGenerateResponse - Visual Studio 이미지 생성 응답
type StudioGenerateResponse struct {
	Success      bool   `json:"success"`
	JobID        string `json:"jobId,omitempty"`
	ImageURL     string `json:"imageUrl,omitempty"`    // 생성된 이미지 URL
	ImageBase64  string `json:"imageBase64,omitempty"` // 생성된 이미지 Base64
	AttachID     int    `json:"attachId,omitempty"`    // quel_attach ID
	ErrorMessage string `json:"errorMessage,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
}

// StudioJob - 스튜디오 Job 데이터
type StudioJob struct {
	JobID           string     `json:"jobId"`
	UserID          string     `json:"userId"`
	Category        string     `json:"category"`
	Prompt          string     `json:"prompt"`
	ReferenceImages []string   `json:"referenceImages"`
	AspectRatio     string     `json:"aspectRatio"`
	Status          string     `json:"status"` // pending, processing, completed, failed
	ResultImageURL  string     `json:"resultImageUrl,omitempty"`
	ResultAttachID  int        `json:"resultAttachId,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

// CategoryPromptConfig - 카테고리별 프롬프트 설정
type CategoryPromptConfig struct {
	SystemPrefix    string
	QualityRules    string
	ForbiddenRules  string
}

// Job status constants
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
)

// StudioAnalyzeRequest - 이미지 분석 요청 (레시피 생성용)
type StudioAnalyzeRequest struct {
	ImageURL       string `json:"imageUrl"`       // 분석할 이미지 URL 또는 Base64
	Category       string `json:"category"`       // fashion, beauty, eats, cinema, cartoon
	OriginalPrompt string `json:"originalPrompt"` // 사용자가 입력한 원본 프롬프트
}

// StudioAnalyzeResponse - 이미지 분석 응답
type StudioAnalyzeResponse struct {
	Success      bool   `json:"success"`
	Prompt       string `json:"prompt,omitempty"`       // 분석된 상세 프롬프트
	ErrorMessage string `json:"errorMessage,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
}
