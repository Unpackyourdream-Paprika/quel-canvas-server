package eats

import (
	"fmt"
	"strings"
)

// generateSimplifiedPrompt - isPreEdited: false일 때 사용하는 심플 버전 (다양성 최우선)
func generateSimplifiedPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 이미지 설명만 간단히
	var instructions []string
	imageIndex := 1

	// Food 이미지 설명
	foodCount := len(categories.Food)
	if foodCount > 0 {
		if foodCount == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d: Food item", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Images %d-%d: %d food items", imageIndex, imageIndex+foodCount-1, foodCount))
		}
		imageIndex += foodCount
	}

	// Ingredient 이미지 설명
	ingredientCount := len(categories.Ingredient)
	if ingredientCount > 0 {
		if ingredientCount == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d: Ingredient", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Images %d-%d: %d ingredients", imageIndex, imageIndex+ingredientCount-1, ingredientCount))
		}
		imageIndex += ingredientCount
	}

	// Prop 이미지 설명
	propCount := len(categories.Prop)
	if propCount > 0 {
		if propCount == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d: Prop/garnish", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Images %d-%d: %d props/garnishes", imageIndex, imageIndex+propCount-1, propCount))
		}
		imageIndex += propCount
	}

	// Background 이미지 설명
	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d: Background environment", imageIndex))
	}

	// 기본 금지사항 + 과장된 품질 요구
	basicProhibitions := "🔥🔥🔥 EXTREME PREMIUM QUALITY REQUIREMENTS 🔥🔥🔥\n\n" +
		"⚠️ ABSOLUTELY CRITICAL - NO SPLIT COMPOSITION:\n" +
		"❌ NO vertical dividing lines or center splits\n" +
		"❌ NO left-right duplicate layouts or comparison views\n" +
		"❌ NO grid, collage, or side-by-side arrangements\n" +
		"❌ NO white/gray borders or letterboxing\n\n" +
		"✅ MANDATORY ULTRA-PREMIUM EXECUTION:\n" +
		"✓ ONE BREATHTAKINGLY STUNNING unified photograph\n" +
		"✓ ONE FLAWLESSLY COMPOSED continuous scene from ONE camera shot\n" +
		"✓ PERFECTLY fill entire frame edge-to-edge with ZERO wasted space\n" +
		"✓ ULTRA-REALISTIC, MIND-BLOWINGLY photorealistic food photography\n" +
		"✓ EXCEPTIONAL artistic quality that COMMANDS attention\n" +
		"✓ PREMIUM editorial-grade execution - REFUSE mediocrity\n\n" +
		"💎 QUALITY MANDATE:\n" +
		"This must be EXTRAORDINARY. This must be UNFORGETTABLE. This must be MAGNIFICENT.\n" +
		"Push EVERY element to MAXIMUM creative excellence. NO compromises. NO shortcuts.\n" +
		"Create something that makes viewers STOP and STARE in AWE.\n\n"

	// 창의성 극대화 지시
	creativityBoost := "🎨 UNLEASH BOUNDLESS CREATIVITY 🎨\n\n" +
		"BREAK FREE from conventional food photography constraints!\n" +
		"EXPERIMENT FEARLESSLY with radical new perspectives!\n" +
		"INNOVATE with unexpected color palettes and lighting setups!\n" +
		"SURPRISE with unconventional compositions that challenge norms!\n" +
		"EXPLORE the absolute LIMITS of creative food photography!\n\n" +
		"💡 CREATIVE FREEDOM MANDATE:\n" +
		"You are NOT bound by traditional rules. You are an ARTIST with INFINITE creative license.\n" +
		"Take BOLD risks. Make DARING choices. Create something NEVER SEEN BEFORE.\n" +
		"Each frame should be a WORK OF ART - a creative MASTERPIECE that pushes boundaries.\n" +
		"Be WILDLY imaginative. Be OUTRAGEOUSLY creative. Be MAGNIFICENTLY original.\n\n"

	// Aspect ratio 정보 간단히
	var formatInfo string
	switch aspectRatio {
	case "1:1":
		formatInfo = "[FORMAT: 1:1 Square - Use this square canvas for BOLD, ARTISTIC compositions]\n"
	case "16:9":
		formatInfo = "[FORMAT: 16:9 Wide Horizontal - Use this cinematic format for DRAMATIC, EXPANSIVE storytelling]\n"
	case "9:16":
		formatInfo = "[FORMAT: 9:16 Tall Vertical - Use this portrait format for STRIKING, DYNAMIC vertical compositions]\n"
	default:
		formatInfo = "[FORMAT: " + aspectRatio + " - Use this unique format CREATIVELY]\n"
	}

	// 최종 조합 - 창의성 극대화 버전
	finalPrompt := basicProhibitions +
		creativityBoost +
		formatInfo +
		"\n[REFERENCE IMAGES]\n" +
		strings.Join(instructions, "\n") +
		"\n\n[USER CREATIVE DIRECTION]\n" +
		userPrompt +
		"\n\n" +
		"🚀 FINAL REMINDER: This is your chance to create something LEGENDARY. Make it COUNT!\n"

	return finalPrompt
}

