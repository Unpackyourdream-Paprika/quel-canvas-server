package cartoon

import (
	"fmt"
	"strings"
)

// PromptCategories - 카테고리별 이미지 분류 구조체 (프롬프트 생성용)
type PromptCategories struct {
	Character  [][]byte // 캐릭터 이미지 배열 (최대 3명) - 웹툰/만화 캐릭터
	Prop       [][]byte // 소품/아이템 이미지 배열
	Background []byte   // 배경 이미지 (최대 1장)
}

// GetAngleDescription - 앵글에 대한 상세 설명 반환
func GetAngleDescription(angle string) string {
	descriptions := map[string]string{
		"front": "FRONT VIEW - Character facing directly toward camera. " +
			"Face visible straight-on, symmetrical composition. " +
			"Both eyes, nose, mouth clearly visible. Eye-level camera.",
		"three-quarter": "THREE-QUARTER VIEW (3/4 ANGLE) - Character turned 30-45 degrees. " +
			"One side of face more visible, creates depth and dimension. " +
			"Dynamic but still shows facial features clearly. Eye-level camera.",
		"side": "SIDE VIEW (PROFILE) - Character turned 90 degrees, showing profile. " +
			"Only one side of face visible. Nose, lips, chin silhouette emphasized. " +
			"Dramatic silhouette composition. Eye-level camera.",
		"low-angle": "LOW ANGLE - CRITICAL: Camera MUST be positioned BELOW the character, looking UP at them. " +
			"NOT eye-level. NOT front facing. The viewer is looking UP from below. " +
			"Character's chin and underside of jaw visible. Nostrils slightly visible. " +
			"Character appears powerful, heroic, towering over the viewer. " +
			"Sky or ceiling visible behind/above character. Legs appear larger due to perspective.",
		"high-angle": "HIGH ANGLE - CRITICAL: Camera MUST be positioned ABOVE the character, looking DOWN at them. " +
			"NOT eye-level. NOT front facing. The viewer is looking DOWN from above. " +
			"Top of character's head more visible. Forehead prominent, chin less visible. " +
			"Character appears smaller, vulnerable, or overwhelmed. " +
			"Ground/floor visible around character. Shoulders appear broader due to perspective.",
	}
	if desc, ok := descriptions[angle]; ok {
		return desc
	}
	return fmt.Sprintf("%s view angle", angle)
}

// GetShotDescription - 샷 타입에 대한 상세 설명 반환
func GetShotDescription(shot string) string {
	descriptions := map[string]string{
		"portrait": "PORTRAIT SHOT - Close focus on face. " +
			"From neck/collar up. Emphasizes facial expression, emotions. " +
			"Intimate, detailed view of character's face.",
		"bust": "BUST SHOT - Frame from chest/shoulders up. " +
			"Focus on face and upper body. Good for expressions and emotions. " +
			"Head and shoulders fill most of the frame.",
		"full-body": "FULL BODY SHOT - Entire character visible from head to toe. " +
			"Shows complete outfit, pose, and body language. " +
			"Character takes up most of vertical space. No cropping.",
	}
	if desc, ok := descriptions[shot]; ok {
		return desc
	}
	return fmt.Sprintf("%s framing", shot)
}

// GetFXDescription - 만화/웹툰 FX 효과에 대한 상세 설명 반환
func GetFXDescription(fx string) string {
	descriptions := map[string]string{
		"none": "", // No FX - return empty string
		"speed-lines": "SPEED LINES FX (집중선) - Add dramatic manga-style speed lines. " +
			"Lines radiate from character or toward action direction. " +
			"Background simplified with motion blur effect. " +
			"Character remains sharp while environment shows movement. " +
			"Creates sense of fast motion, urgency, or dramatic focus.",
		"impact": "IMPACT FX - Add explosive manga-style impact effects. " +
			"Radiating lines burst from point of contact or action. " +
			"Shockwave ripples, debris particles flying outward. " +
			"Screen tone effects, halftone patterns for emphasis. " +
			"Creates sense of powerful collision, punch, or explosion.",
		"aura": "AURA FX - Add glowing energy aura around character. " +
			"Visible energy emanating from character's body. " +
			"Flowing, flame-like or electric energy particles. " +
			"Color can match character's power or emotion. " +
			"Creates sense of power-up, transformation, or supernatural ability.",
		"emotion": "EMOTION FX - Add manga-style emotional expression effects. " +
			"Sweat drops for nervousness, anger veins for frustration. " +
			"Sparkles for happiness, dark aura for depression. " +
			"Floating symbols (hearts, stars, question marks). " +
			"Exaggerated visual cues that amplify character's emotional state.",
	}
	if desc, ok := descriptions[fx]; ok {
		return desc
	}
	return ""
}

