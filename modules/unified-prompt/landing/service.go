package landing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/supabase-community/supabase-go"
	"cloud.google.com/go/vertexai/genai"
	vertexai "quel-canvas-server/modules/common/vertexai"

	"quel-canvas-server/modules/common/config"
	redisutil "quel-canvas-server/modules/common/redis"
	"quel-canvas-server/modules/unified-prompt/common"
)

type Service struct {
	supabase    *supabase.Client
	genaiClient *genai.Client
	redis       *redis.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Supabase 클라이언트 초기화
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ [Landing] Failed to create Supabase client: %v", err)
		return nil
	}

	// Genai 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := vertexai.NewVertexAIClient(ctx, cfg.VertexAIProject, cfg.VertexAILocation)
	if err != nil {
		log.Printf("❌ [Landing] Failed to create Genai client: %v", err)
		return nil
	}

	// Redis 클라이언트 초기화
	redisClient := redisutil.Connect(cfg)
	if redisClient == nil {
		log.Printf("⚠️ [Landing] Failed to connect to Redis - guest limit feature will be disabled")
	}

	log.Println("✅ [Landing] Service initialized")
	return &Service{
		supabase:    supabaseClient,
		genaiClient: genaiClient,
		redis:       redisClient,
	}
}

// CheckGuestLimit - 비회원 사용 제한 확인
func (s *Service) CheckGuestLimit(ctx context.Context, sessionID string) (*GuestUsage, bool, error) {
	if s.redis == nil {
		// Redis 없으면 제한 없음 (개발 환경)
		return &GuestUsage{SessionID: sessionID, UsedCount: 0}, false, nil
	}

	key := fmt.Sprintf("guest:usage:%s", sessionID)

	// Redis에서 사용 기록 조회
	data, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		// 첫 사용
		return &GuestUsage{
			SessionID:   sessionID,
			UsedCount:   0,
			FirstUsedAt: time.Now(),
			LastUsedAt:  time.Now(),
		}, false, nil
	}
	if err != nil {
		log.Printf("⚠️ [Landing] Redis error: %v", err)
		return nil, false, err
	}

	var usage GuestUsage
	if err := json.Unmarshal([]byte(data), &usage); err != nil {
		log.Printf("⚠️ [Landing] Failed to parse guest usage: %v", err)
		return nil, false, err
	}

	// 제한 확인
	limitReached := usage.UsedCount >= common.MaxGuestGenerations

	return &usage, limitReached, nil
}

// IncrementGuestUsage - 비회원 사용 횟수 증가
func (s *Service) IncrementGuestUsage(ctx context.Context, sessionID string) (*GuestUsage, error) {
	if s.redis == nil {
		return &GuestUsage{SessionID: sessionID, UsedCount: 1}, nil
	}

	key := fmt.Sprintf("guest:usage:%s", sessionID)

	// 기존 기록 조회
	usage, _, err := s.CheckGuestLimit(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// 횟수 증가
	usage.UsedCount++
	usage.LastUsedAt = time.Now()
	if usage.FirstUsedAt.IsZero() {
		usage.FirstUsedAt = time.Now()
	}

	// Redis에 저장 (24시간 TTL)
	data, err := json.Marshal(usage)
	if err != nil {
		return nil, err
	}

	ttl := time.Duration(common.GuestLimitTTL) * time.Hour
	if err := s.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		log.Printf("⚠️ [Landing] Failed to save guest usage: %v", err)
		return nil, err
	}

	log.Printf("📊 [Landing] Guest usage updated: session=%s, count=%d/%d",
		sessionID, usage.UsedCount, common.MaxGuestGenerations)

	return usage, nil
}

