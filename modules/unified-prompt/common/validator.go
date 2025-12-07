package common

import (
	"fmt"
	"strings"
)

// ValidateUnifiedPromptRequest - 기본 요청 검증
func ValidateUnifiedPromptRequest(req *UnifiedPromptRequest) error {
	// 프롬프트 검증
	if strings.TrimSpace(req.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}

	if len(req.Prompt) > 2000 {
		return fmt.Errorf("prompt too long (max 2000 characters)")
	}

	// 이미지 개수 검증
	if len(req.ReferenceImages) > 3 {
		return fmt.Errorf("too many reference images (max 3)")
	}

	// Aspect Ratio 검증
	if req.AspectRatio != "" {
		validRatios := map[string]bool{
			"1:1":  true,
			"16:9": true,
			"9:16": true,
			"4:3":  true,
			"3:4":  true,
		}
		if !validRatios[req.AspectRatio] {
			return fmt.Errorf("invalid aspect ratio: %s", req.AspectRatio)
		}
	}

	return nil
}

// ValidateLandingRequest - 랜딩 페이지 요청 검증
func ValidateLandingRequest(req *UnifiedPromptRequest) error {
	// 기본 검증
	if err := ValidateUnifiedPromptRequest(req); err != nil {
		return err
	}

	// 세션 ID 검증 (비회원 제한용)
	if strings.TrimSpace(req.SessionID) == "" {
		return fmt.Errorf("sessionId is required")
	}

	return nil
}

// ValidateStudioRequest - 스튜디오 요청 검증
func ValidateStudioRequest(req *UnifiedPromptRequest, category string) error {
	// 기본 검증
	if err := ValidateUnifiedPromptRequest(req); err != nil {
		return err
	}

	// 카테고리 검증
	if !IsValidCategory(category) {
		return fmt.Errorf("invalid category: %s", category)
	}

	// 회원 전용 - UserID 필수
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("userId is required for studio")
	}

	return nil
}
