package fluxschnell

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"quel-canvas-server/modules/common/config"
)

// Flux Schnell 모델 ID (Runware)
const FluxSchnellModelID = "runware:100@1"

type Service struct {
	httpClient *http.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		log.Println("⚠️ [FluxSchnell] RUNWARE_API_KEY not configured")
		return nil
	}

	log.Println("✅ [FluxSchnell] Service initialized")
	return &Service{
		httpClient: &http.Client{Timeout: 120 * time.Second},
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
