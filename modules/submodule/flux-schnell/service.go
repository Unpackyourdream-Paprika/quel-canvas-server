package fluxschnell

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/supabase-community/supabase-go"

	"quel-canvas-server/modules/common/config"
)

// Flux Schnell 모델 ID (Runware)
const FluxSchnellModelID = "runware:100@1"

type Service struct {
	httpClient *http.Client
	supabase   *supabase.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		log.Println("⚠️ [FluxSchnell] RUNWARE_API_KEY not configured")
		return nil
	}

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ [FluxSchnell] Failed to create Supabase client: %v", err)
		return nil
	}

	log.Println("✅ [FluxSchnell] Service initialized with Supabase")
	return &Service{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		supabase:   supabaseClient,
	}
}

// Generate - Flux Schnell로 이미지 생성
func (s *Service) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: "RUNWARE_API_KEY not configured",
		}, nil
	}

	// 기본값 설정
	width := req.Width
	if width <= 0 {
		width = 1024
	}
	height := req.Height
	if height <= 0 {
		height = 1024
	}
	steps := req.Steps
	if steps <= 0 {
		steps = 4 // Flux Schnell 기본 steps
	}
	cfgScale := req.CFGScale
	if cfgScale <= 0 {
		cfgScale = 1.0
	}

	log.Printf("🎨 [FluxSchnell] Generating image - size: %dx%d, steps: %d, cfg: %.1f, prompt: %s",
		width, height, steps, cfgScale, truncateString(req.Prompt, 50))

	// Runware 요청 구성
	runwareReq := RunwareRequest{
		TaskType:       "imageInference",
		TaskUUID:       generateUUID(),
		PositivePrompt: req.Prompt,
		Model:          FluxSchnellModelID,
		Width:          width,
		Height:         height,
		NumberResults:  1,
		OutputFormat:   "PNG",
		Steps:          steps,
		CFGScale:       cfgScale,
	}

	// Negative prompt 추가
	if req.NegativePrompt != "" {
		runwareReq.NegativePrompt = req.NegativePrompt
	}

	// 입력 이미지가 있으면 img2img 모드
	if len(req.Images) > 0 && req.Images[0].Data != "" {
		runwareReq.InputImage = "data:" + req.Images[0].MimeType + ";base64," + req.Images[0].Data
		strength := req.Strength
		if strength <= 0 {
			strength = 0.7
		}
		runwareReq.Strength = strength
		log.Printf("📷 [FluxSchnell] img2img mode - strength: %.2f", strength)
	}

	// API 호출
	jsonBody, err := json.Marshal([]RunwareRequest{runwareReq})
	if err != nil {
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to marshal request: %v", err),
		}, nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cfg.RunwareAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create request: %v", err),
		}, nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.RunwareAPIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("❌ [FluxSchnell] Runware API error: %v", err)
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Runware API error: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// 응답 파싱
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to read response: %v", err),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ [FluxSchnell] Runware API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Runware API error: %s", string(bodyBytes)),
		}, nil
	}

	var runwareResp RunwareResponse
	if err := json.Unmarshal(bodyBytes, &runwareResp); err != nil {
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse response: %v", err),
		}, nil
	}

	if runwareResp.Error != "" {
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: runwareResp.Error,
		}, nil
	}

	// 이미지 URL 추출
	if len(runwareResp.Data) > 0 && runwareResp.Data[0].ImageURL != "" {
		imageURL := runwareResp.Data[0].ImageURL

		// 이미지 다운로드해서 base64로 변환
		imageBase64, err := s.downloadImageAsBase64(ctx, imageURL)
		if err != nil {
			log.Printf("⚠️ [FluxSchnell] Failed to download image, returning URL: %v", err)
			return &GenerateResponse{
				Success:  true,
				ImageURL: imageURL,
			}, nil
		}

		log.Printf("✅ [FluxSchnell] Image generated successfully")
		return &GenerateResponse{
			Success:     true,
			ImageURL:    imageURL,
			ImageBase64: imageBase64,
		}, nil
	}

	return &GenerateResponse{
		Success:      false,
		ErrorMessage: "No image generated from Runware",
	}, nil
}

// downloadImageAsBase64 - URL에서 이미지 다운로드 후 base64 반환
func (s *Service) downloadImageAsBase64(ctx context.Context, imageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(imageData), nil
}

