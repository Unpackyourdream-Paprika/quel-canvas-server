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

	// isPreEdited: true일 때는 프리미엄 푸드 포토그래피 (자연스러운 고퀄리티)
	hasFood := len(categories.Food) > 0
	hasIngredient := len(categories.Ingredient) > 0
	hasProp := len(categories.Prop) > 0
	hasFoodItems := hasFood || hasIngredient || hasProp
	hasBackground := categories.Background != nil

	// 메인 지시사항 - 자연스럽고 맛있어 보이는 음식 사진
	var mainInstruction string
	if hasFoodItems {
		if hasBackground {
			mainInstruction = "[PREMIUM EDITORIAL FOOD PHOTOGRAPHY - NATURAL STYLE]\n" +
				"Create a stunning food photograph that looks naturally delicious.\n" +
				"The food should look FRESH and APPETIZING - like it was just prepared.\n\n" +
				"PHOTOGRAPHY STYLE:\n" +
				"• 45-DEGREE ANGLE - the most appetizing angle for food\n" +
				"• SHALLOW DEPTH OF FIELD - food sharp, background beautifully blurred (bokeh)\n" +
				"• WARM NATURAL LIGHTING - soft, diffused, like window light\n" +
				"• NATURAL GLOSS - food looks fresh and moist, not artificially oiled\n" +
				"• VIBRANT COLORS - saturated but realistic, appetite-triggering\n" +
				"• SHARP TEXTURE DETAIL - every grain, seed, and surface visible\n" +
				"• DIMENSIONAL LIGHTING - creates depth with soft shadows\n\n"
		} else {
			mainInstruction = "[PREMIUM STUDIO FOOD PHOTOGRAPHY - CLEAN STYLE]\n" +
				"Create a stunning food photograph with clean, professional look.\n" +
				"The food should look FRESH and APPETIZING - magazine cover quality.\n\n" +
				"PHOTOGRAPHY STYLE:\n" +
				"• 45-DEGREE ANGLE - the most appetizing angle for food\n" +
				"• CLEAN LIGHT BACKGROUND - white or soft neutral, not distracting\n" +
				"• SOFT DIFFUSED LIGHTING - creates gentle highlights and soft shadows\n" +
				"• NATURAL GLOSS - food looks fresh and moist from its own juices\n" +
				"• VIBRANT COLORS - saturated but realistic, true-to-life\n" +
				"• CRISP TEXTURE DETAIL - every grain, seed, crumb visible in sharp focus\n" +
				"• THREE-DIMENSIONAL - lighting creates depth and form\n\n"
		}
	} else if hasBackground {
		mainInstruction = "[ENVIRONMENTAL PHOTOGRAPHY]\n" +
			"Capture the atmosphere of this location.\n" +
			"NO food in this shot - environment only.\n\n"
	} else {
		mainInstruction = "[FOOD PHOTOGRAPHY]\n" +
			"Create a delicious-looking food photograph.\n\n"
	}

	var instructions []string
	imageIndex := 1

	// 카테고리별 설명 - 간결하게
	if len(categories.Food) > 0 {
		if len(categories.Food) == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (MAIN FOOD):\n"+
					"Recreate this EXACT food - same ingredients, same form.\n"+
					"Make it look FRESH: natural gloss, vibrant colors, sharp textures.\n"+
					"Every detail visible: grains, seeds, surfaces, layers.", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (FOOD - %d items in grid):\n"+
					"⚠️ Grid is for reference only - DO NOT recreate grid layout!\n"+
					"Arrange all %d items NATURALLY - clustered, overlapping, appetizing.\n"+
					"Each item: fresh gloss, vibrant color, sharp texture detail.", imageIndex, len(categories.Food), len(categories.Food)))
		}
		imageIndex++
	}

	if len(categories.Ingredient) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (INGREDIENTS):\n"+
				"Include these exact ingredients.\n"+
				"Fresh appearance: vibrant colors, natural moisture.", imageIndex))
		imageIndex++
	}

	if len(categories.Prop) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (GARNISHES/PROPS):\n"+
				"Include these garnishes/props.\n"+
				"Fresh herbs vibrant green, sauces glossy.", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (BACKGROUND):\n"+
				"Use this environment. Match lighting direction.\n"+
				"Food sharp, background with beautiful bokeh blur.", imageIndex))
		imageIndex++
	}

	// 텍스처 디테일 - 모든 음식에 적용되는 범용 기준
	textureDetail := "\n[UNIVERSAL TEXTURE STANDARD - ALL FOOD TYPES]\n" +
		"⚠️ These texture rules apply to ANY food, regardless of type:\n\n" +
		"📸 PHOTOREALISTIC TEXTURE REQUIREMENTS:\n" +
		"Every food item MUST show these qualities:\n\n" +
		"1. SURFACE DETAIL:\n" +
		"• Every surface shows MICRO-TEXTURE visible to the eye\n" +
		"• Grains, fibers, pores, seeds - all INDIVIDUALLY DISTINCT\n" +
		"• NO smooth, blended, or mushy appearances\n" +
		"• Think: 'I can see every tiny detail up close'\n\n" +
		"2. NATURAL SHEEN & MOISTURE:\n" +
		"• Fresh food has NATURAL GLOSSY SHEEN from its own moisture\n" +
		"• Light reflects off moist surfaces naturally\n" +
		"• Sauce/marinade coats ingredients with GLISTENING WET SHINE\n" +
		"• Oil and sauce create REFLECTIVE HIGHLIGHTS on surfaces\n" +
		"• NOT artificial glycerin - REAL food moisture from cooking\n" +
		"• Looks like it was JUST PREPARED moments ago, still HOT\n\n" +
		"3. COLOR VIBRANCY:\n" +
		"• Colors are INTENSELY SATURATED but REALISTIC\n" +
		"• GREEN onions/scallions: VIVID bright green, freshly cut\n" +
		"• ORANGE carrots: BRILLIANT saturated orange\n" +
		"• RED chili/sauce: DEEP rich red with glossy sheen\n" +
		"• WHITE sesame seeds: CREAM colored, each seed distinct\n" +
		"• CABBAGE: Fresh pale green with crisp appearance\n" +
		"• MEAT: Rich brown with caramelized edges, sauce coating\n" +
		"• NOT washed out, dull, or faded - PUNCHY vibrant colors\n\n" +
		"4. DEPTH & DIMENSION:\n" +
		"• Food has THREE-DIMENSIONAL presence with VOLUME\n" +
		"• Ingredients OVERLAP and LAYER naturally\n" +
		"• You can see DEPTH - items in front vs items behind\n" +
		"• Shadows and highlights create SCULPTURAL form\n" +
		"• Food looks PILED HIGH and ABUNDANT\n\n" +
		"5. SHARP FOCUS:\n" +
		"• Food is TACK SHARP - not soft or blurry\n" +
		"• SHALLOW DEPTH OF FIELD - main food sharp, background soft bokeh\n" +
		"• You can see every detail clearly on focused area\n" +
		"• Professional camera quality focus\n\n" +
		"6. GARNISH DETAILS:\n" +
		"• SESAME SEEDS: Each seed INDIVIDUALLY VISIBLE, scattered naturally\n" +
		"• GREEN ONIONS: Freshly sliced, bright green, placed on top\n" +
		"• HERBS: Vibrant green, fresh-looking, not wilted\n" +
		"• All garnishes look FRESHLY ADDED moments ago\n\n" +
		"7. SAUCE & COATING:\n" +
		"• Sauce GLISTENS and SHINES under light\n" +
		"• You can see sauce POOLING in crevices\n" +
		"• Sauce creates WET REFLECTIVE surface on ingredients\n" +
		"• Caramelization visible on edges - slightly darker, glossy\n\n" +
		"8. OIL COATING & CARAMELIZATION (COOKED FOOD):\n" +
		"• Cooking oil creates GOLDEN/ORANGE TINT on surfaces\n" +
		"• Oil coating makes surfaces GLISTEN with wet shine\n" +
		"• CARAMELIZED edges where food touched hot pan - darker brown, crispy\n" +
		"• CHAR MARKS on grilled/pan-fried surfaces - appetizing brown spots\n" +
		"• CRISPY TEXTURE visible on fried surfaces - bubbly, crunchy appearance\n" +
		"• MAILLARD REACTION visible - golden brown color from high heat\n" +
		"• Overall WARM GOLDEN TONE from cooking oils and heat\n\n" +
		"9. TOASTED/GRILLED SURFACE TEXTURE:\n" +
		"• Toasted surfaces have MATTE-TO-SLIGHT-SHEEN finish\n" +
		"• CHAR MARKS and BROWNING where it touched direct heat\n" +
		"• Slightly CRINKLED or BUBBLED texture from toasting/grilling\n" +
		"• Visible CRISPY EDGES that look crunchy and fragile\n" +
		"• Not soft or soggy - looks DRY-CRISPY on surface\n\n" +
		"❌ ABSOLUTE TEXTURE FAILURES:\n" +
		"• Plastic, clay, or CGI appearance = REJECTED\n" +
		"• Blended, mushy, or smeared textures = REJECTED\n" +
		"• Flat, matte, lifeless surfaces = REJECTED\n" +
		"• Soft focus or blurry food = REJECTED\n" +
		"• Washed out or dull colors = REJECTED\n" +
		"• Dry-looking food without natural moisture = REJECTED\n" +
		"• Raw/uncooked appearance when food should look cooked = REJECTED\n\n" +
		"✅ SUCCESS CRITERIA:\n" +
		"Viewer reaction: 'This looks SO DELICIOUS I can almost smell it'\n" +
		"Viewer reaction: 'I can see every grain/fiber/texture/seed'\n" +
		"Viewer reaction: 'The sauce looks so glossy and appetizing'\n" +
		"Viewer reaction: 'I can see the caramelization and char marks'\n" +
		"Viewer reaction: 'This is definitely a professional food photo'\n\n"

	// 라이팅 - 자연스러운 스타일
	lightingInstruction := "\n[LIGHTING - NATURAL EDITORIAL STYLE]\n" +
		"Soft, warm, dimensional lighting that makes food look delicious:\n\n" +
		"MAIN LIGHT:\n" +
		"• Soft diffused light from side/front (like window light)\n" +
		"• Creates gentle highlights on glossy surfaces\n" +
		"• Defines the three-dimensional form of the food\n\n" +
		"FILL:\n" +
		"• Subtle fill to open shadows\n" +
		"• Maintains depth and dimension\n" +
		"• Shadows are soft, not harsh black\n\n" +
		"RESULT:\n" +
		"• Food looks WARM and INVITING\n" +
		"• Natural-looking highlights, not artificial\n" +
		"• Depth and dimension, not flat\n" +
		"• Colors are TRUE and VIBRANT\n\n"

	// 금지사항 - 간결하게
	criticalForbidden := "\n\n[FORBIDDEN]\n" +
		"• NO split screen or grid layout\n" +
		"• NO black backgrounds\n" +
		"• NO borders or letterboxing\n\n"

	// 최우선 지시사항 - 전체 사진 퀄리티
	criticalHeader := "[CRITICAL - SCENE SETUP]\n\n" +
		"⚠️ BACKGROUND: Food directly on PLAIN WHITE/CREAM SURFACE - like a seamless paper backdrop.\n" +
		"⚠️ NO PLATES: Food is NOT on a plate, bowl, or dish. Food sits directly on the background.\n" +
		"⚠️ NO TABLEWARE: No plates, bowls, dishes, ceramics, or any container visible.\n\n" +
		"If food is shown ON A PLATE = WRONG.\n" +
		"If any dish/bowl/plate is visible = WRONG.\n\n" +
		"[CRITICAL - TEXTURE AND COLOR TEMPERATURE]\n\n" +
		"⚠️ COLOR TEMPERATURE: Must be WARM - golden/cream tones, NOT cold/gray/blue.\n" +
		"⚠️ RICE COLOR: WARM WHITE or CREAM color - like freshly cooked rice with sesame oil.\n" +
		"⚠️ RICE TEXTURE: Each grain INDIVIDUALLY VISIBLE and SEPARATED - you can count them.\n" +
		"⚠️ OVERALL: WARM, APPETIZING, GOLDEN tones throughout the entire image.\n\n" +
		"If rice looks GRAY or BLUE-TINTED = WRONG.\n" +
		"If rice grains are FUSED together = WRONG.\n" +
		"If image feels COLD or LIFELESS = WRONG.\n\n" +
		"[SCENE]\n" +
		"Clean product photo. Plain white/cream seamless backdrop. Food directly on surface. No plates.\n\n" +
		"[PHOTO STYLE]\n" +
		"Professional DSLR food photography. WARM color grading. Shallow depth of field.\n" +
		"Like a real photograph from a food magazine - NOT CGI, NOT 3D render.\n\n" +
		"⚠️⚠️⚠️ ABSOLUTE #1 PRIORITY - PROFESSIONAL FOOD PHOTOGRAPHY ⚠️⚠️⚠️\n\n" +
		"THIS IMAGE MUST BE INDISTINGUISHABLE FROM A REAL PHOTOGRAPH.\n" +
		"Shot by a professional food photographer with high-end equipment.\n" +
		"NOT CGI. NOT 3D render. NOT AI-looking. REAL CAMERA PHOTO.\n\n" +
		"🚨 CRITICAL TEXTURE REQUIREMENT 🚨\n\n" +
		"[HYPER-REALISTIC TEXTURE - MOST IMPORTANT]\n\n" +
		"RICE/GRAIN TEXTURE (CRITICAL):\n" +
		"• Color: WARM WHITE or CREAM - NOT gray, NOT blue-tinted\n" +
		"• Each grain INDIVIDUALLY VISIBLE - you can COUNT them\n" +
		"• Grains are SEPARATE, not fused together\n" +
		"• GLOSSY SHEEN from sesame oil - light reflects off surface\n" +
		"• Slightly TRANSLUCENT edges on each grain\n" +
		"• Looks FRESHLY COOKED and WARM\n\n" +
		"SEAWEED/NORI TEXTURE:\n" +
		"• Deep BLACK-GREEN color with natural sheen\n" +
		"• FIBROUS texture visible - not smooth plastic\n" +
		"• Natural WRINKLES and slight CRINKLES\n" +
		"• Matte-to-slight-sheen finish, NOT glossy plastic\n\n" +
		"PROTEIN/MEAT/FILLING TEXTURE:\n" +
		"• Individual FIBERS visible in meat\n" +
		"• NATURAL color variation - not uniform single color\n" +
		"• WET/MOIST appearance with sauce coating\n" +
		"• Visible SEASONING particles\n\n" +
		"VEGETABLE TEXTURE:\n" +
		"• CRISP cellular structure visible\n" +
		"• VIBRANT saturated colors - orange carrots, green pickles\n" +
		"• Fresh-cut appearance\n\n" +
		"❌ TEXTURE FAILURES = INSTANT REJECTION:\n" +
		"• Gray/blue/cold colored rice = REJECTED\n" +
		"• Rice grains fused together as blob = REJECTED\n" +
		"• Plastic/clay-like smooth surfaces = REJECTED\n" +
		"• CGI/3D rendered appearance = REJECTED\n" +
		"• Flat matte lifeless colors = REJECTED\n\n" +
		"📷 OVERALL IMAGE CHARACTERISTICS:\n\n" +
		"[FOOD STYLING & PRESENTATION]\n" +
		"• Food is PROFESSIONALLY STYLED - neat, organized, intentional placement\n" +
		"• Each component is CLEARLY SEPARATED and distinct in its own area\n" +
		"• Ingredients are NEATLY ARRANGED - not messy or haphazard\n" +
		"• Sauce drizzles are CLEAN and DELIBERATE - artistic zigzag patterns\n" +
		"• Garnishes placed with INTENTION - not randomly scattered\n" +
		"• Food presentation is CLEAN - no spills, smudges, or mess\n" +
		"• Overall appearance: POLISHED, REFINED, COMMERCIAL-READY\n" +
		"• Looks like a PROFESSIONAL FOOD STYLIST prepared this\n\n" +
		"[CLEAN STUDIO ENVIRONMENT]\n" +
		"• BRIGHT, CLEAN background - white, light gray, or soft neutral\n" +
		"• EVEN, SOFT lighting - no harsh shadows or dark areas\n" +
		"• Professional STUDIO QUALITY - not amateur phone photo\n" +
		"• Background is SIMPLE and NON-DISTRACTING\n" +
		"• Overall feeling: CLEAN, BRIGHT, APPETIZING\n\n" +
		"[FOCUS & DEPTH OF FIELD]\n" +
		"• SHALLOW DEPTH OF FIELD - background is SOFT BLURRED BOKEH\n" +
		"• Main food subject is TACK SHARP with crisp detail\n" +
		"• Smooth gradual transition from sharp foreground to blurry background\n" +
		"• Background objects are visible but SOFTLY OUT OF FOCUS\n" +
		"• Creates beautiful SEPARATION between subject and environment\n\n" +
		"[LIGHTING QUALITY]\n" +
		"• SOFT DIFFUSED STUDIO LIGHT - even and flattering\n" +
		"• Soft shadows that define shape without being harsh\n" +
		"• SPECULAR HIGHLIGHTS on glossy/wet surfaces - sauce, oil, moisture\n" +
		"• Overall BRIGHT and WELL-LIT - no dark, underexposed areas\n" +
		"• Light wraps around food creating THREE-DIMENSIONAL form\n\n" +
		"[COLOR RENDERING]\n" +
		"• Colors are RICH, SATURATED, and VIBRANT\n" +
		"• CLEAN color reproduction - true to life\n" +
		"• High color contrast - colors POP against each other\n" +
		"• NOT flat or desaturated - PUNCHY and appetizing\n" +
		"• Each ingredient's color is DISTINCT and recognizable\n\n" +
		"[COMPOSITION & FRAMING]\n" +
		"• Food fills frame ABUNDANTLY - generous portion visible\n" +
		"• CENTERED or well-balanced composition\n" +
		"• Clean negative space around the subject\n" +
		"• Eye naturally drawn to the food as HERO of image\n\n" +
		"[SENSE OF FRESHNESS]\n" +
		"• Food looks FRESHLY PREPARED - vibrant and appetizing\n" +
		"• Ingredients look VIBRANT and ALIVE, not old or wilted\n" +
		"• Sauce and oil GLISTEN as if just poured\n" +
		"• Overall feeling: 'This was just styled for a photoshoot'\n\n" +
		"❌ INSTANT REJECTION CRITERIA:\n" +
		"• Plastic/clay/CGI appearance\n" +
		"• Smooth, blended, mushy textures\n" +
		"• Flat, matte, lifeless surfaces\n" +
		"• Dull, washed-out colors\n" +
		"• Soft focus or blur ON FOOD (background blur is GOOD)\n" +
		"• Harsh flash lighting or dark shadows\n" +
		"• Messy, unorganized food presentation\n" +
		"• Dirty or messy presentation with spills\n" +
		"• Dark, dingy background\n\n"

	// 스타일 가이드
	styleGuide := "\n\n[STYLE GUIDE]\n" +
		"Premium editorial food photography. Natural warm lighting. " +
		"45-degree angle. Shallow depth of field with beautiful bokeh. " +
		"Sharp texture detail on every surface. Vibrant natural colors. " +
		"Fresh, appetizing appearance. Magazine cover quality.\n"

	// 최종 조합
	var finalPrompt string

	if userPrompt != "" {
		finalPrompt = criticalHeader + "[ADDITIONAL REQUIREMENTS]\n" + userPrompt + "\n\n"
	} else {
		finalPrompt = criticalHeader
	}

	finalPrompt += textureDetail + mainInstruction + strings.Join(instructions, "\n") + lightingInstruction + styleGuide + criticalForbidden

	return finalPrompt
}
