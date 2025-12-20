package seedream

// InputImage - 입력 이미지 데이터
type InputImage struct {
	Data     string `json:"data"`      // base64 인코딩된 이미지 데이터
	MimeType string `json:"mime_type"` // image/jpeg, image/png 등
}

// GenerateRequest - 이미지 생성 요청
type GenerateRequest struct {
	Prompt         string       `json:"prompt"`
	NegativePrompt string       `json:"negative_prompt,omitempty"`
	AspectRatio    string       `json:"aspect_ratio,omitempty"` // 1:1, 16:9, 9:16, 4:5
	Width          int          `json:"width,omitempty"`
	Height         int          `json:"height,omitempty"`
	Images         []InputImage `json:"images,omitempty"` // 참조 이미지
}

// GenerateResponse - 이미지 생성 응답
type GenerateResponse struct {
	Success      bool   `json:"success"`
	ImageURL     string `json:"image_url,omitempty"`
	ImageBase64  string `json:"image_base64,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// RunwareRequest - Runware API 요청 구조체 (Seedream용)
type RunwareRequest struct {
	TaskType        string   `json:"taskType"`
	TaskUUID        string   `json:"taskUUID"`
	PositivePrompt  string   `json:"positivePrompt"`
	NegativePrompt  string   `json:"negativePrompt,omitempty"`
	Model           string   `json:"model"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
	NumberResults   int      `json:"numberResults"`
	OutputFormat    string   `json:"outputFormat"`
	ReferenceImages []string `json:"referenceImages,omitempty"` // Seedream은 referenceImages 사용
}

// RunwareResponse - Runware API 응답 구조체
type RunwareResponse struct {
	Data []struct {
		TaskType  string `json:"taskType"`
		TaskUUID  string `json:"taskUUID"`
		ImageURL  string `json:"imageURL"`
		ImageUUID string `json:"imageUUID"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}
