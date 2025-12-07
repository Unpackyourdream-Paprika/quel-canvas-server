package multiview

// MultiviewGenerateRequest - 360도 다각도 이미지 생성 요청
type MultiviewGenerateRequest struct {
	// 원본 이미지 (정면 기준) - Base64 인코딩
	SourceImage string `json:"sourceImage"`

	// 레퍼런스 이미지들 (특정 각도에서 잘못된 결과 방지용) - 최대 3개
	ReferenceImages []ReferenceImageWithAngle `json:"referenceImages,omitempty"`

	// 생성할 각도 목록 (기본: 0, 45, 90, 135, 180, 225, 270, 315)
	Angles []int `json:"angles,omitempty"`

	// 사용자 ID (회원 전용)
	UserID string `json:"userId"`

	// 세션 ID
	SessionID string `json:"sessionId"`

	// 카테고리 (fashion, beauty, eats, cinema, cartoon)
	Category string `json:"category,omitempty"`

	// 원본 프롬프트 (이미지 분석 시 참고용)
	OriginalPrompt string `json:"originalPrompt,omitempty"`

	// Aspect Ratio (기본: 1:1)
	AspectRatio string `json:"aspectRatio,omitempty"`

	// 배경도 함께 회전할지 여부 (기본: false = 배경 고정)
	RotateBackground bool `json:"rotateBackground,omitempty"`
}

// ReferenceImageWithAngle - 특정 각도에 대한 레퍼런스 이미지
type ReferenceImageWithAngle struct {
	// 레퍼런스 이미지 - Base64 인코딩
	Image string `json:"image"`

	// 이 레퍼런스가 적용될 각도 (예: 90 = 측면)
	Angle int `json:"angle"`

	// 설명 (선택)
	Description string `json:"description,omitempty"`
}

// MultiviewGenerateResponse - 360도 다각도 이미지 생성 응답
type MultiviewGenerateResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`

	// Job ID (비동기 처리용)
	JobID string `json:"jobId,omitempty"`

	// 생성된 이미지들
	GeneratedImages []GeneratedAngleImage `json:"generatedImages,omitempty"`

	// 총 생성 이미지 수
	TotalImages int `json:"totalImages,omitempty"`

	// 크레딧 정보
	CreditsUsed     int `json:"creditsUsed,omitempty"`
	CreditsRemaining int `json:"creditsRemaining,omitempty"`
}

// GeneratedAngleImage - 각도별 생성된 이미지
type GeneratedAngleImage struct {
	// 각도 (0, 45, 90, ...)
	Angle int `json:"angle"`

	// 각도 라벨 (Front, Front-Right, Right, ...)
	AngleLabel string `json:"angleLabel"`

	// 이미지 URL (Supabase Storage)
	ImageURL string `json:"imageUrl,omitempty"`

	// Base64 인코딩된 이미지
	ImageBase64 string `json:"imageBase64,omitempty"`

	// Attach ID (DB 레코드)
	AttachID int `json:"attachId,omitempty"`

	// 생성 성공 여부
	Success bool `json:"success"`

	// 에러 메시지 (실패 시)
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// MultiviewStatusRequest - 작업 상태 조회 요청
type MultiviewStatusRequest struct {
	JobID string `json:"jobId"`
}

// MultiviewStatusResponse - 작업 상태 조회 응답
type MultiviewStatusResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage,omitempty"`

	// 작업 상태 (pending, processing, completed, failed)
	Status string `json:"status"`

	// 진행률 (0-100)
	Progress int `json:"progress"`

	// 완료된 이미지 수
	CompletedCount int `json:"completedCount"`

	// 총 이미지 수
	TotalCount int `json:"totalCount"`

	// 생성된 이미지들 (완료된 것만)
	GeneratedImages []GeneratedAngleImage `json:"generatedImages,omitempty"`
}

// Error codes
const (
	ErrCodeInvalidRequest  = "INVALID_REQUEST"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeInsufficientCredits = "INSUFFICIENT_CREDITS"
	ErrCodeInternalError   = "INTERNAL_ERROR"
	ErrCodeInvalidCategory = "INVALID_CATEGORY"
	ErrCodeInvalidAngle    = "INVALID_ANGLE"
	ErrCodeImageRequired   = "IMAGE_REQUIRED"
)

// Default angles for 360-degree view (8 angles, 45-degree increments)
var DefaultAngles = []int{0, 45, 90, 135, 180, 225, 270, 315}

// AngleLabels - 각도별 라벨
var AngleLabels = map[int]string{
	0:   "Front",
	45:  "Front-Right",
	90:  "Right",
	135: "Back-Right",
	180: "Back",
	225: "Back-Left",
	270: "Left",
	315: "Front-Left",
}

// GetAngleLabel - 각도에 대한 라벨 반환
func GetAngleLabel(angle int) string {
	if label, ok := AngleLabels[angle]; ok {
		return label
	}
	return "Unknown"
}

// IsValidAngle - 유효한 각도인지 확인
func IsValidAngle(angle int) bool {
	return angle >= 0 && angle < 360
}
