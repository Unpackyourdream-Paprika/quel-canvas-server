package klingmigration

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Service - Kling AI API 서비스
type Service struct {
	config     *Config
	httpClient *http.Client
}

// NewService - Service 생성
func NewService() *Service {
	cfg := GetConfig()
	if cfg == nil {
		log.Println("❌ [Kling] Failed to load config")
		return nil
	}

	return &Service{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// generateJWT - Kling AI JWT 토큰 생성
func (s *Service) generateJWT() (string, error) {
	// Header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerBytes, _ := json.Marshal(header)
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerBytes)

	// Payload
	now := time.Now().Unix()
	payload := map[string]interface{}{
		"iss": s.config.AccessKey,
		"exp": now + 1800, // 30분 유효
		"nbf": now - 5,
	}
	payloadBytes, _ := json.Marshal(payload)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// Signature
	signatureInput := headerEncoded + "." + payloadEncoded
	h := hmac.New(sha256.New, []byte(s.config.SecretKey))
	h.Write([]byte(signatureInput))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return signatureInput + "." + signature, nil
}

// CreateImageToVideoTask - 이미지에서 비디오 생성 작업 시작
func (s *Service) CreateImageToVideoTask(imageBase64, prompt string) (string, error) {
	jwt, err := s.generateJWT()
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Kling API 요청 데이터
	reqData := KlingCreateTaskRequest{
		Model:    "kling-v1",
		TaskType: "image2video",
		Input: KlingInput{
			ImageBase64: imageBase64,
			Prompt:      prompt,
			Duration:    5, // 기본 5초
		},
	}

	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", s.config.APIURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	log.Printf("🚀 [Kling] Creating image2video task...")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("📥 [Kling] Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result KlingCreateTaskResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("API error code %d: %s", result.Code, result.Message)
	}

	log.Printf("✅ [Kling] Task created: %s", result.Data.TaskID)
	return result.Data.TaskID, nil
}

// GetTaskStatus - 작업 상태 조회
func (s *Service) GetTaskStatus(taskID string) (*KlingTaskStatusResponse, error) {
	jwt, err := s.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	// 상태 조회 URL
	statusURL := fmt.Sprintf("https://api.klingai.com/v1/videos/image2video/%s", taskID)

	req, err := http.NewRequest("GET", statusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result KlingTaskStatusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// WaitForCompletion - 작업 완료 대기 (폴링)
func (s *Service) WaitForCompletion(taskID string, maxAttempts int) (*KlingTaskStatusResponse, error) {
	log.Printf("⏳ [Kling] Waiting for task %s to complete...", taskID)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		status, err := s.GetTaskStatus(taskID)
		if err != nil {
			log.Printf("⚠️ [Kling] Attempt %d: Failed to get status: %v", attempt, err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("📊 [Kling] Attempt %d: Status = %s", attempt, status.Data.TaskStatus)

		switch status.Data.TaskStatus {
		case "succeed":
			log.Printf("✅ [Kling] Task %s completed successfully", taskID)
			return status, nil
		case "failed":
			return status, fmt.Errorf("task failed: %s", status.Message)
		case "submitted", "processing":
			// 계속 대기
			time.Sleep(5 * time.Second)
		default:
			log.Printf("⚠️ [Kling] Unknown status: %s", status.Data.TaskStatus)
			time.Sleep(5 * time.Second)
		}
	}

	return nil, fmt.Errorf("timeout waiting for task completion after %d attempts", maxAttempts)
}
