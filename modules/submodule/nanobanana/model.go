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
