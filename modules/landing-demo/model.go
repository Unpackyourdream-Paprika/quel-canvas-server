package landingdemo

// ImageWithCategory - 카테고리 정보가 포함된 이미지
type ImageWithCategory struct {
	Data     string `json:"data"`     // base64 이미지 데이터
	Category string `json:"category"` // model, top, pants, bg, shoes 등
}

// LandingDemoRequest - 랜딩 데모 요청 구조체
type LandingDemoRequest struct {
	Prompt      string              `json:"prompt"`
	AspectRatio string              `json:"aspectRatio"` // "1:1", "16:9", "9:16", "4:5"
	Quantity    int                 `json:"quantity"`    // 생성할 이미지 수 (최대 4)
	Images      []ImageWithCategory `json:"images"`      // 카테고리 포함 이미지 배열
}

// LandingDemoResponse - 랜딩 데모 응답 구조체
type LandingDemoResponse struct {
	Success      bool     `json:"success"`
	Images       []string `json:"images"`       // base64 이미지 배열
	ErrorMessage string   `json:"errorMessage,omitempty"`
}

// ImageCategories - 카테고리별 이미지 분류 (fashion 모듈과 동일)
type ImageCategories struct {
	Model       []byte   // 모델 이미지 (최대 1장)
	Clothing    [][]byte // 의류 이미지 배열 (top, pants, outer)
	Accessories [][]byte // 악세사리 이미지 배열 (shoes, bag, accessory)
	Background  []byte   // 배경 이미지 (최대 1장)
}

// RunwareRequest - Runware API 요청 구조체
type RunwareRequest struct {
	TaskType       string   `json:"taskType"`
	TaskUUID       string   `json:"taskUUID"`
	PositivePrompt string   `json:"positivePrompt"`
	Model          string   `json:"model"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	NumberResults  int      `json:"numberResults"`
	OutputFormat   string   `json:"outputFormat"`
	Steps          int      `json:"steps,omitempty"`
	CFGScale       float64  `json:"CFGScale,omitempty"`
	NegativePrompt string   `json:"negativePrompt,omitempty"`
	ReferenceImages []string `json:"referenceImages,omitempty"` // Seedream용
	InputImage     string   `json:"inputImage,omitempty"`       // 일반 RUNWARE용
	Strength       float64  `json:"strength,omitempty"`
}

// RunwareResponse - Runware API 응답 구조체
type RunwareResponse struct {
	Data []struct {
		ImageURL string `json:"imageURL"`
	} `json:"data"`
}

// OpenAIRequest - OpenAI API 요청 구조체
type OpenAIRequest struct {
	Model       string            `json:"model"`
	Messages    []OpenAIMessage   `json:"messages"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature"`
}

// OpenAIMessage - OpenAI 메시지 구조체
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse - OpenAI API 응답 구조체
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
