package multiview

import "fmt"

// BuildMultiviewPrompt - 각도별 이미지 생성을 위한 프롬프트 생성
// sourceAngle: 원본 이미지의 각도 (보통 0 = 정면)
// targetAngle: 생성하려는 목표 각도
// category: 카테고리 (fashion, beauty, etc.)
// originalPrompt: 원본 프롬프트 (있는 경우)
// hasReference: 해당 각도에 레퍼런스 이미지가 있는지
func BuildMultiviewPrompt(sourceAngle, targetAngle int, category, originalPrompt string, hasReference bool) string {
	angleLabel := GetAngleLabel(targetAngle)
	angleDiff := targetAngle - sourceAngle
	if angleDiff < 0 {
		angleDiff += 360
	}

	// 회전 방향 설명
	rotationDesc := getRotationDescription(angleDiff)

	// 카테고리별 컨텍스트
	categoryContext := getCategoryContext(category)

	// 레퍼런스가 있는 경우와 없는 경우 구분
	var prompt string

	if hasReference {
		// 레퍼런스 이미지가 있는 경우 - 더 정확한 재현
		prompt = fmt.Sprintf(`You are given two images:
1. The FIRST image is the SOURCE image showing the subject from the front view (0 degrees)
2. The SECOND image is the REFERENCE image showing how the subject should look from the %s view (%d degrees)

TASK: Generate a new image that shows the SAME subject from the SOURCE image, but from the %s angle (%d degrees).

CRITICAL REQUIREMENTS:
- The generated image MUST maintain the EXACT same subject identity, clothing, colors, textures, and details as the SOURCE image
- Use the REFERENCE image as a guide for the correct angle, pose, and perspective
- The rotation should be %s from the front view
- Maintain consistent lighting and style with the source image
- %s

OUTPUT: Generate ONLY the image, no text or explanations.`,
			angleLabel, targetAngle,
			angleLabel, targetAngle,
			rotationDesc,
			categoryContext)
	} else {
		// 레퍼런스 이미지가 없는 경우 - AI가 각도 추론
		prompt = fmt.Sprintf(`You are given an image showing a subject from the FRONT VIEW (0 degrees).

TASK: Generate a new image that shows the EXACT SAME subject from the %s angle (%d degrees).

ROTATION DETAILS:
- Current view: Front (0 degrees)
- Target view: %s (%d degrees)
- Rotation: %s from the current view

CRITICAL REQUIREMENTS:
- The generated image MUST maintain the EXACT same subject identity
- Keep all visual details consistent: colors, textures, patterns, materials
- Properly show the subject as it would appear when rotated %d degrees
- For %s view: %s
- Maintain consistent lighting, shadows, and atmosphere
- %s

%s

OUTPUT: Generate ONLY the image, no text or explanations.`,
			angleLabel, targetAngle,
			angleLabel, targetAngle,
			rotationDesc,
			targetAngle,
			angleLabel, getAngleSpecificGuidance(targetAngle),
			categoryContext,
			getOriginalPromptContext(originalPrompt))
	}

	return prompt
}

// BuildAnalyzeSourcePrompt - 원본 이미지 분석을 위한 프롬프트
func BuildAnalyzeSourcePrompt(category string) string {
	categoryContext := getCategoryContext(category)

	return fmt.Sprintf(`Analyze this image and extract detailed information for consistent multi-angle image generation.

ANALYZE AND DESCRIBE:
1. SUBJECT: What is the main subject? (person, product, object, etc.)
2. IDENTITY FEATURES: Key identifying features that must remain consistent across all angles
3. COLORS & MATERIALS: Exact colors, textures, and materials present
4. LIGHTING: Direction, intensity, and mood of lighting
5. STYLE: Artistic style, photography style, or render quality
6. BACKGROUND: What is the background like?

CONTEXT: %s

OUTPUT FORMAT:
Return a concise description (2-3 sentences) that captures all essential visual elements for regenerating this subject from different angles. Focus on consistency-critical details.

Example output:
"A female model wearing a navy blue silk blazer with gold buttons, white crew-neck t-shirt, and dark denim jeans. Professional studio lighting from front-left, clean white background. Editorial fashion photography style."`,
		categoryContext)
}

