package gemini

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"
)

// GenerateContentWithRetry - 429 에러 시 여러 API 키로 재시도하는 헬퍼 함수
// apiKeys: 시도할 API 키 리스트
// model: Gemini 모델명 (예: "gemini-2.5-flash-image")
// contents: 생성 요청 컨텐츠
// config: 생성 설정
// 각 키당 최대 3번 재시도
func GenerateContentWithRetry(
	ctx context.Context,
	apiKeys []string,
	model string,
	contents []*genai.Content,
	config *genai.GenerateContentConfig,
) (*genai.GenerateContentResponse, error) {

	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("no API keys provided")
	}

	const maxRetriesPerKey = 3
	var lastErr error

	// 각 API 키로 시도
	for keyIndex, apiKey := range apiKeys {
		log.Printf("🔑 [Gemini Retry] Trying API key #%d/%d", keyIndex+1, len(apiKeys))

		// 각 키당 최대 3번 재시도
		for attempt := 1; attempt <= maxRetriesPerKey; attempt++ {
			if attempt > 1 {
				log.Printf("   🔄 Retry attempt %d/%d for key #%d", attempt, maxRetriesPerKey, keyIndex+1)
			}

			// 새 클라이언트 생성
			client, err := genai.NewClient(ctx, &genai.ClientConfig{
				APIKey:  apiKey,
				Backend: genai.BackendGeminiAPI,
			})

			if err != nil {
				log.Printf("⚠️  [Gemini Retry] Failed to create client with key #%d (attempt %d): %v", keyIndex+1, attempt, err)
				lastErr = err
				continue
			}

			// API 호출
			result, err := client.Models.GenerateContent(ctx, model, contents, config)

			if err == nil {
				// 성공!
				log.Printf("✅ [Gemini Retry] Success with API key #%d (attempt %d/%d)", keyIndex+1, attempt, maxRetriesPerKey)
				return result, nil
			}

			// 에러 체크
			lastErr = err

			// 429가 아닌 다른 에러면 바로 반환 (재시도 안 함)
			if !is429Error(err) {
				log.Printf("❌ [Gemini Retry] Key #%d failed with non-429 error: %v", keyIndex+1, err)
				return nil, err
			}

			// 429 에러 - 같은 키로 재시도 (최대 3번)
			log.Printf("⚠️  [Gemini Retry] Key #%d hit rate limit (429) on attempt %d/%d", keyIndex+1, attempt, maxRetriesPerKey)

			// 마지막 시도가 아니면 2초 대기 후 재시도
			if attempt < maxRetriesPerKey {
				log.Printf("   ⏳ Waiting 2 seconds before retry...")
				time.Sleep(time.Second * 2)
				continue
			}
		}

		// 이 키는 3번 모두 실패 - 다음 키로
		log.Printf("⚠️  [Gemini Retry] Key #%d exhausted all %d attempts, trying next key...", keyIndex+1, maxRetriesPerKey)
	}

	// 모든 키 실패
	return nil, fmt.Errorf("all %d API keys exhausted (3 attempts each), last error: %w", len(apiKeys), lastErr)
}

// is429Error - 429 Rate Limit 에러인지 확인
func is429Error(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Gemini API 429 에러 패턴 체크
	return strings.Contains(errStr, "429") ||
		strings.Contains(strings.ToLower(errStr), "rate limit") ||
		strings.Contains(strings.ToLower(errStr), "quota")
}
