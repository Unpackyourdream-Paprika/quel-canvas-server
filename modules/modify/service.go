package modify

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/supabase-community/supabase-go"
	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
)

type Service struct {
	supabase    *supabase.Client
	genaiClient *genai.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ Failed to create Supabase client: %v", err)
		return nil
	}

	// Genai 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("❌ Failed to create Genai client: %v", err)
		return nil
	}

	log.Println("✅ Modify service initialized (Supabase, Genai)")
	return &Service{
		supabase:    supabaseClient,
		genaiClient: genaiClient,
	}
}

// CheckUserCredits - 사용자 크레딧 확인
func (s *Service) CheckUserCredits(userID string, requiredCredits int) (bool, error) {
	log.Printf("💳 Checking credits for user %s (required: %d)", userID, requiredCredits)

	var members []map[string]interface{}

	data, _, err := s.supabase.From("quel_member").
		Select("quel_member_credit", "exact", false).
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		log.Printf("❌ Database query error: %v", err)
		return false, fmt.Errorf("failed to query user credits: %w", err)
	}

	log.Printf("📊 Raw response data length: %d bytes", len(data))

	if err := json.Unmarshal(data, &members); err != nil {
		log.Printf("❌ JSON unmarshal error: %v", err)
		log.Printf("   Raw data: %s", string(data))
		return false, fmt.Errorf("failed to parse credits response: %w", err)
	}

	log.Printf("📊 Found %d member records", len(members))

	if len(members) == 0 {
		log.Printf("❌ User not found in database: %s", userID)
		return false, fmt.Errorf("user not found: %s", userID)
	}

	log.Printf("📊 Member data: %+v", members[0])

	credits, ok := members[0]["quel_member_credit"].(float64)
	if !ok {
		log.Printf("❌ Invalid credit value type: %T, value: %v", members[0]["quel_member_credit"], members[0]["quel_member_credit"])
		return false, fmt.Errorf("invalid credit value")
	}

	hasEnough := int(credits) >= requiredCredits
	log.Printf("💰 User %s credits: %d (required: %d) - OK: %v", userID, int(credits), requiredCredits, hasEnough)

	return hasEnough, nil
}

// DeductCredits - 크레딧 차감
func (s *Service) DeductCredits(userID string, amount int) error {
	log.Printf("💳 Deducting %d credits from user %s", amount, userID)

	// 먼저 현재 크레딧 조회
	var members []map[string]interface{}
	data, _, err := s.supabase.From("quel_member").
		Select("quel_member_credit", "exact", false).
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to query user credits: %w", err)
	}

	if err := json.Unmarshal(data, &members); err != nil {
		return fmt.Errorf("failed to parse credits response: %w", err)
	}

	if len(members) == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	currentCredits, ok := members[0]["quel_member_credit"].(float64)
	if !ok {
		return fmt.Errorf("invalid credit value")
	}

	newCredits := int(currentCredits) - amount

	// 크레딧 업데이트
	_, _, err = s.supabase.From("quel_member").
		Update(map[string]interface{}{
			"quel_member_credit": newCredits,
		}, "", "").
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to deduct credits: %w", err)
	}

	log.Printf("✅ Deducted %d credits from user %s (new balance: %d)", amount, userID, newCredits)
	return nil
}

// CreateModifyProduction - Modify Production 생성
func (s *Service) CreateModifyProduction(req ModifyRequest) (string, error) {
	productionID := uuid.New().String()
	productionName := fmt.Sprintf("Modify - %s", time.Now().Format("2006-01-02 15:04"))

	if req.Prompt != "" {
		// 프롬프트가 있으면 프롬프트를 이름에 포함 (최대 50자)
		promptPreview := req.Prompt
		if len(promptPreview) > 50 {
			promptPreview = promptPreview[:47] + "..."
		}
		productionName = fmt.Sprintf("Modify - %s", promptPreview)
	}

	production := map[string]interface{}{
		"production_id":          productionID,
		"production_name":        productionName,
		"production_status":      "processing",
		"total_quantity":         req.Quantity,
		"generated_image_count":  0, // Worker가 완료 후 업데이트
		"quel_member_id":         req.UserID,
		"prompt_text":            req.Prompt,
		"attach_ids":             []int{}, // 빈 배열로 초기화
	}

	_, _, err := s.supabase.From("quel_production_photo").
		Insert(production, false, "", "", "").
		Execute()

	if err != nil {
		return "", fmt.Errorf("failed to create production: %w", err)
	}

	log.Printf("✅ Production created: %s (%s)", productionID, productionName)
	return productionID, nil
}

