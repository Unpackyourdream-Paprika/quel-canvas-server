package fluxschnell

// InputImage - 입력 이미지 데이터
type InputImage struct {
	Data     string `json:"data"`      // base64 인코딩된 이미지 데이터
	MimeType string `json:"mime_type"` // image/jpeg, image/png 등
}

// GenerateRequest - 이미지 생성 요청
type GenerateRequest struct {
	Prompt         string       `json:"prompt"`
	NegativePrompt string       `json:"negative_prompt,omitempty"`
	Width          int          `json:"width"`            // 기본 1024
	Height         int          `json:"height"`           // 기본 1024
	Steps          int          `json:"steps,omitempty"`  // 기본 4 (Schnell은 4 steps 권장)
	CFGScale       float64      `json:"cfg_scale,omitempty"` // 기본 1.0
	Images         []InputImage `json:"images,omitempty"` // 참조 이미지 (img2img)
	Strength       float64      `json:"strength,omitempty"` // img2img strength (0.0-1.0)
	UserID         string       `json:"user_id,omitempty"` // 크레딧 차감용 유저 ID
	ProductionID   string       `json:"production_id,omitempty"` // quel_production_photo 연동용
}

// GenerateResponse - 이미지 생성 응답
type GenerateResponse struct {
	Success      bool   `json:"success"`
	ImageURL     string `json:"image_url,omitempty"`
	ImageBase64  string `json:"image_base64,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// RunwareRequest - Runware API 요청 구조체
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
	Steps           int      `json:"steps,omitempty"`
	CFGScale        float64  `json:"CFGScale,omitempty"`
	InputImage      string   `json:"inputImage,omitempty"`
	Strength        float64  `json:"strength,omitempty"`
	ReferenceImages []string `json:"referenceImages,omitempty"`
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
