package eats

import (
	"fmt"
	"strings"
)

// PromptCategories - 카테고리별 이미지 분류 구조체 (Eats 전용)
// 프론트 type: food, ingredient, prop, background
type PromptCategories struct {
	Food       [][]byte // Food (메인 음식) 이미지 배열
	Ingredient [][]byte // Ingredient (재료) 이미지 배열
	Prop       [][]byte // Prop (소품) 이미지 배열
	Background []byte   // Background (배경) 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Eats 모듈 전용 프롬프트 생성 (음식 사진)
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의 (프론트 type 기준)
	hasFood := len(categories.Food) > 0           // type: food
	hasIngredient := len(categories.Ingredient) > 0 // type: ingredient
	hasProp := len(categories.Prop) > 0            // type: prop
	hasFoodItems := hasIngredient || hasProp
	hasBackground := categories.Background != nil // type: background

	// 배경 설정에 따른 환경 지시
	var backgroundInstruction string
	if hasBackground {
		backgroundInstruction = "Use the provided background image as the environment.\n"
	} else {
		backgroundInstruction = "🚨 MANDATORY BACKGROUND: SOLID PURE WHITE (RGB 255,255,255) ONLY\n" +
			"❌ NO shadows on background, NO gradient, NO surface texture\n" +
			"❌ NO table, NO floor, NO wall, NO environment - ONLY white void\n" +
			"✓ Food items floating in pure white space like product catalog\n" +
			"✓ Shadows ONLY under food items, NOT on background\n\n"
	}

	// 간결한 메인 지시사항
	var mainInstruction string
	if hasFood || hasFoodItems {
		mainInstruction = backgroundInstruction +
			"\n🚨 CRITICAL: Generate ONE SINGLE UNIFIED PHOTOGRAPH\n" +
			"• This is NOT a collage - all food items exist together naturally in ONE SCENE\n" +
			"• ALL items clustered together in the CENTER of the frame - like a composed dish\n" +
			"• Items are closely grouped and naturally arranged, NOT scattered across the image\n" +
			"• Natural shadows, depth, and spatial relationships between items\n" +
			"• Professional food photography - photorealistic, appetizing, natural colors\n\n"
	} else {
		mainInstruction = "Environment photography.\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명 (음식 용어로)
	if len(categories.Food) > 0 {
		if len(categories.Food) == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (MAIN FOOD): This is a FOOD photograph showing colors, textures, and presentation. This is NOT a person - it's FOOD. Recreate this FOOD EXACTLY with the same culinary style", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (MAIN FOOD - MULTIPLE ITEMS): These are %d FOOD items shown in a GRID LAYOUT for reference only.\n"+
					"⚠️ CRITICAL: DO NOT recreate this grid layout in the final image!\n"+
					"Instead: CLUSTER all these foods together in the CENTER of the frame - grouped closely like a composed meal.\n"+
					"Arrange them naturally as if they were served together, NOT scattered in a grid pattern.\n"+
					"Create ONE UNIFIED SCENE with all items naturally integrated with shadows and depth.", imageIndex, len(categories.Food)))
		}
		imageIndex++
	}

	if len(categories.Ingredient) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (INGREDIENTS/SIDES): ALL visible ingredients, side items, or components. The food MUST include EVERY item shown here", imageIndex))
		imageIndex++
	}

	if len(categories.Prop) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (TOPPINGS/GARNISH): ALL toppings, garnishes, sauces, herbs, or finishing touches. The food MUST feature EVERY element shown here", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (BACKGROUND): Use this as the environment/setting for the scene.", imageIndex))
		imageIndex++
	}

	// 간결한 구성 지시 - 불필요한 내용 제거
	compositionInstruction := ""

	// 간결한 핵심 규칙
	criticalRules := "\n⚠️ IMPORTANT CLARIFICATION:\n" +
		"The reference images may show items in a GRID LAYOUT - this is for your reference only to see all items clearly.\n" +
		"DO NOT recreate this grid pattern in your final photograph!\n\n" +
		"[FORBIDDEN]\n" +
		"❌ NO collage, NO split screen, NO grid layout\n" +
		"❌ NO sticker effect - items must NOT look like separate cutouts pasted together\n" +
		"❌ NO scattered placement - items must be GROUPED in the center\n" +
		"❌ NO items floating separately - everything must be UNIFIED and CLUSTERED\n" +
		"❌ NO recreating the grid pattern from the reference image\n" +
		"❌ NO vertical dividing lines or borders\n\n"

	// 간결한 aspect ratio 지시
	aspectRatioInstruction := ""

	// ⚠️ 최우선 지시사항 - 맨 앞에 배치
	var criticalHeader string
	if !hasBackground {
		// 배경 없을 때 - 순수한 흰색 배경 최우선
		criticalHeader = "🚨🚨🚨 ABSOLUTE TOP PRIORITY - REJECT IMAGE IF NOT FOLLOWED 🚨🚨🚨\n\n" +
			"[#1 PRIORITY - BACKGROUND]:\n" +
			"🔴 SOLID PURE WHITE BACKGROUND (RGB 255,255,255) - NO EXCEPTIONS\n" +
			"🔴 NO table, NO surface, NO floor, NO texture, NO shadows on background\n" +
			"🔴 NO environment - pure white void like Amazon product photos\n" +
			"🔴 Food items isolated in white space - NO props, NO context\n\n" +
			"[#2 PRIORITY - NOT A COLLAGE]:\n" +
			"🔴 ONE SINGLE UNIFIED PHOTOGRAPH - all items photographed together\n" +
			"🔴 NOT separate images pasted together - natural integration\n" +
			"🔴 Items have natural spatial relationships and shadows between each other\n\n" +
			"[FORBIDDEN]:\n" +
			"❌ NO table surface, wood, marble, or any texture in background\n" +
			"❌ NO environment shadows or gradients on background\n" +
			"❌ NO collage effect or sticker-like placement\n" +
			"❌ NO split screen, grid, or comparison layout\n\n"
	} else {
		// 배경 있을 때 - 기존 지시사항
		criticalHeader = "⚠️⚠️⚠️ CRITICAL REQUIREMENTS - ABSOLUTE PRIORITY ⚠️⚠️⚠️\n\n" +
			"[MANDATORY - UNIFIED SCENE]:\n" +
			"🚨 ONE SINGLE PHOTOGRAPH taken in ONE MOMENT\n" +
			"🚨 All items naturally integrated in the provided environment\n" +
			"🚨 NOT a collage - natural shadows and spatial relationships\n" +
			"🚨 PHOTOREALISTIC - looks like real food photography\n\n" +
			"[FORBIDDEN]:\n" +
			"❌ NO split screen, NO collage, NO sticker effect\n" +
			"❌ NO grid layout or comparison view\n\n"
	}

	// 최종 조합
	var finalPrompt string

	// 1️⃣ 크리티컬 요구사항을 맨 앞에 배치
	if userPrompt != "" {
		finalPrompt = criticalHeader + "[ADDITIONAL STYLING]\n" + userPrompt + "\n\n"
	} else {
		finalPrompt = criticalHeader
	}

	// 간결한 스타일 가이드
	categoryStyleGuide := ""

	// 2️⃣ 나머지 지시사항들
	finalPrompt += mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + categoryStyleGuide + criticalRules + aspectRatioInstruction

	return finalPrompt
}