// CreateJobAndEnqueue - Job 생성 및 Redis Queue에 추가
func (s *Service) CreateJobAndEnqueue(jobID, productionID string, inputData ModifyInputData) error {
	ctx := context.Background()

	// job_input_data를 map으로 변환
	inputDataMap := map[string]interface{}{
		"originalImageUrl":      inputData.OriginalImageURL,
		"originalAttachId":      inputData.OriginalAttachID,
		"originalProductionId":  inputData.OriginalProductionID,
		"maskDataUrl":           inputData.MaskDataURL,
		"prompt":                inputData.Prompt,
		"referenceImageDataUrl": inputData.ReferenceImageDataURL,
		"quantity":              inputData.Quantity,
		"userId":                inputData.UserID,
		"quelMemberId":          inputData.QuelMemberID,
	}

	// quel_production_jobs에 Job 레코드 생성
	job := map[string]interface{}{
		"job_id":              jobID,
		"production_id":       productionID,
		"job_type":            "simple_general", // 체크 제약 조건을 만족하기 위해 simple_general 사용
		"batch_index":         0,                // simple_general 타입에 필수
		"stage_index":         nil,              // simple_general 타입은 NULL이어야 함
		"job_status":          StatusPending,
		"total_images":        inputData.Quantity,
		"completed_images":    0,
		"failed_images":       0,
		"job_input_data":      inputDataMap,
		"retry_count":         0,
		"quel_member_id":      inputData.UserID,
		"quel_production_path": nil, // modify는 production_path가 없음
	}

	_, _, err := s.supabase.From("quel_production_jobs").
		Insert(job, false, "", "", "").
		Execute()

	if err != nil {
		return fmt.Errorf("failed to create job record: %w", err)
	}

	log.Printf("✅ Job record created: %s", jobID)

	// Redis 클라이언트 생성 (common/config 사용)
	cfg := config.GetConfig()

	// TLS 설정
	var tlsConfig *tls.Config
	if cfg.RedisUseTLS {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		}
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Username:     cfg.RedisUsername,
		Password:     cfg.RedisPassword,
		TLSConfig:    tlsConfig,
		DB:           0,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	// Redis Queue에 Job ID만 추가 (worker가 Supabase에서 전체 데이터를 조회)
	err = redisClient.LPush(ctx, "jobs:queue", jobID).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	log.Printf("✅ Job enqueued to Redis: %s", jobID)
	return nil
}

// FetchJobFromSupabase - Job 조회
func (s *Service) FetchJobFromSupabase(jobID string) (*ModifyJob, error) {
	log.Printf("🔍 Fetching job from Supabase: %s", jobID)

	var jobs []ModifyJob

	data, _, err := s.supabase.From("quel_production_jobs").
		Select("*", "exact", false).
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return nil, fmt.Errorf("failed to query Supabase: %w", err)
	}

	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	job := &jobs[0]
	log.Printf("✅ Job fetched: %s (status: %s, %d/%d completed)",
		job.JobID, job.JobStatus, job.CompletedImages, job.TotalImages)

	return job, nil
}

// UpdateJobStatus - Job 상태 업데이트
func (s *Service) UpdateJobStatus(ctx context.Context, jobID string, status string) error {
	log.Printf("📝 Updating job %s status to: %s", jobID, status)

	updateData := map[string]interface{}{
		"job_status": status,
	}

	if status == StatusProcessing {
		updateData["started_at"] = "now()"
	} else if status == StatusCompleted || status == StatusFailed {
		updateData["completed_at"] = "now()"
	}

	_, _, err := s.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	log.Printf("✅ Job %s status updated to: %s", jobID, status)
	return nil
}

// UpdateJobProgress - Job 진행 상황 및 Production의 attach_ids 업데이트
func (s *Service) UpdateJobProgress(ctx context.Context, jobID string, completed, failed int, attachIDs []int64, productionID string) error {
	log.Printf("📊 Updating job %s progress: completed=%d, failed=%d", jobID, completed, failed)

	// generated_attach_ids를 interface{} 배열로 변환
	attachIDsInterface := make([]interface{}, len(attachIDs))
	for i, id := range attachIDs {
		attachIDsInterface[i] = id
	}

	// Job 업데이트
	updateData := map[string]interface{}{
		"completed_images":     completed,
		"failed_images":        failed,
		"generated_attach_ids": attachIDsInterface,
	}

	_, _, err := s.supabase.From("quel_production_jobs").
		Update(updateData, "", "").
		Eq("job_id", jobID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update job progress: %w", err)
	}

	// Production의 attach_ids 업데이트
	_, _, err = s.supabase.From("quel_production_photo").
		Update(map[string]interface{}{
			"attach_ids": attachIDsInterface,
		}, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		log.Printf("⚠️  Failed to update production attach_ids: %v", err)
	}

	return nil
}