// getRotationDescription - 회전 방향에 대한 설명
func getRotationDescription(angleDiff int) string {
	switch {
	case angleDiff == 0:
		return "no rotation (same as source)"
	case angleDiff == 45:
		return "45 degrees clockwise rotation (slight turn to the right)"
	case angleDiff == 90:
		return "90 degrees clockwise rotation (side view, facing right)"
	case angleDiff == 135:
		return "135 degrees clockwise rotation (three-quarter back view, right side)"
	case angleDiff == 180:
		return "180 degrees rotation (full back view)"
	case angleDiff == 225:
		return "225 degrees clockwise / 135 degrees counter-clockwise (three-quarter back view, left side)"
	case angleDiff == 270:
		return "270 degrees clockwise / 90 degrees counter-clockwise (side view, facing left)"
	case angleDiff == 315:
		return "315 degrees clockwise / 45 degrees counter-clockwise (slight turn to the left)"
	default:
		return fmt.Sprintf("%d degrees rotation from front view", angleDiff)
	}
}

// getAngleSpecificGuidance - 각도별 구체적인 가이드
func getAngleSpecificGuidance(angle int) string {
	switch angle {
	case 0:
		return "Show the front face/surface directly facing the camera"
	case 45:
		return "Show front-right perspective, with about 3/4 of the front visible and 1/4 of the right side visible"
	case 90:
		return "Show the complete right side profile, front should not be visible"
	case 135:
		return "Show back-right perspective, with about 1/4 of the right side and 3/4 of the back visible"
	case 180:
		return "Show the complete back view, front should not be visible at all"
	case 225:
		return "Show back-left perspective, with about 3/4 of the back and 1/4 of the left side visible"
	case 270:
		return "Show the complete left side profile, front should not be visible"
	case 315:
		return "Show front-left perspective, with about 3/4 of the front visible and 1/4 of the left side visible"
	default:
		return fmt.Sprintf("Show the subject rotated %d degrees from the front view", angle)
	}
}

// getCategoryContext - 카테고리별 컨텍스트
func getCategoryContext(category string) string {
	switch category {
	case "fashion":
		return "Fashion/Clothing context: Pay special attention to fabric draping, garment fit, and how clothing moves with body rotation. Ensure all fashion details (buttons, zippers, patterns) are correctly positioned for each angle."
	case "beauty":
		return "Beauty/Cosmetics context: Maintain makeup consistency, skin texture, and facial features across angles. Hair should flow naturally for each viewing angle."
	case "eats":
		return "Food/Cuisine context: Maintain food presentation, plating details, and garnish positions as they would appear from each angle. Consider natural food geometry."
	case "cinema":
		return "Cinematic context: Preserve the dramatic lighting, mood, and composition quality. Maintain the cinematic aspect and atmosphere across all angles."
	case "cartoon":
		return "Illustration/Cartoon context: Maintain the art style, line quality, and color palette consistency. Character features should follow the established style guide."
	default:
		return "Commercial photography context: Maintain professional quality and visual consistency across all viewing angles."
	}
}

// getOriginalPromptContext - 원본 프롬프트가 있으면 컨텍스트에 추가
func getOriginalPromptContext(originalPrompt string) string {
	if originalPrompt == "" {
		return ""
	}
	return fmt.Sprintf(`
ORIGINAL DESCRIPTION:
The source image was created with this context: "%s"
Use this information to better understand the subject and maintain consistency.`, originalPrompt)
}

// BuildConsistencyCheckPrompt - 일관성 체크를 위한 프롬프트 (선택적 사용)
func BuildConsistencyCheckPrompt() string {
	return `Compare the two images and verify:
1. Is this the SAME subject shown from different angles?
2. Are colors, textures, and materials consistent?
3. Is the rotation angle correct?

OUTPUT: Answer with "CONSISTENT" or "INCONSISTENT" followed by brief explanation.`
}
