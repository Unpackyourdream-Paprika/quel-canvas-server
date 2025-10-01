package generateimage

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config 구조체 - 모든 환경변수를 담음
type Config struct {
	// Redis
	RedisHost     string
	RedisPort     string
	RedisUsername string
	RedisPassword string
	RedisUseTLS   bool

	// Supabase
	SupabaseURL            string
	SupabaseServiceKey     string
	SupabaseStorageBaseURL string

	// Gemini API
	GeminiAPIKey string
	GeminiModel  string

	// Server
	Port string

	// Credit
	ImagePerPrice int
}

var config *Config

// LoadConfig - 환경변수 로드
func LoadConfig() (*Config, error) {
	// .env 파일 로드 (있으면)
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env file not found, using environment variables")
	}

	// Redis UseTLS 파싱
	useTLS := true // 기본값
	if tlsStr := os.Getenv("REDIS_USE_TLS"); tlsStr != "" {
		if parsed, err := strconv.ParseBool(tlsStr); err == nil {
			useTLS = parsed
		}
	}

	// ImagePerPrice 파싱
	imagePerPrice := 20 // 기본값
	if priceStr := os.Getenv("IMAGE_PER_PRICE"); priceStr != "" {
		if parsed, err := strconv.Atoi(priceStr); err == nil {
			imagePerPrice = parsed
		}
	}

	config = &Config{
		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisUsername: getEnv("REDIS_USERNAME", ""),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisUseTLS:   useTLS,

		// Supabase
		SupabaseURL:            getEnv("SUPABASE_URL", ""),
		SupabaseServiceKey:     getEnv("SUPABASE_SERVICE_KEY", ""),
		SupabaseStorageBaseURL: getEnv("SUPABASE_STORAGE_BASE_URL", ""),

		// Gemini API
		GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-2.5-flash-image-preview"),

		// Server
		Port: getEnv("PORT", "8080"),

		// Credit
		ImagePerPrice: imagePerPrice,
	}

	// 필수 환경변수 검증
	if err := config.validate(); err != nil {
		return nil, err
	}

	log.Println("✅ Configuration loaded successfully")
	log.Printf("   Redis: %s:%s (TLS: %v)", config.RedisHost, config.RedisPort, config.RedisUseTLS)
	log.Printf("   Supabase: %s", config.SupabaseURL)
	log.Printf("   Gemini: %s", config.GeminiModel)
	log.Printf("   Credit: %d per image", config.ImagePerPrice)

	return config, nil
}

// GetConfig - 로드된 설정 가져오기
func GetConfig() *Config {
	if config == nil {
		log.Fatal("❌ Config not loaded. Call LoadConfig() first.")
	}
	return config
}

// validate - 필수 환경변수 검증
func (c *Config) validate() error {
	if c.RedisHost == "" {
		return fmt.Errorf("REDIS_HOST is required")
	}
	if c.SupabaseURL == "" {
		return fmt.Errorf("SUPABASE_URL is required")
	}
	if c.SupabaseServiceKey == "" {
		return fmt.Errorf("SUPABASE_SERVICE_KEY is required")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("GEMINI_API_KEY is required")
	}
	return nil
}

// getEnv - 환경변수 가져오기 (기본값 지원)
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Redis 연결 문자열 생성
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}
