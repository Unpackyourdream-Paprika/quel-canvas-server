package config

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

var globalConfig *Config

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
	imagePerPrice := 5 // 기본값 (5 크레딧 = ₩500/장)
	if priceStr := os.Getenv("IMAGE_PER_PRICE"); priceStr != "" {
		if parsed, err := strconv.Atoi(priceStr); err == nil {
			imagePerPrice = parsed
		}
	}

	globalConfig = &Config{
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
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-2.5-flash-image"),

		// Server
		Port: getEnv("PORT", "8080"),

		// Credit
		ImagePerPrice: imagePerPrice,
	}

	// 필수 환경변수 검증
	if err := globalConfig.validate(); err != nil {
		return nil, err
	}

	log.Println("✅ Configuration loaded successfully")
	log.Printf("   Redis: %s:%s (TLS: %v)", globalConfig.RedisHost, globalConfig.RedisPort, globalConfig.RedisUseTLS)
	log.Printf("   Supabase: %s", globalConfig.SupabaseURL)
	log.Printf("   Gemini: %s", globalConfig.GeminiModel)
	log.Printf("   Credit: %d per image", globalConfig.ImagePerPrice)

	return globalConfig, nil
}

// GetConfig - 로드된 설정 가져오기
func GetConfig() *Config {
	if globalConfig == nil {
		log.Fatal("❌ Config not loaded. Call LoadConfig() first.")
	}
	return globalConfig
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

// GetRedisAddr - Redis 연결 문자열 생성
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}
