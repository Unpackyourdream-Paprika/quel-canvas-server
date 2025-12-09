package common

// UnifiedPromptRequest - 통합 프롬프트 요청 기본 구조체
type UnifiedPromptRequest struct {
	Prompt          string   `json:"prompt"`
	ReferenceImages []string `json:"referenceImages"` // Base64 인코딩된 이미지 배열
	AspectRatio     string   `json:"aspectRatio"`     // "1:1", "16:9", "9:16", "4:3", "3:4"
	UserID          string   `json:"userId"`          // 회원 ID (비회원은 빈 문자열)
	SessionID       string   `json:"sessionId"`       // 브라우저 세션 ID (비회원 제한용)
}

// UnifiedPromptResponse - 통합 프롬프트 응답 기본 구조체
type UnifiedPromptResponse struct {
	Success      bool     `json:"success"`
	JobID        string   `json:"jobId,omitempty"`
	ImageURLs    []string `json:"imageUrls,omitempty"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
	ErrorCode    string   `json:"errorCode,omitempty"`
}

// GuestLimitResponse - 비회원 제한 응답
type GuestLimitResponse struct {
	Success       bool   `json:"success"`
	LimitReached  bool   `json:"limitReached"`
	UsedCount     int    `json:"usedCount"`
	MaxCount      int    `json:"maxCount"`
	ErrorCode     string `json:"errorCode,omitempty"`
	RedirectToLogin bool `json:"redirectToLogin,omitempty"`
}

// LandingJobInput - 랜딩 페이지 Job 입력 데이터
type LandingJobInput struct {
	Prompt          string   `json:"prompt"`
	ReferenceImages []string `json:"referenceImages"`
	AspectRatio     string   `json:"aspectRatio"`
	SessionID       string   `json:"sessionId"`
}

// StudioJobInput - 스튜디오 Job 입력 데이터
type StudioJobInput struct {
	Prompt          string   `json:"prompt"`
	ReferenceImages []string `json:"referenceImages"`
	AspectRatio     string   `json:"aspectRatio"`
	Category        string   `json:"category"` // fashion, beauty, eats, cinema, cartoon
	UserID          string   `json:"userId"`
}

// Error codes
const (
	ErrCodeGuestLimitReached = "GUEST_LIMIT_REACHED"
	ErrCodeInvalidRequest    = "INVALID_REQUEST"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeUnauthorized      = "UNAUTHORIZED"
	ErrCodeInvalidCategory   = "INVALID_CATEGORY"
)

// Guest limits
const (
	MaxGuestGenerations = 2  // 비회원 최대 생성 횟수
	GuestLimitTTL       = 24 // 비회원 제한 TTL (시간)
)

// Valid categories
var ValidCategories = map[string]bool{
	"fashion": true,
	"beauty":  true,
	"eats":    true,
	"cinema":  true,
	"cartoon": true,
}

// IsValidCategory - 카테고리 유효성 검사
func IsValidCategory(category string) bool {
	return ValidCategories[category]
}
