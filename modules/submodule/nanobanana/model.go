package nanobanana

// InputImage - 입력 이미지 데이터
type InputImage struct {
	Data     string `json:"data"`      // base64 인코딩된 이미지 데이터
	MimeType string `json:"mime_type"` // image/jpeg, image/png 등
}

// GenerateRequest - 이미지 생성 요청
type GenerateRequest struct {
	Prompt string       `json:"prompt"`
	Model  string       `json:"model"`            // "2.5-flash-image", "2.5-pro-image"
	Width  int          `json:"width"`            // 기본 512
	Height int          `json:"height"`           // 기본 512
	Images []InputImage `json:"images,omitempty"` // 참조 이미지 (최대 2개)
}

// GenerateResponse - 이미지 생성 응답
type GenerateResponse struct {
	Success      bool   `json:"success"`
	ImageURL     string `json:"image_url,omitempty"`
	ImageBase64  string `json:"image_base64,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// AnalyzeRequest - 이미지 분석 요청
type AnalyzeRequest struct {
	Image    InputImage `json:"image"`              // 분석할 이미지
	Language string     `json:"language,omitempty"` // 응답 언어 (en, ko 등)
}

// AnalyzedElement - 분석된 개별 요소
type AnalyzedElement struct {
	Type        string   `json:"type"`                  // "tone_mood", "background", "item", "style"
	Name        string   `json:"name"`                  // 요소 이름
	Description string   `json:"description,omitempty"` // 상세 설명
	Keywords    []string `json:"keywords,omitempty"`    // 관련 키워드
	Prompt      string   `json:"prompt,omitempty"`      // 해당 요소 재생성용 프롬프트
}

// AnalyzeResponse - 이미지 분석 응답
type AnalyzeResponse struct {
	Success      bool              `json:"success"`
	ToneMood     *AnalyzedElement  `json:"tone_mood,omitempty"`  // 톤앤무드
	Background   *AnalyzedElement  `json:"background,omitempty"` // 배경
	Items        []AnalyzedElement `json:"items,omitempty"`      // 개별 아이템들
	Style        *AnalyzedElement  `json:"style,omitempty"`      // 스타일
	ColorPalette []string          `json:"color_palette,omitempty"` // 색상 팔레트
	ErrorMessage string            `json:"error_message,omitempty"`
}
