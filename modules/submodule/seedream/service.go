package seedream

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

// Seedream 3.0 모델 ID (Runware - ByteDance)
const SeedreamModelID = "bytedance:seedream-3.0"

type Service struct {
	httpClient *http.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		log.Println("⚠️ [Seedream] RUNWARE_API_KEY not configured")
		return nil
	}

	log.Println("✅ [Seedream] Service initialized")
	return &Service{
		httpClient: &http.Client{Timeout: 180 * time.Second}, // Seedream은 좀 더 긴 타임아웃
	}
}

// Generate - Seedream 3.0으로 이미지 생성
func (s *Service) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	cfg := config.GetConfig()

	if cfg.RunwareAPIKey == "" {
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: "RUNWARE_API_KEY not configured",
		}, nil
	}

	// Seedream은 고해상도 지원 (2048x2048)
	width, height := s.calculateDimensions(req.AspectRatio, req.Width, req.Height)

	log.Printf("🎨 [Seedream] Generating image - size: %dx%d, aspectRatio: %s, prompt: %s",
		width, height, req.AspectRatio, truncateString(req.Prompt, 50))

	// Runware 요청 구성 (Seedream 전용)
	runwareReq := RunwareRequest{
		TaskType:       "imageInference",
		TaskUUID:       generateUUID(),
		PositivePrompt: req.Prompt,
		Model:          SeedreamModelID,
		Width:          width,
		Height:         height,
		NumberResults:  1,
		OutputFormat:   "PNG",
	}

	// Seedream은 negativePrompt를 사용하지 않음 (무시됨)
	// steps, cfgScale도 사용하지 않음

	// 참조 이미지가 있으면 referenceImages로 추가
	if len(req.Images) > 0 && req.Images[0].Data != "" {
		dataURL := "data:" + req.Images[0].MimeType + ";base64," + req.Images[0].Data
		runwareReq.ReferenceImages = []string{dataURL}
		log.Printf("📷 [Seedream] Adding reference image")
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
		log.Printf("❌ [Seedream] Runware API error: %v", err)
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
		log.Printf("❌ [Seedream] Runware API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
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
			log.Printf("⚠️ [Seedream] Failed to download image, returning URL: %v", err)
			return &GenerateResponse{
				Success:  true,
				ImageURL: imageURL,
			}, nil
		}

		log.Printf("✅ [Seedream] Image generated successfully (2K resolution)")
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

// GenerateWithBytes - 바이트 배열로 이미지 생성 (landing-demo 호환용)
func (s *Service) GenerateWithBytes(ctx context.Context, prompt string, aspectRatio string, inputImageBase64 string) ([]byte, error) {
	req := &GenerateRequest{
		Prompt:      prompt,
		AspectRatio: aspectRatio,
	}

	if inputImageBase64 != "" {
		req.Images = []InputImage{{
			Data:     inputImageBase64,
			MimeType: "image/png",
		}}
	}

	resp, err := s.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf(resp.ErrorMessage)
	}

	if resp.ImageBase64 != "" {
		return base64.StdEncoding.DecodeString(resp.ImageBase64)
	}

	// URL에서 다운로드
	if resp.ImageURL != "" {
		return s.downloadImage(ctx, resp.ImageURL)
	}

	return nil, fmt.Errorf("no image data available")
}

// calculateDimensions - 해상도 계산 (Seedream은 2048 기준)
func (s *Service) calculateDimensions(aspectRatio string, reqWidth, reqHeight int) (int, int) {
	// 요청에 명시적인 값이 있으면 사용
	if reqWidth > 0 && reqHeight > 0 {
		return reqWidth, reqHeight
	}

	// Seedream 기본 해상도 (2048 기준)
	switch aspectRatio {
	case "16:9":
		return 2048, 1152
	case "9:16":
		return 1152, 2048
	case "4:5":
		return 1638, 2048
	case "3:4":
		return 1536, 2048
	case "4:3":
		return 2048, 1536
	default: // 1:1 또는 미지정
		return 2048, 2048
	}
}

// downloadImageAsBase64 - URL에서 이미지 다운로드 후 base64 반환
func (s *Service) downloadImageAsBase64(ctx context.Context, imageURL string) (string, error) {
	imageData, err := s.downloadImage(ctx, imageURL)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(imageData), nil
}

// downloadImage - URL에서 이미지 다운로드
func (s *Service) downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
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

// IsSeedreamModel - Seedream 모델 여부 확인
func IsSeedreamModel(modelID string) bool {
	if modelID == "" {
		return false
	}
	return modelID == SeedreamModelID ||
		modelID == "seedream" ||
		modelID == "bytedance:seedream-3.0" ||
		modelID == "seedream-3.0"
}
