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

// GenerateDynamicPrompt - Cartoon 모듈 전용 프롬프트 생성 (웹툰, 만화, 애니메이션)
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasCharacter := len(categories.Character) > 0
	characterCount := len(categories.Character)
	hasProp := len(categories.Prop) > 0
	hasBackground := categories.Background != nil

	// 웹툰 스타일 기본 지시사항 (간소화)
	var baseStyle string
	if characterCount <= 1 {
		baseStyle = "[WEBTOON/MANGA ILLUSTRATION STYLE]\n" +
			"Generate webtoon/manga style character illustration.\n" +
			"Style: Clean linework, vibrant saturated colors, cel-shading or flat shading.\n" +
			"2D comic art aesthetic - NOT photorealistic, NOT 3D render.\n\n"
	} else {
		baseStyle = fmt.Sprintf("[WEBTOON/MANGA ILLUSTRATION STYLE - %d CHARACTERS]\n"+
			"Generate webtoon/manga style illustration with MULTIPLE CHARACTERS.\n"+
			"Each CHARACTER must appear exactly as shown in their reference image.\n"+
			"Style: Clean linework, vibrant saturated colors, cel-shading or flat shading.\n"+
			"2D comic art aesthetic - NOT photorealistic, NOT 3D render.\n\n", characterCount)
	}

	// 참조 이미지 설명 - 다중 캐릭터 지원
	var instructions []string
	imageIndex := 1

	for i := range categories.Character {
		if len(categories.Character) == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (CHARACTER APPEARANCE): Use this person's face, hairstyle, body features as character reference", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (CHARACTER %d APPEARANCE): Use this person's face, hairstyle, body features as CHARACTER %d reference - MUST appear exactly as shown", imageIndex, i+1, i+1))
		}
		imageIndex++
	}

	if len(categories.Prop) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (PROPS/ITEMS): Character wears/carries ALL these items", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (SCENE BACKGROUND): Recreate this setting/atmosphere in webtoon background art style", imageIndex))
		imageIndex++
	}

	// 사용자 프롬프트 (캐릭터 묘사) - 최우선
	var characterDescription string
	if userPrompt != "" {
		characterDescription = "\n[CHARACTER DESCRIPTION - USER INPUT]\n" + userPrompt + "\n"
	}

	// 기본 구성 지시
	var compositionInstruction string
	if hasCharacter {
		if characterCount == 1 {
			compositionInstruction = "\n[COMPOSITION]\n" +
				"Generate ONE webtoon/manga character illustration.\n" +
				"Character wears complete outfit (all props/items).\n"
		} else {
			compositionInstruction = fmt.Sprintf("\n[COMPOSITION - %d CHARACTERS]\n"+
				"Generate ONE webtoon/manga illustration with %d DISTINCT CHARACTERS.\n"+
				"Each character MUST appear exactly as shown in their reference image.\n"+
				"Characters interact naturally within the same scene.\n", characterCount, characterCount)
		}
		if hasBackground {
			compositionInstruction += "Character(s) positioned naturally in the scene background.\n"
		}
	} else if hasProp {
		compositionInstruction = "\n[COMPOSITION]\n" +
			"Generate webtoon-style item/prop illustration.\n" +
			"⚠️ NO characters - items only.\n"
	} else if hasBackground {
		compositionInstruction = "\n[COMPOSITION]\n" +
			"Generate webtoon-style background scene.\n" +
			"⚠️ NO characters - background only.\n"
	}

	// 필수 금지사항 (최소화)
	criticalRules := "\n[CRITICAL REQUIREMENTS]\n" +
		"✓ Webtoon/manga illustration style (clean lines, vibrant colors, cel-shading)\n" +
		"✓ ONE unified comic panel - no split/collage layouts\n" +
		"✓ Character wears all referenced clothing/items\n" +
		"❌ NO photorealistic rendering\n" +
		"❌ NO vertical dividing lines or panel splits\n\n" +
		"[ABSOLUTELY FORBIDDEN - IMAGE WILL BE REJECTED]:\n" +
		"- NO left-right split, NO side-by-side layout, NO duplicate subject on both sides\n" +
		"- NO grid, NO collage, NO comparison view, NO before/after layout\n" +
		"- NO vertical dividing line, NO center split, NO symmetrical duplication\n" +
		"- NO white/gray borders, NO letterboxing, NO empty margins on any side\n" +
		"- NO multiple identical poses, NO mirrored images, NO panel divisions\n" +
		"- NO vertical portrait orientation with side margins\n\n" +
		"[REQUIRED - MUST GENERATE THIS WAY]:\n" +
		"- ONE single continuous illustration in ONE unified style\n" +
		"- ONE unified moment in time - NOT two or more moments combined\n" +
		"- FILL entire frame edge-to-edge with NO empty space\n" +
		"- Natural asymmetric composition - left side MUST be different from right side\n" +
		"- Professional webtoon/manga style - single panel illustration only\n"

	// Aspect ratio 기본 지시
	var aspectRatioHint string
	if aspectRatio == "9:16" {
		aspectRatioHint = "\n[FORMAT: 9:16 Vertical] Show character in vertical webtoon panel composition.\n"
	} else if aspectRatio == "16:9" {
		aspectRatioHint = "\n[FORMAT: 16:9 Wide] Wide webtoon scene composition.\n"
	} else {
		aspectRatioHint = "\n[FORMAT: Square] Balanced webtoon character composition.\n"
	}

	// 카테고리별 고정 스타일 가이드
	categoryStyleGuide := "\n\n[CARTOON CHARACTER DESCRIPTION GUIDE]\n" +
		"Describe character details: facial expression (eyes, mouth, eyebrows), gesture (hand position, arm movement), body pose (standing, sitting, jumping), emotion (happy, angry, surprised, sad), and situation (what they are doing). Be specific and detailed. Do NOT mention art style, colors, or visual effects.\n\n" +
		"[TECHNICAL CONSTRAINTS]\n" +
		"ABSOLUTELY NO VERTICAL COMPOSITION. ABSOLUTELY NO SIDE MARGINS. ABSOLUTELY NO WHITE/GRAY BARS ON LEFT OR RIGHT. Fill entire width from left edge to right edge. NO letterboxing. NO pillarboxing. NO empty sides.\n"

	// 최종 조합: 스타일 → 참조 이미지 → 캐릭터 묘사(사용자) → 구성 → 카테고리 스타일 → 금지사항 → 비율
	finalPrompt := baseStyle +
		strings.Join(instructions, "\n") +
		characterDescription +
		compositionInstruction +
		categoryStyleGuide +
		criticalRules +
		aspectRatioHint

	return finalPrompt
}
