package klingmigration

import (
	"log"
	"os"
)

// Config - Kling AI API 설정
type Config struct {
	AccessKey  string
	SecretKey  string
	APIURL     string
	ImagePrice int
}

var klingConfig *Config

// LoadConfig - 환경변수에서 설정 로드
func LoadConfig() *Config {
	if klingConfig != nil {
		return klingConfig
	}

	accessKey := os.Getenv("KLING_AI_ACCESS_KEY")
	secretKey := os.Getenv("KLING_AI_SECRET_KEY")
	apiURL := os.Getenv("KLING_AI_API_URL")

	if accessKey == "" || secretKey == "" {
		log.Println("⚠️ [Kling] KLING_AI_ACCESS_KEY or KLING_AI_SECRET_KEY not set")
		return nil
	}

	if apiURL == "" {
		apiURL = "https://api.klingai.com/v1/videos/image2video"
	}

	// IMAGE_PER_PRICE 환경변수 (기본값 20)
	imagePrice := 20
	if priceStr := os.Getenv("IMAGE_PER_PRICE"); priceStr != "" {
		// 간단한 파싱 (strconv 없이)
		price := 0
		for _, c := range priceStr {
			if c >= '0' && c <= '9' {
				price = price*10 + int(c-'0')
			}
		}
		if price > 0 {
			imagePrice = price
		}
	}

	klingConfig = &Config{
		AccessKey:  accessKey,
		SecretKey:  secretKey,
		APIURL:     apiURL,
		ImagePrice: imagePrice,
	}

	log.Printf("✅ [Kling] Config loaded - API URL: %s, Price: %d credits", apiURL, imagePrice)
	return klingConfig
}

// GetConfig - 설정 반환
func GetConfig() *Config {
	if klingConfig == nil {
		return LoadConfig()
	}
	return klingConfig
}
