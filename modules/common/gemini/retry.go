package gemini

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"
)

// GenerateContentWithRetry - 단일 API 키로 최대 10번 재시도하는 헬퍼 함수
// 429 에러 시 3초 대기 후 재시도, 10번 실패 시 차단(에러 반환)
// apiKey: 사용할 단일 API 키
// model: Gemini 모델명 (예: "gemini-2.5-flash-image")
// contents: 생성 요청 컨텐츠
// config: 생성 설정
func GenerateContentWithRetry(
	ctx context.Context,
	apiKey string,
	model string,
	contents []*genai.Content,
	config *genai.GenerateContentConfig,
) (*genai.GenerateContentResponse, error) {

	if apiKey == "" {
		return nil, fmt.Errorf("no API key provided")
	}

	const maxRetries = 10
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("   🔄 [Gemini Retry] Attempt %d/%d", attempt, maxRetries)
		} else {
			log.Printf("🔑 [Gemini Retry] Calling Gemini API (attempt %d/%d)", attempt, maxRetries)
		}

		// 클라이언트 생성
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})

		if err != nil {
			log.Printf("⚠️  [Gemini Retry] Failed to create client (attempt %d): %v", attempt, err)
			lastErr = err
			time.Sleep(3 * time.Second)
			continue
		}

		// API 호출
		result, err := client.Models.GenerateContent(ctx, model, contents, config)

		if err == nil {
			log.Printf("✅ [Gemini Retry] Success on attempt %d/%d", attempt, maxRetries)
			return result, nil
		}

		lastErr = err

		// 429가 아닌 에러면 바로 반환 (재시도 안 함)
		if !is429Error(err) {
			log.Printf("❌ [Gemini Retry] Non-retryable error on attempt %d: %v", attempt, err)
			return nil, err
		}

		// 429 에러 - 3초 대기 후 재시도
		log.Printf("⚠️  [Gemini Retry] Rate limited (429) on attempt %d/%d", attempt, maxRetries)

		if attempt < maxRetries {
			log.Printf("   ⏳ Waiting 3 seconds before retry...")
			time.Sleep(3 * time.Second)
		}
	}

	// 10번 모두 실패 - 차단
	log.Printf("🚫 [Gemini Retry] BLOCKED - All %d attempts exhausted", maxRetries)
	return nil, fmt.Errorf("gemini API blocked after %d failed attempts, last error: %w", maxRetries, lastErr)
}

// is429Error - 429 Rate Limit 에러인지 확인
func is429Error(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(strings.ToLower(errStr), "rate limit") ||
		strings.Contains(strings.ToLower(errStr), "quota")
}
