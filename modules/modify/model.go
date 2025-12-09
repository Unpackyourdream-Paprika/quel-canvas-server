package modify

import "time"

// ModifyJob - quel_production_jobs 테이블 구조 (job_type: "modify")
type ModifyJob struct {
	JobID              string                 `json:"job_id"`
	ProductionID       *string                `json:"production_id"`
	JobType            string                 `json:"job_type"` // "modify"
	StageIndex         *int                   `json:"stage_index"`
	StageName          *string                `json:"stage_name"`
	BatchIndex         *int                   `json:"batch_index"`
	JobStatus          string                 `json:"job_status"` // pending, processing, completed, failed
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
}

// ModifyInputData - job_input_data JSONB 구조
type ModifyInputData struct {
	// 원본 이미지 정보
	OriginalImageURL      string `json:"originalImageUrl"`      // 원본 이미지 Public URL
	OriginalAttachID      int    `json:"originalAttachId"`      // 원본 이미지 attach_id
	OriginalProductionID  string `json:"originalProductionId"`  // 원본이 속한 production_id (선택)

	// Inpaint 마스크 정보
	MaskDataURL           string `json:"maskDataUrl"`           // Base64 mask image (white=modify, black=keep)

	// 프롬프트
	Prompt                string `json:"prompt"`                // Inpaint 지시사항 (optional)

	// 참조 이미지 (optional)
	ReferenceImageDataURL *string `json:"referenceImageDataUrl"` // Base64 reference image (optional)

	// 생성 개수
	Quantity              int    `json:"quantity"`              // 생성할 이미지 개수 (1-10)

	// 이미지 비율
	AspectRatio           string `json:"aspect-ratio"`          // 이미지 비율 (16:9, 4:3, 1:1, etc.)

	// 사용자 정보
	UserID                string `json:"userId"`                // 사용자 ID
	QuelMemberID          string `json:"quelMemberId"`          // quel_member_id
}

// ModifyRequest - HTTP API 요청 구조체
type ModifyRequest struct {
	ImageURL              string  `json:"imageUrl"`              // 원본 이미지 URL
	MaskDataURL           string  `json:"maskDataUrl"`           // Mask 데이터
	Prompt                string  `json:"prompt"`                // Inpaint 프롬프트
	AttachID              int     `json:"attachId"`              // 원본 attach_id
	UserID                string  `json:"userId"`                // 사용자 ID
	ReferenceImage        *string `json:"referenceImage"`        // 참조 이미지 (Base64, optional)
	Quantity              int     `json:"quantity"`              // 생성 개수 (1-10)
	OriginalProductionID  *string `json:"originalProductionId"`  // 원본 production_id (optional)
	AspectRatio           string  `json:"aspectRatio"`           // 이미지 비율 (16:9, 4:3, 1:1, etc.)
}

// ModifyResponse - HTTP API 응답 구조체
type ModifyResponse struct {
	Success      bool   `json:"success"`
	JobID        string `json:"jobId"`
	ProductionID string `json:"productionId"`
	Message      string `json:"message"`
	TotalImages  int    `json:"totalImages"`
}

// Attach - quel_attach 테이블 구조
type Attach struct {
	AttachID           int64      `json:"attach_id"`
	CreatedAt          time.Time  `json:"created_at"`
	AttachOriginalName *string    `json:"attach_original_name"`
	AttachFileName     *string    `json:"attach_file_name"`
	AttachFilePath     string     `json:"attach_file_path"`
	AttachFileSize     *int64     `json:"attach_file_size"`
	AttachFileType     string     `json:"attach_file_type"`
	AttachDirectory    string     `json:"attach_directory"`
	AttachStorageType  *string    `json:"attach_storage_type"`
	QuelMemberID       string     `json:"quel_member_id"`
	IsFavorite         bool       `json:"is_favorite"`
}

// Production - quel_production 테이블 구조
type Production struct {
	ProductionID   string                 `json:"production_id"`
	ProductionName string                 `json:"production_name"`
	JobType        string                 `json:"job_type"` // "modify"
	TotalQuantity  int                    `json:"total_quantity"`
	ImageCount     int                    `json:"image_count"`
	QuelMemberID   string                 `json:"quel_member_id"`
	MetaData       map[string]interface{} `json:"meta_data"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// ProductionAttach - quel_production_attach 관계 테이블
type ProductionAttach struct {
	ProductionID string    `json:"production_id"`
	AttachID     int64     `json:"attach_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// Job Status Constants
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Credit Cost
const (
	ModifyCreditCost = 20 // Modify 작업당 크레딧 비용
)
