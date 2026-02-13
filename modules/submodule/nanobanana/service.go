package nanobanana

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	"google.golang.org/genai"

	"quel-canvas-server/modules/common/config"
	geminiretry "quel-canvas-server/modules/common/gemini"
)

type Service struct {
}

func NewService() *Service {
	log.Println("✅ [Nanobanana] Service initialized")
	return &Service{}
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

	result, err := geminiretry.GenerateContentWithRetry(
		ctx,
		cfg.GeminiAPIKeys,
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

// Analyze - 이미지 요소 분석 (Gemini Vision)
func (s *Service) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
	cfg := config.GetConfig()

	if req.Image.Data == "" {
		return &AnalyzeResponse{
			Success:      false,
			ErrorMessage: "Image data is required",
		}, nil
	}

	// base64 디코딩
	imageData, err := base64.StdEncoding.DecodeString(req.Image.Data)
	if err != nil {
		return &AnalyzeResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to decode image: %v", err),
		}, nil
	}

	mimeType := req.Image.MimeType
	if mimeType == "" {
		mimeType = "image/png"
	}

	log.Printf("🔍 [Nanobanana] Analyzing image: %d bytes, %s", len(imageData), mimeType)

	// 분석 프롬프트 구성
	analysisPrompt := `Analyze this image and extract the following elements in JSON format.

IMPORTANT: Each element's "prompt" must describe ONLY that specific element in ISOLATION, as if it were a standalone asset. The prompt should:
- Include the art style (anime, realistic, etc.) to maintain visual consistency
- For background: describe the scene/environment WITHOUT any characters or objects
- For items/characters: describe the element on a TRANSPARENT or SIMPLE SOLID background, completely isolated from the scene
- Include lighting, color tone, and mood that matches the original image's atmosphere

Elements to extract:
1. tone_mood: The overall tone, mood, and atmosphere
2. background: The background scene/environment ONLY (no characters, no foreground objects)
3. items: List of distinct objects/characters in the image (max 5 most prominent). Each item should be described as an ISOLATED asset.
4. style: The artistic/visual style
5. color_palette: Main colors used (3-5 hex codes)

For each element, provide:
- name: Short descriptive name
- description: Detailed description
- keywords: Related keywords for searching
- prompt: A detailed prompt to recreate this ISOLATED element with matching art style and tone

Example prompts:
- Background: "dreamy night sky with full moon, stars and sparkles, soft purple and blue gradient, anime style illustration, peaceful atmosphere, no characters"
- Item (character): "anime girl with long turquoise twintails, wearing white sleeveless shirt and turquoise tie, headphones, isolated character on transparent background, anime illustration style"
- Item (object): "dark headphones with pink accents, isolated object on white background, anime style illustration"

Respond ONLY with valid JSON in this exact format:
{
  "tone_mood": {"name": "...", "description": "...", "keywords": ["..."], "prompt": "..."},
  "background": {"name": "...", "description": "...", "keywords": ["..."], "prompt": "..."},
  "items": [{"type": "character|object|effect", "name": "...", "description": "...", "keywords": ["..."], "prompt": "..."}, ...],
  "style": {"name": "...", "description": "...", "keywords": ["..."], "prompt": "..."},
  "color_palette": ["#hex1", "#hex2", ...]
}`

	// Gemini API 호출 - Vision 모델 사용
	model := cfg.GeminiModel
	if model == "" {
		model = "gemini-2.0-flash"
	}

	parts := []*genai.Part{
		genai.NewPartFromText(analysisPrompt),
		genai.NewPartFromBytes(imageData, mimeType),
	}

	content := &genai.Content{
		Parts: parts,
	}

	result, err := geminiretry.GenerateContentWithRetry(
		ctx,
		cfg.GeminiAPIKeys,
		model,
		[]*genai.Content{content},
		&genai.GenerateContentConfig{
			Temperature: floatPtr(0.3), // 더 일관된 분석을 위해 낮은 온도
		},
	)
	if err != nil {
		log.Printf("❌ [Nanobanana] Gemini API error: %v", err)
		return &AnalyzeResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Gemini API error: %v", err),
		}, nil
	}

	// 응답에서 텍스트 추출
	var responseText string
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				responseText += part.Text
			}
		}
	}

	if responseText == "" {
		return &AnalyzeResponse{
			Success:      false,
			ErrorMessage: "No analysis result from Gemini",
		}, nil
	}

	log.Printf("📝 [Nanobanana] Raw response: %s", truncateString(responseText, 200))

	// JSON 파싱
	response, err := parseAnalysisResponse(responseText)
	if err != nil {
		log.Printf("⚠️ [Nanobanana] Failed to parse response: %v", err)
		return &AnalyzeResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse analysis: %v", err),
		}, nil
	}

	response.Success = true
	log.Printf("✅ [Nanobanana] Analysis complete: %d items found", len(response.Items))

	return response, nil
}

// parseAnalysisResponse - JSON 응답 파싱
func parseAnalysisResponse(text string) (*AnalyzeResponse, error) {
	// JSON 블록 추출 (```json ... ``` 형태 처리)
	jsonStr := text
	if idx := findJSONStart(text); idx >= 0 {
		jsonStr = text[idx:]
		if endIdx := findJSONEnd(jsonStr); endIdx > 0 {
			jsonStr = jsonStr[:endIdx+1]
		}
	}

	var raw struct {
		ToneMood struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
			Prompt      string   `json:"prompt"`
		} `json:"tone_mood"`
		Background struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
			Prompt      string   `json:"prompt"`
		} `json:"background"`
		Items []struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
			Prompt      string   `json:"prompt"`
		} `json:"items"`
		Style struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
			Prompt      string   `json:"prompt"`
		} `json:"style"`
		ColorPalette []string `json:"color_palette"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	response := &AnalyzeResponse{
		ColorPalette: raw.ColorPalette,
	}

	// ToneMood
	if raw.ToneMood.Name != "" {
		response.ToneMood = &AnalyzedElement{
			Type:        "tone_mood",
			Name:        raw.ToneMood.Name,
			Description: raw.ToneMood.Description,
			Keywords:    raw.ToneMood.Keywords,
			Prompt:      raw.ToneMood.Prompt,
		}
	}

	// Background
	if raw.Background.Name != "" {
		response.Background = &AnalyzedElement{
			Type:        "background",
			Name:        raw.Background.Name,
			Description: raw.Background.Description,
			Keywords:    raw.Background.Keywords,
			Prompt:      raw.Background.Prompt,
		}
	}

	// Style
	if raw.Style.Name != "" {
		response.Style = &AnalyzedElement{
			Type:        "style",
			Name:        raw.Style.Name,
			Description: raw.Style.Description,
			Keywords:    raw.Style.Keywords,
			Prompt:      raw.Style.Prompt,
		}
	}

	// Items
	for _, item := range raw.Items {
		if item.Name != "" {
			response.Items = append(response.Items, AnalyzedElement{
				Type:        "item",
				Name:        item.Name,
				Description: item.Description,
				Keywords:    item.Keywords,
				Prompt:      item.Prompt,
			})
		}
	}

	return response, nil
}

// findJSONStart - JSON 시작 위치 찾기
func findJSONStart(s string) int {
	for i, c := range s {
		if c == '{' {
			return i
		}
	}
	return -1
}

// findJSONEnd - JSON 끝 위치 찾기 (중첩 괄호 처리)
func findJSONEnd(s string) int {
	depth := 0
	for i, c := range s {
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
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
