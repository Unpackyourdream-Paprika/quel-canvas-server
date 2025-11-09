package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/supabase-community/supabase-go"
	"quel-canvas-server/modules/common/config"
	"quel-canvas-server/modules/common/model"
)

type Client struct {
	supabase *supabase.Client
}

// NewClient - Database 클라이언트 생성
func NewClient() *Client {
	cfg := config.GetConfig()

	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ Failed to create Supabase client: %v", err)
		return nil
	}

	return &Client{
		supabase: supabaseClient,
	}
}

// FetchJobFromSupabase - Supabase에서 Job 데이터 조회
func (c *Client) FetchJobFromSupabase(jobID string) (*model.ProductionJob, error) {
	log.Printf("🔍 Fetching job from Supabase: %s", jobID)

	var jobs []model.ProductionJob

	// Supabase에서 Job 조회
	data, _, err := c.supabase.From("quel_production_jobs").
		Select("*", "exact", false).
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return nil, fmt.Errorf("failed to query Supabase: %w", err)
	}

	// JSON 파싱
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	job := &jobs[0]
	log.Printf("✅ Job fetched successfully: %s (status: %s, total_images: %d)",
		job.JobID, job.JobStatus, job.TotalImages)

	return job, nil
}

// UpdateJobStatus - Job 상태 업데이트
func (c *Client) UpdateJobStatus(ctx context.Context, jobID string, status string) error {
	log.Printf("📝 Updating job %s status to: %s", jobID, status)

	updateData := map[string]interface{}{
		"job_status": status,
		"updated_at": "now()",
	}

	if status == model.StatusProcessing {
		updateData["started_at"] = "now()"
	} else if status == model.StatusCompleted || status == model.StatusFailed {
		updateData["completed_at"] = "now()"
	}

	_, _, err := c.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	log.Printf("✅ Job %s status updated to: %s", jobID, status)
	return nil
}

// FetchAttachInfo - quel_attach 테이블에서 파일 정보 조회
func (c *Client) FetchAttachInfo(attachID int) (*model.Attach, error) {
	log.Printf("🔍 Fetching attach info: %d", attachID)

	var attaches []model.Attach

	// Supabase에서 Attach 조회
	data, _, err := c.supabase.From("quel_attach").
		Select("*", "exact", false).
		Eq("attach_id", fmt.Sprintf("%d", attachID)).
		Execute()

	if err != nil {
		return nil, fmt.Errorf("failed to query quel_attach: %w", err)
	}

	// JSON 파싱
	if err := json.Unmarshal(data, &attaches); err != nil {
		return nil, fmt.Errorf("failed to parse attach response: %w", err)
	}

	if len(attaches) == 0 {
		return nil, fmt.Errorf("attach not found: %d", attachID)
	}

	attach := &attaches[0]

	// 실제 경로 값 출력
	var pathStr string
	if attach.AttachFilePath != nil {
		pathStr = *attach.AttachFilePath
	} else if attach.AttachDirectory != nil {
		pathStr = *attach.AttachDirectory
	} else {
		pathStr = "null"
	}

	log.Printf("✅ Attach info fetched: ID=%d, Path=%s", attach.AttachID, pathStr)

	return attach, nil
}

// UpdateProductionPhotoStatus - Production Photo 상태 업데이트
func (c *Client) UpdateProductionPhotoStatus(ctx context.Context, productionID string, status string) error {
	log.Printf("📝 Updating production %s status to: %s", productionID, status)

	updateData := map[string]interface{}{
		"production_status": status,
	}

	_, _, err := c.supabase.From("quel_production_photo").
		Update(updateData, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update production status: %w", err)
	}

	log.Printf("✅ Production %s status updated to: %s", productionID, status)
	return nil
}

// CreateAttachRecord - quel_attach 테이블에 레코드 생성
func (c *Client) CreateAttachRecord(ctx context.Context, filePath string, fileSize int64) (int, error) {
	log.Printf("💾 Creating attach record for: %s", filePath)

	// 파일명 추출
	fileName := filePath[len(filePath)-1:]
	if idx := len(filePath) - 1; idx >= 0 {
		for i := len(filePath) - 1; i >= 0; i-- {
			if filePath[i] == '/' {
				fileName = filePath[i+1:]
				break
			}
		}
	}

	insertData := map[string]interface{}{
		"attach_original_name": fileName,
		"attach_file_name":     fileName,
		"attach_file_path":     filePath,
		"attach_file_size":     fileSize,
		"attach_file_type":     "image/webp",
		"attach_directory":     filePath,
		"attach_storage_type":  "supabase",
	}

	data, _, err := c.supabase.From("quel_attach").
		Insert(insertData, false, "", "", "").
		Execute()

	if err != nil {
		return 0, fmt.Errorf("failed to insert attach record: %w", err)
	}

	// attach_id 추출
	var attaches []model.Attach
	if err := json.Unmarshal(data, &attaches); err != nil {
		return 0, fmt.Errorf("failed to parse attach response: %w", err)
	}

	if len(attaches) == 0 {
		return 0, fmt.Errorf("no attach record returned")
	}

	attachID := int(attaches[0].AttachID)
	log.Printf("✅ Attach record created: ID=%d", attachID)

	return attachID, nil
}

// UpdateJobProgress - Job 진행 상황 업데이트
func (c *Client) UpdateJobProgress(ctx context.Context, jobID string, completedImages int, generatedAttachIds []int) error {
	log.Printf("📊 Updating job progress: %d/%d completed", completedImages, len(generatedAttachIds))

	updateData := map[string]interface{}{
		"completed_images":     completedImages,
		"generated_attach_ids": generatedAttachIds,
		"updated_at":           "now()",
	}

	_, _, err := c.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update job progress: %w", err)
	}

	log.Printf("✅ Job progress updated: %d images completed", completedImages)
	return nil
}

// UpdateProductionAttachIds - Production Photo의 attach_ids 배열에 추가
func (c *Client) UpdateProductionAttachIds(ctx context.Context, productionID string, newAttachIds []int) error {
	log.Printf("📎 Updating production %s attach_ids with %d new IDs", productionID, len(newAttachIds))

	// 1. 기존 attach_ids 조회
	var productions []struct {
		AttachIds []interface{} `json:"attach_ids"`
	}

	data, _, err := c.supabase.From("quel_production_photo").
		Select("attach_ids", "", false).
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to fetch existing attach_ids: %w", err)
	}

	// JSON 파싱
	if err := json.Unmarshal(data, &productions); err != nil {
		return fmt.Errorf("failed to parse productions: %w", err)
	}

	// 2. 기존 배열과 병합
	var existingIds []int
	if len(productions) > 0 && productions[0].AttachIds != nil {
		for _, id := range productions[0].AttachIds {
			if floatID, ok := id.(float64); ok {
				existingIds = append(existingIds, int(floatID))
			}
		}
	}

	// 3. 새로운 ID들 추가
	mergedIds := append(existingIds, newAttachIds...)
	log.Printf("📎 Merged attach_ids: %d existing + %d new = %d total", len(existingIds), len(newAttachIds), len(mergedIds))

	// 4. Production 업데이트 (JSONB는 직접 배열로 전달)
	updateData := map[string]interface{}{
		"attach_ids": mergedIds,
	}

	_, _, err = c.supabase.From("quel_production_photo").
		Update(updateData, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update production attach_ids: %w", err)
	}

	log.Printf("✅ Production attach_ids updated: %v", mergedIds)
	return nil
}
