package model

import "time"

// ProductionJob - quel_production_jobs 테이블 구조
type ProductionJob struct {
	JobID              string                 `json:"job_id"`
	ProductionID       *string                `json:"production_id"`
	QuelProductionPath string                 `json:"quel_production_path"` // 카테고리 경로
	JobType            string                 `json:"job_type"`
	StageIndex         *int                   `json:"stage_index"`
	StageName          *string                `json:"stage_name"`
	BatchIndex         *int                   `json:"batch_index"`
	JobStatus          string                 `json:"job_status"`
	TotalImages        int                    `json:"total_images"`
	CompletedImages    int                    `json:"completed_images"`
	FailedImages       int                    `json:"failed_images"`
	JobInputData       map[string]interface{} `json:"job_input_data"`
	GeneratedAttachIDs []interface{}          `json:"generated_attach_ids"`
	ErrorMessage       *string                `json:"error_message"`
	RetryCount         int                    `json:"retry_count"`
	CreatedAt          time.Time              `json:"created_at"`
	StartedAt          *time.Time             `json:"started_at"`
	CompletedAt        *time.Time             `json:"completed_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	QuelMemberID       *string                `json:"quel_member_id"`       // 멤버 ID
	OrgID              *string                `json:"org_id"`               // 조직 ID (조직 크레딧 사용 시)
	EstimatedCredits   int                    `json:"estimated_credits"`    // 예상 크레딧
}

// Combination - Camera Angle & Shot Type 조합
type Combination struct {
	Angle    string `json:"angle"`    // "front", "side", "profile", "back"
	Shot     string `json:"shot"`     // "tight", "middle", "full"
	Quantity int    `json:"quantity"` // 해당 조합 생성 개수
}

// JobInputData - job_input_data JSONB 구조
type JobInputData struct {
	// 새로운 구조 (다중 조합 지원)
	BasePrompt               string        `json:"basePrompt"`  // angle/shot 제외된 순수 프롬프트
	MergedImageAttachID      int           `json:"mergedImageAttachId"`
	IndividualImageAttachIDs []int         `json:"individualImageAttachIds"`
	Combinations             []Combination `json:"combinations"` // Camera Angle & Shot Type 조합 배열
	UserID                   string        `json:"userId"`

	// 하위 호환성을 위해 유지 (deprecated)
	Prompt      string `json:"prompt"`
	CameraAngle string `json:"cameraAngle"`
	ShotType    string `json:"shotType"`
	Quantity    int    `json:"quantity"`
}

// Attach - quel_attach 테이블 구조
type Attach struct {
	AttachID           int64     `json:"attach_id"`
	CreatedAt          time.Time `json:"created_at"`
	AttachOriginalName *string   `json:"attach_original_name"`
	AttachFileName     *string   `json:"attach_file_name"`
	AttachFilePath     *string   `json:"attach_file_path"`
	AttachFileSize     *int64    `json:"attach_file_size"`
	AttachFileType     *string   `json:"attach_file_type"`
	AttachDirectory    *string   `json:"attach_directory"`
	AttachStorageType  *string   `json:"attach_storage_type"`
}

const (
	StatusPending       = "pending"
	StatusProcessing    = "processing"
	StatusCompleted     = "completed"
	StatusFailed        = "failed"
	StatusUserCancelled = "user_cancelled"
)
