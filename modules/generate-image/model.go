package generateimage

import "time"

// ProductionJob - quel_production_jobs 테이블 구조
type ProductionJob struct {
	JobID              string                 `json:"job_id"`
	ProductionID       *string                `json:"production_id"`
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
}

// JobInputData - job_input_data JSONB 구조
type JobInputData struct {
	Prompt                   string   `json:"prompt"`
	MergedImageAttachID      int      `json:"mergedImageAttachId"`
	IndividualImageAttachIDs []int    `json:"individualImageAttachIds"`
	CameraAngle              string   `json:"cameraAngle"`
	ShotType                 string   `json:"shotType"`
	Quantity                 int      `json:"quantity"`
	UserID                   string   `json:"userId"`
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
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)