// GenerateImage - 이미지 생성 (동기 방식 - 랜딩 데모용)
func (s *Service) GenerateImage(ctx context.Context, req *LandingGenerateRequest) (*LandingGenerateResponse, error) {
	cfg := config.GetConfig()

	// Aspect ratio 기본값
	aspectRatio := req.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "1:1"
	}

	log.Printf("🎨 [Landing] Generating image - prompt: %s, images: %d, ratio: %s",
		truncateString(req.Prompt, 50), len(req.ReferenceImages), aspectRatio)

	// Gemini API 호출 준비
	var parts []genai.Part

	// 레퍼런스 이미지 추가
	for i, imgBase64 := range req.ReferenceImages {
		// Base64 디코딩
		// data:image/xxx;base64, 접두사 제거
		base64Data := imgBase64
		if idx := findBase64Start(imgBase64); idx > 0 {
			base64Data = imgBase64[idx:]
			}

		imageData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			log.Printf("⚠️ [Landing] Failed to decode image %d: %v", i, err)
			continue
			}

		parts = append(parts, genai.ImageData("image/png", imageData))
		log.Printf("📎 [Landing] Added reference image %d (%d bytes)", i+1, len(imageData))
	}

	// 프롬프트 생성
	prompt := buildLandingPrompt(req.Prompt, len(req.ReferenceImages))
	parts = append(parts, genai.Text(prompt))

	// Parts are already prepared above

	// Gemini API 호출
	log.Printf("📤 [Landing] Calling Gemini API...")
	model := s.genaiClient.GenerativeModel(cfg.GeminiModel)
	model.SetTemperature(0.7)
	// Note: ResponseMIMEType should NOT be set for image generation with Gemini

	result, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		log.Printf("❌ [Landing] Gemini API error: %v", err)
		return &LandingGenerateResponse{
			Success:      false,
			ErrorMessage: "Image generation failed",
			ErrorCode:    common.ErrCodeInternalError,
		}, err
	}

	// 응답에서 이미지 추출
	if len(result.Candidates) == 0 {
		return &LandingGenerateResponse{
			Success:      false,
			ErrorMessage: "No image generated",
			ErrorCode:    common.ErrCodeInternalError,
		}, fmt.Errorf("no candidates in response")
	}

	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
			}

		for _, part := range candidate.Content.Parts {
			if blob, ok := part.(genai.Blob); ok && len(blob.Data) > 0 {
				imageBase64 := base64.StdEncoding.EncodeToString(blob.Data)
				log.Printf("✅ [Landing] Image generated: %d bytes", len(blob.Data))

				return &LandingGenerateResponse{
					Success:     true,
					JobID:       uuid.New().String(),
					ImageBase64: imageBase64,
				}, nil
			}
		}
	}

	return &LandingGenerateResponse{
		Success:      false,
		ErrorMessage: "No image data in response",
		ErrorCode:    common.ErrCodeInternalError,
	}, fmt.Errorf("no image data in response")
}

// buildLandingPrompt - 랜딩 페이지용 범용 프롬프트 생성
func buildLandingPrompt(userPrompt string, imageCount int) string {
	baseInstruction := `[CREATIVE IMAGE GENERATION]
You are a creative AI artist generating stunning, high-quality images.

INSTRUCTIONS:
- Generate ONE photorealistic, beautiful image based on the user's description
- If reference images are provided, use them as style/content inspiration
- Focus on visual appeal, composition, and artistic quality
- Create something that will impress and inspire the viewer

QUALITY REQUIREMENTS:
- High resolution, sharp details
- Professional lighting and composition
- Rich colors and contrast
- Cohesive artistic vision

`

	if imageCount > 0 {
		baseInstruction += fmt.Sprintf(`
REFERENCE IMAGES: %d image(s) provided
- Use these as inspiration for style, mood, or content
- Blend elements creatively while maintaining quality
- The final image should feel cohesive and intentional

`, imageCount)
	}

	return baseInstruction + "USER REQUEST:\n" + userPrompt
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func findBase64Start(s string) int {
	// "data:image/xxx;base64," 패턴 찾기
	marker := ";base64,"
	idx := 0
	for i := 0; i < len(s)-len(marker); i++ {
		if s[i:i+len(marker)] == marker {
			return i + len(marker)
			}
	}
	return idx
}

func floatPtr(f float64) *float32 {
	f32 := float32(f)
	return &f32
}
