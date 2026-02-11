package vertexai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/api/option"
)

// NewVertexAIClient - Vertex AI 클라이언트 생성 (환경 변수 자동 처리)
func NewVertexAIClient(ctx context.Context, project, location string) (*genai.Client, error) {
	var opts []option.ClientOption

	// 1. 환경 변수 VERTEXAI_CREDENTIALS_JSON 확인 (Render 배포용)
	if credsJSON := os.Getenv("VERTEXAI_CREDENTIALS_JSON"); credsJSON != "" {
		log.Println("✅ [VertexAI] Using VERTEXAI_CREDENTIALS_JSON from environment")
		opts = append(opts, option.WithCredentialsJSON([]byte(credsJSON)))
	} else if credsPath := os.Getenv("VERTEXAI_CREDENTIALS_PATH"); credsPath != "" {
		// 2. 환경 변수 VERTEXAI_CREDENTIALS_PATH 확인 (로컬 테스트용)
		log.Printf("✅ [VertexAI] Using credentials from file: %s\n", credsPath)
		credsData, err := os.ReadFile(credsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read credentials file: %w", err)
		}
		// JSON 유효성 검사
		var creds map[string]interface{}
		if err := json.Unmarshal(credsData, &creds); err != nil {
			return nil, fmt.Errorf("invalid JSON credentials: %w", err)
		}
		opts = append(opts, option.WithCredentialsJSON(credsData))
	} else {
		// 3. Application Default Credentials (ADC) 사용
		log.Println("⚠️  [VertexAI] No explicit credentials found, using Application Default Credentials")
	}

	// Vertex AI 클라이언트 생성
	client, err := genai.NewClient(ctx, project, location, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	log.Printf("✅ [VertexAI] Client initialized for project=%s, location=%s\n", project, location)
	return client, nil
}
