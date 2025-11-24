package veo3

import (
	"log"
	"os"
)

type Config struct {
	Veo3APIKey      string
	Veo3APIEndpoint string
	RedisURL        string
	SupabaseURL     string
	SupabaseKey     string
}

func LoadConfig() *Config {
	apiKey := os.Getenv("VEO3_API_KEY")
	if apiKey == "" {
		log.Println("Warning: VEO3_API_KEY not set")
	}

	endpoint := os.Getenv("VEO3_API_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.veo3.ai/v1/generate" // Default endpoint
	}

	return &Config{
		Veo3APIKey:      apiKey,
		Veo3APIEndpoint: endpoint,
		RedisURL:        os.Getenv("REDIS_URL"),
		SupabaseURL:     os.Getenv("SUPABASE_URL"),
		SupabaseKey:     os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
	}
}
