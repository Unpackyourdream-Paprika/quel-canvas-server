package redis

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"quel-canvas-server/modules/common/config"
)

// Connect - Redis 연결 생성
func Connect(cfg *config.Config) *redis.Client {
	log.Printf("🔌 Connecting to Redis: %s", cfg.GetRedisAddr())

	// TLS 설정 (InsecureSkipVerify 추가)
	var tlsConfig *tls.Config
	if cfg.RedisUseTLS {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, // Render.com Redis용
		}
	}

	// Redis 클라이언트 생성
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Username:     cfg.RedisUsername,
		Password:     cfg.RedisPassword,
		TLSConfig:    tlsConfig,
		DB:           0,                // 기본 DB
		DialTimeout:  10 * time.Second, // 타임아웃 늘림
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	// 연결 테스트
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("🔍 Testing Redis connection...")
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("❌ Redis ping failed: %v", err)
		return nil
	}

	return rdb
}
