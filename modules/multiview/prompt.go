package multiview

import "fmt"

// BuildMultiviewPrompt - 각도별 이미지 생성을 위한 프롬프트 생성
// sourceAngle: 원본 이미지의 각도 (보통 0 = 정면)
// targetAngle: 생성하려는 목표 각도
// category: 카테고리 (fashion, beauty, etc.)
// originalPrompt: 원본 프롬프트 (있는 경우)
// hasReference: 해당 각도에 레퍼런스 이미지가 있는지
// rotateBackground: 배경도 함께 회전할지 여부
func BuildMultiviewPrompt(sourceAngle, targetAngle int, category, originalPrompt string, hasReference, rotateBackground bool) string {
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
1. The FIRST image is the SOURCE image showing a scene from the front view (0 degrees)
2. The SECOND image is the REFERENCE image showing how the scene should look from the %s view (%d degrees)

TASK: Generate a new image that shows the SAME ENTIRE SCENE from the SOURCE image, but viewed from the %s angle (%d degrees).

IMPORTANT - CAMERA ORBIT ROTATION:
Imagine the camera is orbiting around the ENTIRE SCENE (not just the subject).
The camera moves %s while keeping the scene center fixed.
BOTH the subject AND the background/environment should rotate together as a unified scene.

CRITICAL REQUIREMENTS:
- The generated image MUST show the ENTIRE SCENE rotated, including background and environment
- Maintain the EXACT same subject identity, clothing, colors, textures, and details
- The background should change perspective naturally as the camera orbits (e.g., if there's a wall on the left in front view, it should appear differently from side/back views)
- Use the REFERENCE image as a guide for the correct angle, pose, and perspective
- Maintain consistent lighting direction relative to the scene
- %s

OUTPUT: Generate ONLY the image, no text or explanations.`,
			angleLabel, targetAngle,
			angleLabel, targetAngle,
			rotationDesc,
			categoryContext)
	} else {
		// 레퍼런스 이미지가 없는 경우 - AI가 각도 추론
		if rotateBackground {
			// 배경도 함께 회전하는 경우 (카메라가 씬 주위를 공전)
			prompt = fmt.Sprintf(`Generate the SAME ENTIRE SCENE from a different camera angle. The camera is orbiting around the scene.

TARGET: %s view (%d degrees) - %s

IMPORTANT - CAMERA ORBIT ROTATION:
The camera moves around the ENTIRE SCENE (including background).
Both the subject AND the background/environment rotate together as a unified scene.

REQUIREMENTS:
1. Show the subject from their %s: %s
2. The background should also rotate with the camera orbit - show what would naturally be visible from this camera position
3. Keep the subject's identity, clothing, colors exactly the same
4. The overall mood and lighting style should be consistent
5. Imagine the camera is physically moving around the scene, so the background perspective changes accordingly

%s

%s

OUTPUT: Generate ONLY the image. No text.`,
				angleLabel, targetAngle, rotationDesc,
				angleLabel, getAngleSpecificGuidance(targetAngle),
				categoryContext,
				getOriginalPromptContext(originalPrompt))
		} else {
			// 배경 고정 - 피사체만 회전 (기본값)
			prompt = fmt.Sprintf(`Generate the SAME SUBJECT from a different viewing angle, keeping the background fixed.

TARGET: %s view (%d degrees) - %s

IMPORTANT - SUBJECT ROTATION ONLY:
Only rotate the SUBJECT (person/object), NOT the background.
Keep the same background/environment as the original image.
The subject should appear to have turned/rotated while standing in the same place.

REQUIREMENTS:
1. Show the subject from their %s: %s
2. Keep the EXACT SAME background as the original image (do not rotate or change the background)
3. Keep the subject's identity, clothing, colors exactly the same
4. The subject should look like they simply turned to face a different direction
5. The overall mood and lighting style should be consistent

%s

%s

OUTPUT: Generate ONLY the image. No text.`,
				angleLabel, targetAngle, rotationDesc,
				angleLabel, getAngleSpecificGuidance(targetAngle),
				categoryContext,
				getOriginalPromptContext(originalPrompt))
		}
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

// getBackgroundAngleDescription - 배경이 어떻게 보여야 하는지 설명
func getBackgroundAngleDescription(angle int) string {
	switch angle {
	case 45:
		return "right-front"
	case 90:
		return "right"
	case 135:
		return "right-back"
	case 180:
		return "back"
	case 225:
		return "left-back"
	case 270:
		return "left"
	case 315:
		return "left-front"
	default:
		return "corresponding"
	}
}

// getDetailedAngleInstruction - 각도별 상세 지시
func getDetailedAngleInstruction(angle int) string {
	switch angle {
	case 45:
		return `- Camera moved 45° to the right
- We now see the subject's front-right side
- Background elements that were on the LEFT in original are now more visible
- Background elements on the RIGHT are now less visible or hidden
- The arch/doorway frame should show its right inner edge more`
	case 90:
		return `- Camera moved 90° to the right (side view)
- We now see the subject's complete right profile
- Background that was BEHIND the subject is now on the LEFT side of frame
- Background that was in FRONT is now on the RIGHT side
- The arch/doorway should be viewed from its side`
	case 135:
		return `- Camera moved 135° (back-right view)
- We see mostly the subject's back with some right side visible
- The original background is now mostly on our LEFT
- We should see what was BEHIND the camera in the original shot
- The arch is now viewed from behind-right`
	case 180:
		return `- Camera moved 180° (complete back view)
- We see the subject's back completely
- The original background (building/arch) should now be BEHIND the camera
- We see what was originally behind the photographer
- The arch/doorway is now behind us, not visible`
	case 225:
		return `- Camera moved 225° (back-left view)
- We see mostly the subject's back with some left side visible
- Similar to 135° but mirrored
- The arch is viewed from behind-left`
	case 270:
		return `- Camera moved 270° to the left (left side view)
- We see the subject's complete left profile
- Background layout is mirrored from 90° view
- The arch/doorway viewed from its left side`
	case 315:
		return `- Camera moved 315° (front-left view)
- We see the subject's front-left side
- Background elements on the RIGHT in original are now more visible
- The arch should show its left inner edge more`
	default:
		return "Rotate the entire scene accordingly"
	}
}

// BuildConsistencyCheckPrompt - 일관성 체크를 위한 프롬프트 (선택적 사용)
func BuildConsistencyCheckPrompt() string {
	return `Compare the two images and verify:
1. Is this the SAME subject shown from different angles?
2. Are colors, textures, and materials consistent?
3. Is the rotation angle correct?

OUTPUT: Answer with "CONSISTENT" or "INCONSISTENT" followed by brief explanation.`
}
