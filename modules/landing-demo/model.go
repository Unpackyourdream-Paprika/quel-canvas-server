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
