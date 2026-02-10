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
	"quel-canvas-server/modules/common/org"
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

// CreditCheckResult - 크레딧 확인 결과
type CreditCheckResult struct {
	CreditSource     string // "organization" | "personal"
	OrgCredits       int    // org 크레딧 (org 멤버인 경우)
	PersonalCredits  int    // 개인 크레딧
	AvailableCredits int    // 실제 사용 가능한 크레딧
	CanFallback      bool   // org 부족 시 개인으로 fallback 가능 여부
}

// CheckUserCredits - 사용자 크레딧 확인 (기존 호환성 유지)
func (s *Service) CheckUserCredits(userID string, requiredCredits int) (bool, error) {
	log.Printf("💳 Checking credits for user %s (required: %d)", userID, requiredCredits)

	result, err := s.CheckUserCreditsDetailed(context.Background(), userID)
	if err != nil {
		return false, err
	}

	hasEnough := result.AvailableCredits >= requiredCredits
	log.Printf("💰 User %s credits: %d (required: %d) - OK: %v", userID, result.AvailableCredits, requiredCredits, hasEnough)

	return hasEnough, nil
}

// CheckUserCreditsDetailed - 사용자 크레딧 상세 확인 (org/개인 구분)
func (s *Service) CheckUserCreditsDetailed(ctx context.Context, userID string) (*CreditCheckResult, error) {
	result := &CreditCheckResult{}

	// 개인 크레딧 조회
	var members []map[string]interface{}

	data, _, err := s.supabase.From("quel_member").
		Select("quel_member_credit", "exact", false).
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		log.Printf("❌ Database query error: %v", err)
		return nil, fmt.Errorf("failed to query user credits: %w", err)
	}

	if err := json.Unmarshal(data, &members); err != nil {
		log.Printf("❌ JSON unmarshal error: %v", err)
		return nil, fmt.Errorf("failed to parse credits response: %w", err)
	}

	if len(members) == 0 {
		log.Printf("❌ User not found in database: %s", userID)
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	credits, ok := members[0]["quel_member_credit"].(float64)
	if !ok {
		log.Printf("❌ Invalid credit value type: %T, value: %v", members[0]["quel_member_credit"], members[0]["quel_member_credit"])
		return nil, fmt.Errorf("invalid credit value")
	}

	result.PersonalCredits = int(credits)

	// userID로 org_id 조회
	orgID, err := s.GetUserOrganization(ctx, userID)
	if err != nil {
		log.Printf("⚠️ [Modify] Failed to get user organization: %v", err)
	}

	var orgIDPtr *string
	if orgID != "" {
		orgIDPtr = &orgID
	}

	// 조직 크레딧인지 개인 크레딧인지 구분
	isOrgCredit := org.ShouldUseOrgCredit(s.supabase, orgIDPtr)

	if isOrgCredit && orgID != "" {
		// org 크레딧 조회
		var orgs []struct {
			OrgCredit int64 `json:"org_credit"`
		}
		data, _, err := s.supabase.From("quel_organization").
			Select("org_credit", "", false).
			Eq("org_id", orgID).
			Execute()

		if err != nil {
			log.Printf("⚠️ [Modify] Failed to fetch org credits: %v", err)
			// org 크레딧 조회 실패 시 개인 크레딧 사용
			result.CreditSource = "personal"
			result.AvailableCredits = result.PersonalCredits
			result.CanFallback = false
			return result, nil
		}

		if err := json.Unmarshal(data, &orgs); err == nil && len(orgs) > 0 {
			result.OrgCredits = int(orgs[0].OrgCredit)
			result.CreditSource = "organization"
			result.AvailableCredits = result.OrgCredits
			result.CanFallback = result.PersonalCredits > 0
		} else {
			// org 데이터 파싱 실패 시 개인 크레딧 사용
			result.CreditSource = "personal"
			result.AvailableCredits = result.PersonalCredits
			result.CanFallback = false
		}
	} else {
		// 개인 크레딧 사용
		result.CreditSource = "personal"
		result.AvailableCredits = result.PersonalCredits
		result.CanFallback = false
	}

	return result, nil
}

