package eats

import (
	"fmt"
	"strings"
)

// ImageCategories - 카테고리별 이미지 분류 구조체 (음식용)
type PromptCategories struct {
	Model       []byte   // 메인 요리 이미지 (최대 1장)
	Clothing    [][]byte // 부재료/사이드 이미지 배열
	Accessories [][]byte // 토핑/가니쉬 이미지 배열
	Background  []byte   // 레스토랑/세팅 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Eats 모듈 전용 프롬프트 생성 (음식 사진)
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasMainDish := categories.Model != nil
	hasIngredients := len(categories.Clothing) > 0
	hasToppings := len(categories.Accessories) > 0
	hasFoodItems := hasIngredients || hasToppings
	hasRestaurant := categories.Background != nil

	// 케이스별 메인 지시사항
	var mainInstruction string
	if hasMainDish {
		// 메인 요리 있음 → 음식 에디토리얼
		mainInstruction = "[PROFESSIONAL FOOD PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class culinary photographer shooting for a Michelin-star restaurant editorial.\n" +
			"The DISH is the HERO - its natural colors, textures, and composition are SACRED and CANNOT be altered.\n" +
			"The plating and presentation are PERFECT - showcase them with editorial excellence.\n\n" +
			"Create ONE photorealistic photograph with HIGH-END CULINARY EDITORIAL STYLE:\n" +
			"• ONE beautifully plated dish - this is professional food photography\n" +
			"• AUTHENTIC FOOD STYLING - natural, appetizing, editorial presentation\n" +
			"• Perfect plating with ALL ingredients and toppings visible\n" +
			"• Professional restaurant photography aesthetic\n" +
			"• Directional lighting highlights textures, colors, and steam\n" +
			"• This is a MOMENT of culinary artistry and gastronomic excellence\n\n"
	} else if hasFoodItems {
		// 음식 재료만 → 재료 스틸라이프
		mainInstruction = "[CULINARY STILL LIFE PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class food photographer creating editorial-style ingredient photography.\n" +
			"The INGREDIENTS are the STARS - showcase them as fresh, beautiful objects with perfect details.\n" +
			"⚠️ CRITICAL: NO people or hands in this shot - ingredients only.\n\n" +
			"Create ONE photorealistic photograph with EDITORIAL FOOD STYLING:\n" +
			"• Artistic arrangement of fresh ingredients - creative composition\n" +
			"• Dramatic lighting that highlights textures and natural colors\n" +
			"• Restaurant kitchen or rustic table atmosphere\n" +
			"• This is high-end culinary still life with editorial quality\n\n"
	} else {
		// 배경만 → 레스토랑 환경 사진
		mainInstruction = "[RESTAURANT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class restaurant photographer capturing dining atmosphere.\n" +
			"The RESTAURANT is the SUBJECT - showcase its ambiance, design, and character.\n" +
			"⚠️ CRITICAL: NO people or food in this shot - environment only.\n\n" +
			"Create ONE photorealistic photograph with ATMOSPHERIC STORYTELLING:\n" +
			"• Dramatic composition that captures the restaurant's essence\n" +
			"• Interior design, lighting, and dining atmosphere\n" +
			"• Professional architectural photography of dining spaces\n" +
			"• This is editorial restaurant photography with cinematic quality\n\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명 (음식 용어로)
	if categories.Model != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (MAIN DISH - FOOD ONLY): This is a FOOD/DISH photograph showing plating, colors, textures, and presentation. This is NOT a person - it's FOOD. Recreate this DISH EXACTLY with the same culinary style and plating", imageIndex))
		imageIndex++
	}

	if len(categories.Clothing) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (INGREDIENTS/SIDES): ALL visible ingredients, side dishes, or components. The dish MUST include EVERY item shown here", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (TOPPINGS/GARNISH): ALL toppings, garnishes, sauces, herbs, or finishing touches. The dish MUST feature EVERY element shown here", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (RESTAURANT/SETTING INSPIRATION): This shows the ATMOSPHERE and DINING ENVIRONMENT you should recreate. Use this to understand the setting mood, lighting style, and restaurant ambiance. Generate a COMPLETELY NEW environment inspired by this reference", imageIndex))
		imageIndex++
	}

	// 구성 지시사항
	var compositionInstruction string

	// 케이스 1: 메인 요리가 있는 경우 → 플레이팅 샷
	if hasMainDish {
		compositionInstruction = "\n[CULINARY EDITORIAL COMPOSITION]\n" +
			"Generate ONE photorealistic culinary photograph showing the referenced dish with professional plating (including all ingredients + toppings).\n" +
			"This is high-end restaurant photography with the dish as the centerpiece."
	} else if hasFoodItems {
		// 케이스 2: 재료만 → 재료 스틸라이프
		compositionInstruction = "\n[INGREDIENT STILL LIFE PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic food photograph showcasing the ingredients as fresh, beautiful OBJECTS.\n" +
			"⚠️ DO NOT add any people, hands, or cooking in progress.\n" +
			"⚠️ Display the items artistically arranged - like high-end food magazine photography.\n"

		if hasRestaurant {
			compositionInstruction += "The ingredients are placed naturally within the referenced restaurant environment - " +
				"as if styled by a professional food photographer on location.\n" +
				"The items interact with the space (resting on wooden boards, marble counters, rustic tables)."
		} else {
			compositionInstruction += "Create a stunning culinary still life with professional lighting and composition.\n" +
				"The ingredients are arranged artistically - overhead flat lay, rustic board, or elegantly displayed."
		}
	} else if hasRestaurant {
		// 케이스 3: 레스토랑만 → 환경 사진
		compositionInstruction = "\n[RESTAURANT ENVIRONMENTAL PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic restaurant photograph of the referenced dining environment.\n" +
			"⚠️ DO NOT add any people or food to this scene.\n" +
			"Focus on capturing the atmosphere, interior design, and ambiance of the restaurant space."
	} else {
		// 케이스 4: 아무것도 없는 경우
		compositionInstruction = "\n[CULINARY PHOTOGRAPHY]\n" +
			"Generate a high-quality photorealistic food image based on the references provided."
	}

	// 배경 관련 지시사항 - 메인 요리가 있을 때만 추가
	if hasMainDish && hasRestaurant {
		compositionInstruction += " photographed in a restaurant setting with environmental storytelling.\n\n" +
			"[FOOD PHOTOGRAPHER'S APPROACH TO LOCATION]\n" +
			"The photographer CHOSE this dining environment to complement the dish - not to overwhelm it.\n" +
			"🎬 Use the restaurant reference as INSPIRATION ONLY:\n" +
			"   • Recreate the dining atmosphere, lighting mood, and interior style\n" +
			"   • Generate a NEW scene - do NOT paste or overlay the reference\n" +
			"   • The restaurant serves as a STAGE for the culinary presentation\n\n" +
			"[ABSOLUTE PRIORITY: DISH INTEGRITY]\n" +
			"⚠️ CRITICAL: The dish's colors and textures are UNTOUCHABLE\n" +
			"⚠️ DO NOT distort, over-saturate, or artificially enhance the food\n" +
			"⚠️ The plating and presentation are PERFECT - show them authentically\n\n" +
			"[PROFESSIONAL FOOD PHOTOGRAPHY INTEGRATION]\n" +
			"✓ Dish positioned naturally on table or serving surface\n" +
			"✓ Realistic table setting with natural shadows and reflections\n" +
			"✓ Restaurant elements create DEPTH - use foreground/background layers\n" +
			"✓ Directional lighting from windows or restaurant lights enhances textures\n" +
			"✓ Natural light or warm ambient lighting wraps around the dish\n" +
			"✓ Atmospheric perspective adds editorial depth\n" +
			"✓ Shot composition tells a STORY - this is dining as experience\n\n" +
			"[TECHNICAL EXECUTION]\n" +
			"✓ Single camera angle - this is ONE photograph\n" +
			"✓ Editorial food photography aesthetic with natural color grading\n" +
			"✓ Shallow depth of field focuses attention on the dish\n" +
			"✓ The environment and dish look appetizing and naturally integrated"
	} else if hasMainDish && !hasRestaurant {
		// 메인 요리만 있고 배경 없음 → 스튜디오 테이블
		compositionInstruction += " on a professional table setting with editorial food lighting."
	}

	// 핵심 요구사항 - 케이스별로 다르게
	var criticalRules string

	// 공통 금지사항
	commonForbidden := "\n\n[CRITICAL: ABSOLUTELY FORBIDDEN - THESE WILL CAUSE IMMEDIATE REJECTION]\n\n" +
		"⚠️ NO VERTICAL DIVIDING LINES - ZERO TOLERANCE:\n" +
		"❌ NO white vertical line down the center\n" +
		"❌ NO colored vertical line separating the image\n" +
		"❌ NO border or separator dividing left and right\n" +
		"❌ NO panel division or split layout\n" +
		"❌ The image must be ONE continuous scene without ANY vertical dividers\n\n" +
		"⚠️ NO DUAL/SPLIT COMPOSITION - THIS IS NOT A COMPARISON IMAGE:\n" +
		"❌ DO NOT show the same dish twice (left side vs right side)\n" +
		"❌ DO NOT create before/after, comparison, or variation layouts\n" +
		"❌ DO NOT duplicate the subject on both sides\n" +
		"❌ This is ONE SINGLE MOMENT with ONE DISH in ONE UNIFIED SCENE\n" +
		"❌ Left side and right side must be PART OF THE SAME TABLE, not separate panels\n\n" +
		"⚠️ SINGLE UNIFIED COMPOSITION ONLY:\n" +
		"✓ ONE continuous background that flows naturally across the entire frame\n" +
		"✓ ONE dish in ONE presentation at ONE moment in time\n" +
		"✓ NO repeating elements on left and right sides\n" +
		"✓ The entire image is ONE COHESIVE PHOTOGRAPH - not a collage or split screen\n" +
		"✓ Background elements (table, walls, windows) must be CONTINUOUS with no breaks or seams\n"

	if hasMainDish {
		// 메인 요리 있는 케이스 - 음식 에디토리얼 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS - CULINARY EDITORIAL]\n" +
			"🎯 ONLY ONE DISH in the photograph - this is professional plating photography\n" +
			"🎯 AUTHENTIC FOOD COLORS - natural, appetizing, NOT over-saturated or artificial\n" +
			"🎯 PROFESSIONAL PLATING - elegant presentation, chef-quality composition\n" +
			"🎯 FOOD TEXTURES VISIBLE - show steam, moisture, freshness, natural appeal\n" +
			"🎯 Dish's natural appearance is PERFECT - ZERO tolerance for distortion or fake enhancement\n" +
			"🎯 The dish is the STAR - everything else supports its presentation\n" +
			"🎯 Michelin-star restaurant aesthetic - high-end culinary editorial, NOT fast food catalog\n" +
			"🎯 Dramatic composition with ELEGANCE and APPETITE APPEAL\n" +
			"🎯 Gastronomic storytelling - what's the dining experience of this moment?\n" +
			"🎯 ALL ingredients and toppings plated simultaneously\n" +
			"🎯 Single cohesive photograph - looks like ONE shot from ONE camera\n" +
			"🎯 Editorial food photography aesthetic - warm, natural, appetizing\n" +
			"🎯 Dynamic framing - use negative space and shallow depth of field\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"❌ TWO or more identical dishes in the frame - this is NOT a catalog grid\n" +
			"❌ Multiple portions, duplicate plating, or buffet-style arrangement\n" +
			"❌ ANY distortion of the food's colors (over-saturated, neon, fake-looking)\n" +
			"❌ Food looking plastic, artificial, or CGI-rendered\n" +
			"❌ Hands, people, or cooking in progress visible in frame\n" +
			"❌ Messy, unappetizing, or amateur plating\n" +
			"❌ Fast food catalog style - this is FINE DINING editorial\n" +
			"❌ Centered, boring composition without depth\n" +
			"❌ Flat lighting that doesn't enhance food textures"
	} else if hasFoodItems {
		// 재료 케이스 - 음식 스틸라이프 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS - INGREDIENT PHOTOGRAPHY]\n" +
			"🎯 Showcase the ingredients as fresh, beautiful OBJECTS with perfect details\n" +
			"🎯 Artistic arrangement - creative composition like high-end food magazine\n" +
			"🎯 Dramatic lighting that highlights natural textures and colors\n" +
			"🎯 Fresh, organic, appetizing appearance - peak ingredient quality\n" +
			"🎯 ALL items displayed clearly and beautifully\n" +
			"🎯 Single cohesive photograph - ONE shot from ONE camera\n" +
			"🎯 Editorial food styling aesthetic - natural, rustic, elegant\n" +
			"🎯 Dynamic framing - use negative space and depth creatively\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"❌ ANY people, hands, or cooking in progress in the frame\n" +
			"❌ Ingredients looking artificial, plastic, or fake\n" +
			"❌ Boring, flat catalog-style layouts\n" +
			"❌ Cluttered composition without focal point\n" +
			"❌ Flat lighting that doesn't create appetite appeal"
	} else {
		// 레스토랑만 있는 케이스 - 환경 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS - RESTAURANT PHOTOGRAPHY]\n" +
			"🎯 Capture the pure atmosphere and dining ambiance\n" +
			"🎯 Dramatic composition with architectural depth and visual interest\n" +
			"🎯 Environmental storytelling - what story does this dining space tell?\n" +
			"🎯 Professional interior photography aesthetic\n" +
			"🎯 Dynamic framing - use negative space and layers creatively\n\n" +
			"[FORBIDDEN]\n" +
			"❌ DO NOT add people or food to the scene\n" +
			"❌ Flat, boring composition without depth"
	}

	// aspect ratio별 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "1:1" {
		if hasMainDish {
			// 메인 요리가 있는 1:1 케이스 (정사각형 - 음식 에디토리얼)
			aspectRatioInstruction = "\n\n[1:1 SQUARE CULINARY EDITORIAL - OVERHEAD/45-DEGREE ANGLE]\n" +
				"This is a SQUARE format - perfect for Instagram-style food photography and overhead plating shots.\n\n" +
				"🎬 CAMERA ANGLE & PERSPECTIVE:\n" +
				"✓ OVERHEAD (bird's eye view) - camera directly above dish looking straight down\n" +
				"✓ OR 45-DEGREE ANGLE - camera at diagonal angle showing dish height and depth\n" +
				"✓ NATURAL PERSPECTIVE - no distortion, food has correct proportions\n" +
				"✓ STRAIGHT FRAMING - camera level, not tilted or dutch angle\n\n" +
				"🎬 SQUARE PLATING COMPOSITION:\n" +
				"✓ Balanced composition utilizing the square format\n" +
				"✓ Dish centered or using rule of thirds for visual interest\n" +
				"✓ Surrounding table elements (cutlery, napkin, drink) create context\n" +
				"✓ Negative space around the dish creates elegance\n\n" +
				"🎬 PLATING PHOTOGRAPHY EXECUTION:\n" +
				"✓ Directional lighting from above or side highlights textures\n" +
				"✓ Natural food photography aesthetic with warm tones\n" +
				"✓ Shallow depth of field emphasizes the dish\n" +
				"✓ Dynamic styling - NOT static or boring\n\n" +
				"GOAL: A stunning square food photograph like Bon Appétit or Kinfolk magazine - \n" +
				"showcasing the dish's beauty with editorial sophistication and proper perspective."
		} else if hasFoodItems {
			// 재료 샷 1:1 케이스
			aspectRatioInstruction = "\n\n[1:1 SQUARE INGREDIENT SHOT]\n" +
				"This is a SQUARE format ingredient shot - balanced and elegant.\n\n" +
				"🎬 CAMERA ANGLE:\n" +
				"✓ OVERHEAD flat lay - camera directly above ingredients\n" +
				"✓ NATURAL PERSPECTIVE - no distortion\n\n" +
				"🎬 SQUARE INGREDIENT COMPOSITION:\n" +
				"✓ Ingredients arranged to utilize the square space creatively\n" +
				"✓ Overhead flat lay or rustic board presentation\n" +
				"✓ Balanced composition with artistic arrangement\n" +
				"✓ Negative space creates visual breathing room\n\n" +
				"🎬 EXECUTION:\n" +
				"✓ Directional lighting creates drama and highlights freshness\n" +
				"✓ Natural food photography aesthetic\n\n" +
				"GOAL: A stunning square ingredient shot."
		} else {
			// 레스토랑만 있는 1:1 케이스
			aspectRatioInstruction = "\n\n[1:1 SQUARE RESTAURANT SHOT]\n" +
				"This is a SQUARE environmental shot - balanced composition.\n\n" +
				"🎬 SQUARE COMPOSITION:\n" +
				"✓ Balanced framing utilizing the square format\n" +
				"✓ Architectural layers create depth\n\n" +
				"🎬 EXECUTION:\n" +
				"✓ Restaurant lighting creates ambiance\n" +
				"✓ Professional interior photography aesthetic\n\n" +
				"GOAL: A stunning square restaurant shot."
		}
	} else if aspectRatio == "16:9" || aspectRatio == "9:16" {
		if hasMainDish {
			// 메인 요리가 있는 wide/tall 케이스
			var formatDesc string
			if aspectRatio == "16:9" {
				formatDesc = "WIDE HORIZONTAL format - perfect for editorial food photography spreads"
			} else {
				formatDesc = "TALL VERTICAL format - perfect for social media food photography"
			}

			aspectRatioInstruction = fmt.Sprintf("\n\n[%s CULINARY EDITORIAL - 45-DEGREE/EYE-LEVEL ANGLE]\n", aspectRatio) +
				fmt.Sprintf("This is a %s.\n\n", formatDesc) +
				"🎬 CAMERA ANGLE & PERSPECTIVE:\n" +
				"✓ 45-DEGREE ANGLE - camera at diagonal showing dish depth and layers\n" +
				"✓ OR EYE-LEVEL ANGLE - camera at table height for dramatic perspective\n" +
				"✓ NATURAL PERSPECTIVE - no distortion, food has correct proportions\n" +
				"✓ STRAIGHT FRAMING - camera level, not tilted\n" +
				"✓ REALISTIC DEPTH - proper shallow depth of field\n\n" +
				"🎬 FOOD PHOTOGRAPHY COMPOSITION:\n" +
				"✓ Dish positioned naturally on table/surface with proper depth\n" +
				"✓ Background elements (table, restaurant ambiance) add context\n" +
				"✓ Leading lines and layers create visual interest\n" +
				"✓ Negative space creates breathing room\n\n" +
				"🎬 PROFESSIONAL EXECUTION:\n" +
				"✓ Directional lighting from window or side highlights textures\n" +
				"✓ Natural food photography aesthetic with warm, appetizing tones\n" +
				"✓ Shallow depth of field emphasizes the dish\n" +
				"✓ Professional editorial style - looks DELICIOUS and mouth-watering\n\n" +
				"GOAL: A stunning food photograph with proper perspective and appetizing presentation - \n" +
				"like high-end culinary magazine editorial with correct camera angle."
		} else if hasFoodItems {
			aspectRatioInstruction = fmt.Sprintf("\n\n[%s INGREDIENT SHOT]\n", aspectRatio) +
				"🎬 CAMERA ANGLE:\n" +
				"✓ OVERHEAD or 45-DEGREE angle showing ingredients\n" +
				"✓ NATURAL PERSPECTIVE - no distortion\n\n" +
				"GOAL: Beautiful ingredient photography with proper framing."
		} else {
			aspectRatioInstruction = fmt.Sprintf("\n\n[%s RESTAURANT SHOT]\n", aspectRatio) +
				"GOAL: Professional restaurant interior photography."
		}
	}

	// ⚠️ 최우선 지시사항 - 맨 앞에 배치
	criticalHeader := "⚠️⚠️⚠️ CRITICAL REQUIREMENTS - ABSOLUTE PRIORITY - IMAGE WILL BE REJECTED IF NOT FOLLOWED ⚠️⚠️⚠️\n\n" +
		"[MANDATORY - CAMERA ANGLE & PERSPECTIVE]:\n" +
		"🚨 PROPER FOOD PHOTOGRAPHY ANGLE - use 45-degree angle, overhead (bird's eye), or eye-level\n" +
		"🚨 NATURAL PERSPECTIVE - food must have correct proportions, NOT distorted or warped\n" +
		"🚨 STRAIGHT CAMERA - no extreme dutch angles or tilted perspectives\n" +
		"🚨 PROFESSIONAL FRAMING - dish positioned naturally on table/surface, NOT floating or fake\n" +
		"🚨 REALISTIC DEPTH - proper shallow depth of field, background slightly blurred\n\n" +
		"[MANDATORY - AUTHENTIC FOOD PHOTOGRAPHY]:\n" +
		"🚨 100% PHOTOREALISTIC - must look like real food photography, NOT CGI or illustration\n" +
		"🚨 NATURAL FOOD COLORS - appetizing, authentic, NOT over-saturated or fake-looking\n" +
		"🚨 REAL FOOD TEXTURES - show moisture, steam, freshness, natural appeal\n" +
		"🚨 DELICIOUS-LOOKING - food must look APPETIZING, mouth-watering, tempting to eat\n" +
		"🚨 NO CARTOON, NO PAINTING, NO ILLUSTRATION STYLE - this is editorial food photography\n" +
		"🚨 Professional restaurant photography aesthetic - Michelin-star quality\n\n" +
		"[MANDATORY - PROFESSIONAL PLATING]:\n" +
		"🚨 CHEF-QUALITY PRESENTATION - elegant, sophisticated, high-end plating\n" +
		"🚨 ALL ingredients visible and beautifully arranged\n" +
		"🚨 Professional food styling - NOT messy or amateur\n" +
		"🚨 This is FINE DINING editorial - NOT fast food catalog\n\n" +
		"[FORBIDDEN - IMAGE WILL BE REJECTED]:\n" +
		"❌ NO distorted perspective, warped angles, or unnatural proportions\n" +
		"❌ NO extreme dutch angles, crooked framing, or tilted camera\n" +
		"❌ NO floating food, pasted-looking dishes, or fake composition\n" +
		"❌ NO cartoon style, illustration, painting, or artistic interpretation\n" +
		"❌ NO over-saturated neon colors or fake CGI food appearance\n" +
		"❌ NO left-right split, NO side-by-side layout, NO duplicate dishes\n" +
		"❌ NO grid, NO collage, NO comparison view, NO before/after layout\n" +
		"❌ NO vertical dividing line, NO center split\n" +
		"❌ NO white/gray borders, NO letterboxing, NO empty margins\n" +
		"❌ ONLY ONE DISH in the photograph - NO multiple identical portions\n\n" +
		"[REQUIRED - MUST GENERATE THIS WAY]:\n" +
		"✓ PROPER FOOD PHOTOGRAPHY ANGLE - 45-degree, overhead, or eye-level camera position\n" +
		"✓ NATURAL PERSPECTIVE - correct proportions, realistic depth, proper framing\n" +
		"✓ ONE single photograph taken with ONE camera shutter\n" +
		"✓ ONE unified moment in time - NOT multiple dishes combined\n" +
		"✓ ONLY ONE DISH/SERVING in the entire frame\n" +
		"✓ PHOTOREALISTIC food photography - looks like a real restaurant photograph\n" +
		"✓ Natural, appetizing colors - warm, inviting, DELICIOUS-looking\n" +
		"✓ Professional editorial style - Bon Appétit, Kinfolk, Saveur magazine quality\n" +
		"✓ Natural asymmetric composition - left side different from right side\n\n"

	// 최종 조합
	var finalPrompt string

	// 1️⃣ 크리티컬 요구사항을 맨 앞에 배치
	if userPrompt != "" {
		finalPrompt = criticalHeader + "[ADDITIONAL STYLING]\n" + userPrompt + "\n\n"
	} else {
		finalPrompt = criticalHeader
	}

	// 2️⃣ 나머지 지시사항들
	finalPrompt += mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + criticalRules + aspectRatioInstruction

	return finalPrompt
}