// GenerateDynamicPrompt - Cartoon 모듈 전용 프롬프트 생성
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	hasCharacter := len(categories.Character) > 0
	characterCount := len(categories.Character)
	hasProp := len(categories.Prop) > 0
	hasBackground := categories.Background != nil

	var promptBuilder strings.Builder

	// 최우선 규칙 - 통합된 씬 (캐릭터가 있는 경우)
	if hasCharacter {
		promptBuilder.WriteString("=== CREATE ONE UNIFIED ILLUSTRATION ===\n\n")
		promptBuilder.WriteString("[THE GOAL]\n")
		promptBuilder.WriteString("Place the character INTO the scene. They must look like they BELONG there.\n")
		promptBuilder.WriteString("The scene's lighting must affect the character naturally.\n")
		promptBuilder.WriteString("Character must cast shadows and interact with the environment.\n\n")
		promptBuilder.WriteString("[CHARACTER - KEEP DESIGN]\n")
		promptBuilder.WriteString("Same face, same hairstyle, same outfit design, same color palette.\n")
		promptBuilder.WriteString("BUT the character must be LIT by the scene's lighting.\n")
		promptBuilder.WriteString("If background has warm sunset, character has warm tones.\n")
		promptBuilder.WriteString("If background has cool night colors, character reflects cool tones.\n\n")
		promptBuilder.WriteString("[BODY - CORRECT PROPORTIONS]\n")
		promptBuilder.WriteString("Maintain the character's original body proportions.\n")
		promptBuilder.WriteString("No stretched or distorted limbs.\n\n")
		promptBuilder.WriteString("=================================\n\n")
	}

	// 메인 지시사항
	promptBuilder.WriteString("[ILLUSTRATION STYLE]\n\n")
	promptBuilder.WriteString("You are an illustrator creating a cohesive artwork.\n")
	if hasCharacter {
		promptBuilder.WriteString(fmt.Sprintf("Create ONE unified illustration with EXACTLY %d character(s).\n\n", characterCount))
	} else if hasProp {
		promptBuilder.WriteString("Create ONE illustration showcasing the items. NO characters.\n\n")
	} else {
		promptBuilder.WriteString("Create ONE environment illustration. NO characters, NO items.\n\n")
	}

	// 우선순위
	if hasCharacter {
		promptBuilder.WriteString("[PRIORITY 1 - CHARACTER]: Keep design IDENTICAL. Apply scene lighting.\n")
	}
	if hasProp {
		promptBuilder.WriteString("[PRIORITY 2 - PROPS]: Keep shape/color. Redraw with scene lighting.\n")
	}
	promptBuilder.WriteString("[PRIORITY LAST - BACKGROUND]: Reference is just inspiration. Create a fitting environment.\n\n")

	// 통일된 조명
	promptBuilder.WriteString("[UNIFIED LIGHTING]:\n")
	promptBuilder.WriteString("One light source for entire scene. Character, props, background - all same direction.\n")
	promptBuilder.WriteString("Original lighting from reference must be replaced with scene lighting.\n\n")

	// 레퍼런스 이미지 설명
	promptBuilder.WriteString("[REFERENCE IMAGES]:\n")
	imageIndex := 1
	if hasCharacter {
		for i := 0; i < characterCount; i++ {
			promptBuilder.WriteString(fmt.Sprintf("Image %d (CHARACTER %d): Design reference. Keep appearance, apply scene lighting.\n", imageIndex, i+1))
			imageIndex++
		}
	}
	if hasProp {
		promptBuilder.WriteString(fmt.Sprintf("Image %d (PROPS): Shape and color reference. Redraw with scene lighting.\n", imageIndex))
		imageIndex++
	}
	if hasBackground {
		promptBuilder.WriteString(fmt.Sprintf("Image %d (BACKGROUND): Mood inspiration ONLY. Create a NEW fitting environment.\n", imageIndex))
	}
	promptBuilder.WriteString("\n")

	// 출력 요구사항
	promptBuilder.WriteString("[OUTPUT - FILL ENTIRE FRAME]:\n")
	promptBuilder.WriteString("Image must touch all 4 edges. NO empty space.\n")
	promptBuilder.WriteString("NO black bars. Content fills 100% of canvas.\n\n")

	// 금지사항
	promptBuilder.WriteString("[FORBIDDEN]:\n")
	promptBuilder.WriteString("- ANY character design modification (different face, hair, outfit)\n")
	promptBuilder.WriteString("- Body distortion (stretched limbs, wrong proportions)\n")
	promptBuilder.WriteString("- Split screens, collages, borders, multiple panels\n")
	promptBuilder.WriteString("- Floating objects, physics violations\n\n")

	// 16:9 비율
	if aspectRatio == "16:9" {
		promptBuilder.WriteString("[16:9 WIDE FORMAT]:\n")
		promptBuilder.WriteString("Use horizontal space for dynamic composition.\n\n")
	}

	// 사용자 프롬프트
	if userPrompt != "" {
		promptBuilder.WriteString("[STYLING]:\n")
		promptBuilder.WriteString(userPrompt)
	}

	return promptBuilder.String()
}