// PromptCategories - 카테고리별 이미지 분류 구조체 (Eats 전용)
// 프론트 type: food, ingredient, prop, background
type PromptCategories struct {
	Food       [][]byte // Food (메인 음식) 이미지 배열
	Ingredient [][]byte // Ingredient (재료) 이미지 배열
	Prop       [][]byte // Prop (소품) 이미지 배열
	Background []byte   // Background (배경) 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Eats 모듈 전용 프롬프트 생성 (음식 사진)
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string, isPreEdited bool) string {
	// isPreEdited: false일 때는 간결한 버전 사용 (다양성 중시)
	if !isPreEdited {
		return generateSimplifiedPrompt(categories, userPrompt, aspectRatio)
	}

	// isPreEdited: true일 때는 기존 상세 버전 사용 (정확성 중시)
	// 케이스 분석을 위한 변수 정의 (프론트 type 기준)
	hasFood := len(categories.Food) > 0             // type: food
	hasIngredient := len(categories.Ingredient) > 0 // type: ingredient
	hasProp := len(categories.Prop) > 0             // type: prop
	hasFoodItems := hasIngredient || hasProp
	hasBackground := categories.Background != nil // type: background

	// 배경 설정에 따른 환경 지시
	var backgroundInstruction string
	if hasBackground {
		backgroundInstruction = "Use the provided background image as the environment.\n" +
			"STRONG studio lighting creating intense specular highlights and glossy reflections on food.\n"
	} else {
		backgroundInstruction = "White background with HIGH-INTENSITY professional food photography lighting.\n" +
			"CRITICAL: Lighting MUST create very strong bright highlights and wet glossy appearance on all food surfaces.\n"
	}

	// 간결한 메인 지시사항
	var mainInstruction string
	if hasFood || hasFoodItems {
		mainInstruction = backgroundInstruction +
			"\nPREMIUM FOOD PHOTOGRAPHY - ULTRA GLOSSY:\n" +
			"• Every food element must have individual shine and light reflection\n" +
			"• Food surface appears freshly oiled or moistened - extremely glossy and wet-looking\n" +
			"• Strong directional lighting creates bright specular highlights on all food surfaces\n" +
			"• Deep shadows and high-contrast lighting enhance three-dimensional form\n" +
			"• Professional studio lighting setup specifically for maximum gloss and shine\n\n"
	} else {
		mainInstruction = "Environment photography.\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명 (음식 용어로)
	if len(categories.Food) > 0 {
		if len(categories.Food) == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (MAIN FOOD): Recreate this SAME FOOD TYPE with the SAME INGREDIENTS.\n"+
					"KEEP: Same food identity, same core ingredients, same basic structure\n"+
					"ENHANCE: Make it look fresher, glossier, more appetizing with better lighting and presentation\n"+
					"Goal: Same food, elevated to professional food photography quality", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (MAIN FOOD - MULTIPLE ITEMS): These are %d FOOD items shown in a GRID LAYOUT for reference only.\n"+
					"⚠️ CRITICAL: DO NOT recreate this grid layout in the final image!\n"+
					"KEEP: Same food types, same ingredients from all items\n"+
					"CHANGE: CLUSTER all foods together naturally - NOT in a grid pattern\n"+
					"ENHANCE: Make them look fresher, glossier, more appetizing with professional lighting\n"+
					"Goal: Same foods, better composition and presentation quality", imageIndex, len(categories.Food)))
		}
		imageIndex++
	}

	if len(categories.Ingredient) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (INGREDIENTS/SIDES): Include these SAME ingredients/components.\n"+
				"ENHANCE with better freshness and visual appeal.", imageIndex))
		imageIndex++
	}

	if len(categories.Prop) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (TOPPINGS/GARNISH): Include these SAME toppings/garnishes.\n"+
				"ENHANCE with better color vibrancy and appetizing look.", imageIndex))
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
	criticalRules := "\n[FORBIDDEN]\n" +
		"❌ NO collage or split screen layout\n" +
		"❌ NO grid pattern from reference images\n\n"

	// 간결한 aspect ratio 지시
	aspectRatioInstruction := ""

	// ⚠️ 최우선 지시사항 - 맨 앞에 배치
	var criticalHeader string
	if !hasBackground {
		criticalHeader = "🚨 CRITICAL: ULTRA HIGH-GLOSS FOOD PHOTOGRAPHY 🚨\n\n" +
			"SURFACE QUALITY (ABSOLUTE PRIORITY):\n" +
			"• EVERY food element MUST sparkle with bright glossy highlights - like jewels\n" +
			"• Food surface MUST appear SOAKING WET with visible oil coating - EXTREMELY glossy\n" +
			"• INTENSE specular highlights creating bright white spots on ALL ingredients and surfaces\n" +
			"• Water droplets, moisture beads, or condensation on food surface HIGHLY PREFERRED\n" +
			"• MAXIMUM contrast - very bright highlights next to deep shadows\n" +
			"• Food looks like it was JUST sprayed with water or brushed with oil - ULTRA SHINY\n" +
			"• Every texture appears glistening and wet with individual light reflections\n\n" +
			"FORBIDDEN:\n" +
			"❌ ABSOLUTELY NO dry, matte, or dull appearance\n" +
			"❌ NO subtle or weak lighting - must be STRONG and BRIGHT\n" +
			"❌ NO flat cutout appearance\n\n"
	} else {
		criticalHeader = "🚨 CRITICAL: ULTRA HIGH-GLOSS FOOD PHOTOGRAPHY 🚨\n\n" +
			"SURFACE QUALITY (ABSOLUTE PRIORITY):\n" +
			"• EVERY food element MUST sparkle with bright glossy highlights - like jewels\n" +
			"• Food surface MUST appear SOAKING WET with visible oil coating - EXTREMELY glossy\n" +
			"• INTENSE specular highlights creating bright white spots on ALL food elements\n" +
			"• MAXIMUM contrast - very bright highlights next to deep shadows\n" +
			"• Food looks like it was JUST sprayed with water or brushed with oil\n\n" +
			"FORBIDDEN:\n" +
			"❌ ABSOLUTELY NO dry or matte appearance\n" +
			"❌ NO weak lighting\n\n"
	}

	// 최종 조합
	var finalPrompt string

	// 🚨 ABSOLUTE PROHIBITIONS - 맨 앞에 배치하여 절대 금지 사항 명확히
	absoluteProhibitions := "⛔ ABSOLUTE PROHIBITIONS (MUST NEVER HAPPEN):\n" +
		"❌ NEVER create images with BLACK or DARK backgrounds\n" +
		"❌ NEVER make food appear as floating PNG cutout on black/dark background\n" +
		"❌ NEVER use transparent or isolated product shot style\n" +
		"❌ NEVER create collage or split-screen layouts\n" +
		"❌ Background MUST be WHITE or light-colored studio environment\n\n" +
		"✅ MANDATORY: Clean white studio background with professional food photography lighting\n" +
		"✅ MANDATORY: Food naturally placed on surface with proper shadows and depth\n" +
		"✅ MANDATORY: Cohesive studio photograph - NOT a cutout or isolated element\n\n" +
		"📐 COMPOSITION VARIETY (avoid rigid centering):\n" +
		"• Use diverse professional food photography compositions\n" +
		"• Consider rule of thirds, off-center placement, dynamic angles\n" +
		"• Overhead shots, 45-degree angles, close-ups, cross-sections - vary naturally\n" +
		"• Avoid always centering single food items - be creative with placement\n" +
		"• Natural, editorial-style food photography composition\n\n"

	// 🔥 CRITICAL: 항상 강력한 시스템 프롬프트 먼저 (food photography 기본 품질 보장)
	finalPrompt = absoluteProhibitions + criticalHeader + mainInstruction + strings.Join(instructions, "\n") + compositionInstruction

	// 간결한 스타일 가이드
	categoryStyleGuide := ""

	// 사용자 프롬프트가 있으면 추가 (시스템 프롬프트 뒤에 배치하여 보완 역할)
	if userPrompt != "" {
		finalPrompt += "\n\n[ADDITIONAL USER REQUIREMENTS]:\n" + userPrompt + "\n" +
			"(Apply these additional requirements while maintaining the glossy professional food photography style above)\n\n"
	}

	// 마지막 필수 규칙들
	finalPrompt += categoryStyleGuide + criticalRules + aspectRatioInstruction

	return finalPrompt
}