// GenerateWithURL - URL만 반환 (빠른 응답용, 다운로드 없음)
func (s *Service) GenerateWithURL(ctx context.Context, req *GenerateRequest) (string, error) {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		return "", fmt.Errorf("RUNWARE_API_KEY not configured")
	}

	// 기본값 설정
	width := req.Width
	if width <= 0 {
		width = 1024
	}
	height := req.Height
	if height <= 0 {
		height = 1024
	}
	steps := req.Steps
	if steps <= 0 {
		steps = 4
	}
	cfgScale := req.CFGScale
	if cfgScale <= 0 {
		cfgScale = 1.0
	}

	log.Printf("🎨 [FluxSchnell] URL request - size: %dx%d, steps: %d, prompt: %s",
		width, height, steps, truncateString(req.Prompt, 50))

	// Runware 요청 구성
	runwareReq := RunwareRequest{
		TaskType:       "imageInference",
		TaskUUID:       generateUUID(),
		PositivePrompt: req.Prompt,
		Model:          FluxSchnellModelID,
		Width:          width,
		Height:         height,
		NumberResults:  1,
		OutputFormat:   "PNG",
		Steps:          steps,
		CFGScale:       cfgScale,
	}

	if req.NegativePrompt != "" {
		runwareReq.NegativePrompt = req.NegativePrompt
	}

	if len(req.Images) > 0 && req.Images[0].Data != "" {
		runwareReq.InputImage = "data:" + req.Images[0].MimeType + ";base64," + req.Images[0].Data
		strength := req.Strength
		if strength <= 0 {
			strength = 0.7
		}
		runwareReq.Strength = strength
	}

	jsonBody, err := json.Marshal([]RunwareRequest{runwareReq})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cfg.RunwareAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.RunwareAPIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("Runware API error: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Runware API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var runwareResp RunwareResponse
	if err := json.Unmarshal(bodyBytes, &runwareResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if runwareResp.Error != "" {
		return "", fmt.Errorf(runwareResp.Error)
	}

	if len(runwareResp.Data) > 0 && runwareResp.Data[0].ImageURL != "" {
		imageURL := runwareResp.Data[0].ImageURL
		log.Printf("✅ [FluxSchnell] URL received: %s", truncateString(imageURL, 50))
		return imageURL, nil
	}

	return "", fmt.Errorf("no image URL in response")
}

// DownloadImageFromURL - URL에서 이미지 다운로드 (외부에서 호출 가능)
func (s *Service) DownloadImageFromURL(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// IsFluxSchnellModel - Flux Schnell 모델 여부 확인
func IsFluxSchnellModel(modelID string) bool {
	return modelID == FluxSchnellModelID ||
		modelID == "flux-schnell" ||
		modelID == "runware:100@1"
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func generateUUID() string {
	return uuid.New().String()
}

// UploadImageToStorage - Runware 이미지를 Supabase Storage에 업로드하고 attach 레코드 생성
func (s *Service) UploadImageToStorage(ctx context.Context, imageURL string, userID string) (int, error) {
	cfg := config.GetConfig()

	// 1. Runware에서 이미지 다운로드
	log.Printf("📥 [FluxSchnell] Downloading image from Runware: %s", truncateString(imageURL, 50))

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read image data: %w", err)
	}

	log.Printf("📦 [FluxSchnell] Downloaded image: %d bytes", len(imageData))

	// 2. 파일명 및 경로 생성
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	randomID := rand.Intn(999999)
	fileName := fmt.Sprintf("dream_%d_%d.png", timestamp, randomID)
	filePath := fmt.Sprintf("%s/ai-assistant/%s", userID, fileName)

	log.Printf("📤 [FluxSchnell] Uploading to Storage: attachments/%s", filePath)

	// 3. Supabase Storage에 업로드
	uploadURL := fmt.Sprintf("%s/storage/v1/object/attachments/%s", cfg.SupabaseURL, filePath)

	uploadReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(imageData))
	if err != nil {
		return 0, fmt.Errorf("failed to create upload request: %w", err)
	}

	uploadReq.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceKey)
	uploadReq.Header.Set("Content-Type", "image/png")

	uploadResp, err := s.httpClient.Do(uploadReq)
	if err != nil {
		return 0, fmt.Errorf("failed to upload image: %w", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(uploadResp.Body)
		return 0, fmt.Errorf("upload failed with status %d: %s", uploadResp.StatusCode, string(body))
	}

	log.Printf("✅ [FluxSchnell] Image uploaded to Storage: %s", filePath)

	// 4. quel_attach 테이블에 레코드 생성
	attachData := map[string]interface{}{
		"attach_original_name": fileName,
		"attach_file_name":     fileName,
		"attach_file_path":     filePath,
		"attach_file_size":     len(imageData),
		"attach_file_type":     "image/png",
		"attach_directory":     filePath,
		"attach_storage_type":  "supabase",
	}

	data, _, err := s.supabase.From("quel_attach").
		Insert(attachData, false, "", "", "").
		Execute()

	if err != nil {
		return 0, fmt.Errorf("failed to create attach record: %w", err)
	}

	// attach_idx 추출
	var attaches []struct {
		AttachID int `json:"attach_id"`
	}
	if err := json.Unmarshal(data, &attaches); err != nil {
		return 0, fmt.Errorf("failed to parse attach response: %w", err)
	}

	if len(attaches) == 0 {
		return 0, fmt.Errorf("no attach record returned")
	}

	attachIdx := attaches[0].AttachID
	log.Printf("✅ [FluxSchnell] Attach record created: attach_idx=%d, path=%s", attachIdx, filePath)

	return attachIdx, nil
}

// GetUserOrganization - 유저가 속한 조직 ID 조회
func (s *Service) GetUserOrganization(ctx context.Context, userID string) (string, error) {
	var members []struct {
		OrgID string `json:"org_id"`
	}

	data, _, err := s.supabase.From("quel_organization_member").
		Select("org_id", "", false).
		Eq("member_id", userID).
		Eq("status", "active").
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

// DeductCredits - 크레딧 차감 (조직 또는 개인)
func (s *Service) DeductCredits(ctx context.Context, userID string, orgID *string, productionID string, imageCount int) error {
	cfg := config.GetConfig()
	creditsPerImage := cfg.ImagePerPrice
	totalCredits := imageCount * creditsPerImage

	// 조직 크레딧인지 개인 크레딧인지 구분
	isOrgCredit := orgID != nil && *orgID != ""

	if isOrgCredit {
		log.Printf("💰 [FluxSchnell] Deducting ORGANIZATION credits: OrgID=%s, User=%s, Images=%d, Total=%d credits", *orgID, userID, imageCount, totalCredits)
	} else {
		log.Printf("💰 [FluxSchnell] Deducting PERSONAL credits: User=%s, Images=%d, Total=%d credits", userID, imageCount, totalCredits)
	}

	var currentCredits int
	var newBalance int

	if isOrgCredit {
		// 조직 크레딧 차감
		var orgs []struct {
			OrgCredit int64 `json:"org_credit"`
		}

		data, _, err := s.supabase.From("quel_organization").
			Select("org_credit", "", false).
			Eq("org_id", *orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to fetch organization credits: %w", err)
		}

		if err := json.Unmarshal(data, &orgs); err != nil {
			return fmt.Errorf("failed to parse organization data: %w", err)
		}

		if len(orgs) == 0 {
			return fmt.Errorf("organization not found: %s", *orgID)
		}

		currentCredits = int(orgs[0].OrgCredit)
		newBalance = currentCredits - totalCredits

		log.Printf("💰 [FluxSchnell] Organization credit balance: %d → %d (-%d)", currentCredits, newBalance, totalCredits)

		// 조직 크레딧 차감
		_, _, err = s.supabase.From("quel_organization").
			Update(map[string]interface{}{
				"org_credit": newBalance,
			}, "", "").
			Eq("org_id", *orgID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to deduct organization credits: %w", err)
		}

		// 트랜잭션 기록 - 조직 크레딧
		for i := 0; i < imageCount; i++ {
			transactionData := map[string]interface{}{
				"user_id":           userID,
				"org_id":            *orgID,
				"used_by_member_id": userID,
				"transaction_type":  "DEDUCT",
				"amount":            -creditsPerImage,
				"balance_after":     newBalance,
				"description":       "Organization Dream Mode Image",
				"api_provider":      "flux-schnell",
				"production_idx":    productionID,
			}

			_, _, err := s.supabase.From("quel_credits").
				Insert(transactionData, false, "", "", "").
				Execute()

			if err != nil {
				log.Printf("⚠️ [FluxSchnell] Failed to record organization transaction: %v", err)
			}
		}

		log.Printf("✅ [FluxSchnell] Organization credits deducted: %d credits from org %s (used by %s)", totalCredits, *orgID, userID)
	} else {
		// 개인 크레딧 차감
		var members []struct {
			QuelMemberCredit int `json:"quel_member_credit"`
		}

		data, _, err := s.supabase.From("quel_member").
			Select("quel_member_credit", "", false).
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to fetch user credits: %w", err)
		}

		if err := json.Unmarshal(data, &members); err != nil {
			return fmt.Errorf("failed to parse member data: %w", err)
		}

		if len(members) == 0 {
			return fmt.Errorf("user not found: %s", userID)
		}

		currentCredits = members[0].QuelMemberCredit
		newBalance = currentCredits - totalCredits

		log.Printf("💰 [FluxSchnell] Personal credit balance: %d → %d (-%d)", currentCredits, newBalance, totalCredits)

		// 개인 크레딧 차감
		_, _, err = s.supabase.From("quel_member").
			Update(map[string]interface{}{
				"quel_member_credit": newBalance,
			}, "", "").
			Eq("quel_member_id", userID).
			Execute()

		if err != nil {
			return fmt.Errorf("failed to deduct credits: %w", err)
		}

		// 트랜잭션 기록 - 개인 크레딧
		for i := 0; i < imageCount; i++ {
			transactionData := map[string]interface{}{
				"user_id":          userID,
				"transaction_type": "DEDUCT",
				"amount":           -creditsPerImage,
				"balance_after":    newBalance,
				"description":      "Dream Mode Image",
				"api_provider":     "flux-schnell",
				"production_idx":   productionID,
			}

			_, _, err := s.supabase.From("quel_credits").
				Insert(transactionData, false, "", "", "").
				Execute()

			if err != nil {
				log.Printf("⚠️ [FluxSchnell] Failed to record transaction: %v", err)
			}
		}

		log.Printf("✅ [FluxSchnell] Personal credits deducted: %d credits from user %s", totalCredits, userID)
	}

	return nil
}

// UpdateProductionStatus - quel_production_photo 상태 업데이트
func (s *Service) UpdateProductionStatus(ctx context.Context, productionID string, status string) error {
	if productionID == "" {
		return nil
	}

	_, _, err := s.supabase.From("quel_production_photo").
		Update(map[string]interface{}{
			"production_status": status,
		}, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update production status: %w", err)
	}

	log.Printf("📋 [FluxSchnell] Production %s status updated to: %s", productionID, status)
	return nil
}

// UpdateProductionImageComplete - 이미지 생성 완료 시 attach_ids에 attach_idx 추가 및 generated_image_count 증가
func (s *Service) UpdateProductionImageComplete(ctx context.Context, productionID string, attachIdx int) error {
	if productionID == "" {
		return nil
	}

	// 현재 production 데이터 조회
	var productions []struct {
		AttachIDs           []int `json:"attach_ids"`
		GeneratedImageCount int   `json:"generated_image_count"`
		TotalQuantity       int   `json:"total_quantity"`
	}

	data, _, err := s.supabase.From("quel_production_photo").
		Select("attach_ids, generated_image_count, total_quantity", "", false).
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to fetch production: %w", err)
	}

	if err := json.Unmarshal(data, &productions); err != nil {
		return fmt.Errorf("failed to parse production data: %w", err)
	}

	if len(productions) == 0 {
		return fmt.Errorf("production not found: %s", productionID)
	}

	// attach_ids 배열에 attach_idx 추가
	currentAttachIDs := productions[0].AttachIDs
	if currentAttachIDs == nil {
		currentAttachIDs = []int{}
	}
	currentAttachIDs = append(currentAttachIDs, attachIdx)

	// attach_ids 배열 길이로 완료 체크
	newCount := len(currentAttachIDs)
	totalQuantity := productions[0].TotalQuantity

	// 모든 이미지 생성 완료 체크
	isCompleted := newCount >= totalQuantity

	// 업데이트 데이터 구성
	updateData := map[string]any{
		"attach_ids":            currentAttachIDs,
		"generated_image_count": newCount,
	}

	// 완료 시 상태 변경
	if isCompleted {
		updateData["production_status"] = "completed"
		log.Printf("🎉 [FluxSchnell] Production %s: ALL %d images completed!", productionID, totalQuantity)
	}

	// 업데이트
	_, _, err = s.supabase.From("quel_production_photo").
		Update(updateData, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to update production attach_ids: %w", err)
	}

	log.Printf("📷 [FluxSchnell] Production %s: attach_idx=%d added (%d/%d images)", productionID, attachIdx, newCount, totalQuantity)
	return nil
}

// CompleteProduction - 모든 이미지 생성 완료 처리
func (s *Service) CompleteProduction(ctx context.Context, productionID string, durationSeconds int) error {
	if productionID == "" {
		return nil
	}

	_, _, err := s.supabase.From("quel_production_photo").
		Update(map[string]interface{}{
			"production_status":            "completed",
			"processing_duration_seconds": durationSeconds,
		}, "", "").
		Eq("production_id", productionID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to complete production: %w", err)
	}

	log.Printf("✅ [FluxSchnell] Production %s completed in %d seconds", productionID, durationSeconds)
	return nil
}
