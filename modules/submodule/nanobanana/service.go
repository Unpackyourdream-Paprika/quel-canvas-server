package nanobanana

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
)

type Service struct {
	genaiClient *genai.Client
}

func NewService() *Service {
	cfg := config.GetConfig()

	// Genai 클라이언트 초기화
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("❌ [Nanobanana] Failed to create Genai client: %v", err)
		return nil
	}

	log.Println("✅ [Nanobanana] Service initialized")
	return &Service{
		genaiClient: genaiClient,
	}
}

// Generate - 단순 프롬프트 기반 이미지 생성
func (s *Service) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	cfg := config.GetConfig()

	// 기본값 설정
	width := req.Width
	if width <= 0 {
		width = 512
	}
	height := req.Height
	if height <= 0 {
		height = 512
	}

	// 모델 결정 (요청에서 지정하거나 기본값 사용)
	model := req.Model
	if model == "" {
		model = cfg.GeminiModel
	}

	// aspect ratio 계산
	aspectRatio := "1:1"
	if width > height {
		if float64(width)/float64(height) >= 1.7 {
			aspectRatio = "16:9"
		} else {
			aspectRatio = "4:3"
		}
	} else if height > width {
		if float64(height)/float64(width) >= 1.7 {
			aspectRatio = "9:16"
		} else {
			aspectRatio = "3:4"
		}
	}

	log.Printf("🎨 [Nanobanana] Generating image - model: %s, ratio: %s, images: %d, prompt: %s",
		model, aspectRatio, len(req.Images), truncateString(req.Prompt, 50))

	// Gemini API 호출 - Parts 구성
	parts := []*genai.Part{
		genai.NewPartFromText(req.Prompt),
	}

	// 입력 이미지가 있으면 추가 (최대 2개)
	for i, img := range req.Images {
		if i >= 2 {
			break // 최대 2개까지만
		}
		if img.Data == "" || img.MimeType == "" {
			continue
		}

		// base64 디코딩
		imageData, err := base64.StdEncoding.DecodeString(img.Data)
		if err != nil {
			log.Printf("⚠️ [Nanobanana] Failed to decode image %d: %v", i, err)
			continue
		}

		log.Printf("📷 [Nanobanana] Adding input image %d: %s, %d bytes", i+1, img.MimeType, len(imageData))
		parts = append(parts, genai.NewPartFromBytes(imageData, img.MimeType))
	}

	content := &genai.Content{
		Parts: parts,
	}

	result, err := s.genaiClient.Models.GenerateContent(
		ctx,
		model,
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			ImageConfig: &genai.ImageConfig{
				AspectRatio: aspectRatio,
			},
			Temperature: floatPtr(0.7),
		},
	)
	if err != nil {
		log.Printf("❌ [Nanobanana] Gemini API error: %v", err)
		return &GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Gemini API error: %v", err),
		}, nil
	}

	// 응답에서 이미지 추출
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				imageBase64 := base64.StdEncoding.EncodeToString(part.InlineData.Data)
				log.Printf("✅ [Nanobanana] Image generated: %d bytes", len(part.InlineData.Data))

				return &GenerateResponse{
					Success:     true,
					ImageBase64: imageBase64,
				}, nil
			}
		}
	}

	return &GenerateResponse{
		Success:      false,
		ErrorMessage: "No image generated from Gemini",
	}, nil
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func floatPtr(f float64) *float32 {
	f32 := float32(f)
	return &f32
}