// DeductCredits - 크레딧 차감 (개인/조직 크레딧 지원, org 부족 시 개인으로 fallback)
func (s *Service) DeductCredits(ctx context.Context, userID string, amount int, productionID string, attachIds []int64) error {
	// userID로 org_id 조회
	orgID, err := s.GetUserOrganization(ctx, userID)
	if err != nil {
		log.Printf("⚠️ [Modify] Failed to get user organization: %v", err)
	}

	var orgIDPtr *string
	if orgID != "" {
		orgIDPtr = &orgID
	}

	// 조직 크레딧인지 개인 크레딧인지 구분
	isOrgCredit := org.ShouldUseOrgCredit(s.supabase, orgIDPtr)

	var currentCredits int
	var newBalance int
	usedOrgCredit := false
	fallbackToPersonal := false

	if isOrgCredit && orgID != "" {
		// 조직 크레딧 차감 시도
		var orgs []struct {
			OrgCredit int64 `json:"org_credit"`
		}
		data, _, err := s.supabase.From("quel_organization").
			Select("org_credit", "", false).
			Eq("org_id", orgID).
			Execute()

		if err != nil {
			log.Printf("⚠️ [Modify] Failed to fetch org credits, fallback to personal: %v", err)
			isOrgCredit = false
		} else if err := json.Unmarshal(data, &orgs); err != nil {
			log.Printf("⚠️ [Modify] Failed to parse org data, fallback to personal: %v", err)
			isOrgCredit = false
		} else if len(orgs) == 0 {
			log.Printf("⚠️ [Modify] Organization not found, fallback to personal: %s", orgID)
			isOrgCredit = false
		} else {
			currentCredits = int(orgs[0].OrgCredit)

			// org 크레딧이 부족한 경우 개인 크레딧으로 fallback
			if currentCredits < amount {
				log.Printf("⚠️ [Modify] Insufficient org credits (%d < %d), fallback to personal credits", currentCredits, amount)
				isOrgCredit = false
				fallbackToPersonal = true
			} else {
				// org 크레딧 차감
				log.Printf("💰 [Modify] Deducting ORGANIZATION credits: OrgID=%s, User=%s, Amount=%d", orgID, userID, amount)
				newBalance = currentCredits - amount

				_, _, err = s.supabase.From("quel_organization").
					Update(map[string]interface{}{
						"org_credit": newBalance,
					}, "", "").
					Eq("org_id", orgID).
					Execute()

				if err != nil {
					log.Printf("⚠️ [Modify] Failed to deduct org credits, fallback to personal: %v", err)
					isOrgCredit = false
					fallbackToPersonal = true
				} else {
					usedOrgCredit = true
				}
			}
		}
	}

	if !isOrgCredit {
		// 개인 크레딧 차감
		if fallbackToPersonal {
			log.Printf("💰 [Modify] FALLBACK to PERSONAL credits: User=%s, Amount=%d", userID, amount)
		} else {
			log.Printf("💰 [Modify] Deducting PERSONAL credits: User=%s, Amount=%d", userID, amount)
		}

		var members []map[string]interface{}
		data, _, err := s.supabase.From("quel_member").
			Select("quel_member_credit", "exact", false).
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to query personal credits: %w", err)
		}

		if err := json.Unmarshal(data, &members); err != nil {
			return fmt.Errorf("failed to parse credits response: %w", err)
		}

		if len(members) == 0 {
			return fmt.Errorf("user not found: %s", userID)
		}

		credits, ok := members[0]["quel_member_credit"].(float64)
		if !ok {
			return fmt.Errorf("invalid credit value")
		}

		currentCredits = int(credits)

		// 개인 크레딧도 부족한 경우 에러
		if currentCredits < amount {
			return fmt.Errorf("insufficient credits: required=%d, available=%d", amount, currentCredits)
		}

		newBalance = currentCredits - amount

		_, _, err = s.supabase.From("quel_member").
			Update(map[string]interface{}{
				"quel_member_credit": newBalance,
			}, "", "").
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to deduct personal credits: %w", err)
		}
	}

	log.Printf("💰 [Modify] Credit balance: %d → %d (-%d)", currentCredits, newBalance, amount)

	// 트랜잭션 기록
	for _, attachID := range attachIds {
		transactionData := map[string]interface{}{
			"user_id":          userID,
			"transaction_type": "DEDUCT",
			"amount":           -amount / len(attachIds),
			"balance_after":    newBalance,
			"description":      "Modify Image Generation",
			"api_provider":     "gemini",
			"attach_idx":       attachID,
			"production_idx":   productionID,
		}

		if usedOrgCredit {
			transactionData["org_id"] = orgID
			transactionData["used_by_member_id"] = userID
		}

		_, _, err := s.supabase.From("quel_credits").
			Insert(transactionData, false, "", "", "").
			Execute()

		if err != nil {
			log.Printf("⚠️ [Modify] Failed to record transaction: %v", err)
		}
	}

	if fallbackToPersonal {
		log.Printf("✅ [Modify] Credits deducted (FALLBACK): %d credits from user %s personal account", amount, userID)
	} else if usedOrgCredit {
		log.Printf("✅ [Modify] Credits deducted (ORG): %d credits from organization %s", amount, orgID)
	} else {
		log.Printf("✅ [Modify] Credits deducted (PERSONAL): %d credits from user %s", amount, userID)
	}

	return nil
}

// GetUserOrganization - 사용자의 조직 ID 조회
func (s *Service) GetUserOrganization(ctx context.Context, userID string) (string, error) {
	var members []struct {
		OrgID string `json:"org_id"`
	}

	data, _, err := s.supabase.From("quel_organization_member").
		Select("org_id", "", false).
		Eq("member_id", userID).
		Execute()

	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(data, &members); err != nil {
		return "", err
	}

	if len(members) > 0 {
		return members[0].OrgID, nil
	}

	return "", nil
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
		"layers":                inputData.Layers, // 색상별 inpaint 지시사항
		"referenceImageDataUrl": inputData.ReferenceImageDataURL,
		"quantity":              inputData.Quantity,
		"aspect-ratio":          inputData.AspectRatio,
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
