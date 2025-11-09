package cartoon

import (
	"fmt"
	"strings"
)

// ImageCategories - 카테고리별 이미지 분류 구조체
type PromptCategories struct {
	Model       []byte   // 모델 이미지 (최대 1장)
	Clothing    [][]byte // 의류 이미지 배열 (top, pants, outer)
	Accessories [][]byte // 악세사리 이미지 배열 (shoes, bag, accessory)
	Background  []byte   // 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Fashion 모듈 전용 프롬프트 생성
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasModel := categories.Model != nil
	hasClothing := len(categories.Clothing) > 0
	hasAccessories := len(categories.Accessories) > 0
	hasProducts := hasClothing || hasAccessories
	hasBackground := categories.Background != nil

	// 케이스별 메인 지시사항
	var mainInstruction string
	if hasModel {
		// 모델 있음 → 패션 에디토리얼
		mainInstruction = "[FASHION PHOTOGRAPHER'S DRAMATIC COMPOSITION]\n" +
			"You are a world-class fashion photographer shooting an editorial campaign.\n" +
			"The PERSON is the HERO - their natural proportions are SACRED and CANNOT be distorted.\n" +
			"The environment serves the subject, NOT the other way around.\n\n" +
			"Create ONE photorealistic photograph with DRAMATIC CINEMATIC STORYTELLING:\n" +
			"• The model wears ALL clothing and accessories in ONE complete outfit\n" +
			"• Dynamic pose and angle - NOT static or stiff\n" +
			"• Environmental storytelling - use the location for drama\n" +
			"• Directional lighting creates mood and depth\n" +
			"• This is a MOMENT full of energy and narrative\n\n"
	} else if hasProducts {
		// 프로덕트만 → 프로덕트 포토그래피
		mainInstruction = "[CINEMATIC PRODUCT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class product photographer creating editorial-style still life.\n" +
			"The PRODUCTS are the STARS - showcase them as beautiful objects with perfect details.\n" +
			"⚠️ CRITICAL: NO people or models in this shot - products only.\n\n" +
			"Create ONE photorealistic photograph with ARTISTIC STORYTELLING:\n" +
			"• Artistic arrangement of all items - creative composition\n" +
			"• Dramatic lighting that highlights textures and materials\n" +
			"• Environmental context (if location provided) or studio elegance\n" +
			"• Directional lighting creates depth and mood\n" +
			"• This is high-end product photography with cinematic quality\n\n"
	} else {
		// 배경만 → 환경 포토그래피
		mainInstruction = "[CINEMATIC ENVIRONMENTAL PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class environmental photographer capturing pure atmosphere.\n" +
			"The LOCATION is the SUBJECT - showcase its mood, scale, and character.\n" +
			"⚠️ CRITICAL: NO people, models, or products in this shot - environment only.\n\n" +
			"Create ONE photorealistic photograph with ATMOSPHERIC STORYTELLING:\n" +
			"• Dramatic composition that captures the location's essence\n" +
			"• Layers of depth - foreground, midground, background\n" +
			"• Directional lighting creates mood and drama\n" +
			"• This is cinematic environmental photography with narrative quality\n\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명
	if categories.Model != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (MODEL): This person's face, body shape, skin tone, and physical features - use EXACTLY this appearance", imageIndex))
		imageIndex++
	}

	if len(categories.Clothing) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (CLOTHING): ALL visible garments - tops, bottoms, dresses, outerwear, layers. The person MUST wear EVERY piece shown here", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (ACCESSORIES): ALL items - shoes, bags, hats, glasses, jewelry, watches. The person MUST wear/carry EVERY item shown here", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (LOCATION INSPIRATION): This shows the MOOD and ATMOSPHERE you should recreate - NOT a background to paste. Like a photographer's location scout photo, use this to understand the setting, lighting direction, and visual style. Generate a COMPLETELY NEW environment inspired by this reference that serves as the perfect stage for your subject", imageIndex))
		imageIndex++
	}

	// 시네마틱 구성 지시사항
	var compositionInstruction string

	// 케이스 1: 모델 이미지가 있는 경우 → 모델 착용 샷 (패션 에디토리얼)
	if hasModel {
		compositionInstruction = "\n[FASHION EDITORIAL COMPOSITION]\n" +
			"Generate ONE photorealistic film photograph showing the referenced model wearing the complete outfit (all clothing + accessories).\n" +
			"This is a high-end fashion editorial shoot with the model as the star."
	} else if hasProducts {
		// 케이스 2: 모델 없이 의상/액세서리만 → 프로덕트 샷 (오브젝트만)
		compositionInstruction = "\n[CINEMATIC PRODUCT PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic product photograph showcasing the clothing and accessories as OBJECTS.\n" +
			"⚠️ DO NOT add any people, models, or human figures.\n" +
			"⚠️ Display the items artistically arranged - like high-end product photography.\n"

		if hasBackground {
			compositionInstruction += "The products are placed naturally within the referenced environment - " +
				"as if styled by a professional photographer on location.\n" +
				"The items interact with the space (resting on surfaces, hanging naturally, artfully positioned)."
		} else {
			compositionInstruction += "Create a stunning studio product shot with professional lighting and composition.\n" +
				"The items are arranged artistically - flat lay, suspended, or elegantly displayed."
		}
	} else if hasBackground {
		// 케이스 3: 배경만 → 환경 사진
		compositionInstruction = "\n[CINEMATIC ENVIRONMENTAL PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic cinematic photograph of the referenced environment.\n" +
			"⚠️ DO NOT add any people, models, or products to this scene.\n" +
			"Focus on capturing the atmosphere, lighting, and mood of the location itself."
	} else {
		// 케이스 4: 아무것도 없는 경우 (에러 케이스)
		compositionInstruction = "\n[CINEMATIC COMPOSITION]\n" +
			"Generate a high-quality photorealistic image based on the references provided."
	}

	// 배경 관련 지시사항 - 모델이 있을 때만 추가
	if hasModel && hasBackground {
		// 모델 + 배경 케이스 → 환경 통합 지시사항
		compositionInstruction += " shot on location with environmental storytelling.\n\n" +
			"[PHOTOGRAPHER'S APPROACH TO LOCATION]\n" +
			"The photographer CHOSE this environment to complement the subject - not to overwhelm them.\n" +
			"🎬 Use the background reference as INSPIRATION ONLY:\n" +
			"   • Recreate the atmosphere, lighting mood, and setting type\n" +
			"   • Generate a NEW scene - do NOT paste or overlay the reference\n" +
			"   • The location serves as a STAGE for the subject's story\n\n" +
			"[ABSOLUTE PRIORITY: SUBJECT INTEGRITY]\n" +
			"⚠️ CRITICAL: The person's body proportions are UNTOUCHABLE\n" +
			"⚠️ DO NOT distort, stretch, compress, or alter the person to fit the frame\n" +
			"⚠️ The background adapts to showcase the subject - NEVER the reverse\n\n" +
			"[DRAMATIC ENVIRONMENTAL INTEGRATION]\n" +
			"✓ Subject positioned naturally in the space (standing, sitting, moving)\n" +
			"✓ Realistic ground contact with natural shadows\n" +
			"✓ Background elements create DEPTH - use foreground/midground/background layers\n" +
			"✓ Directional lighting from the environment enhances drama\n" +
			"✓ Environmental light wraps around the subject naturally\n" +
			"✓ Atmospheric perspective adds cinematic depth\n" +
			"✓ Shot composition tells a STORY - what is happening in this moment?\n\n" +
			"[TECHNICAL EXECUTION]\n" +
			"✓ Single camera angle - this is ONE photograph\n" +
			"✓ Film photography aesthetic with natural color grading\n" +
			"✓ Rule of thirds or dynamic asymmetric composition\n" +
			"✓ Depth of field focuses attention on the subject\n" +
			"✓ The environment and subject look like they exist in the SAME REALITY"
	} else if hasModel && !hasBackground {
		// 모델만 있고 배경 없음 → 스튜디오
		compositionInstruction += " in a cinematic studio setting with professional film lighting."
	}
	// 프로덕트 샷이나 배경만 있는 케이스는 위에서 이미 처리됨

	// 핵심 요구사항 - 케이스별로 다르게
	var criticalRules string

	// 공통 금지사항 - 모든 케이스에 적용
	commonForbidden := "\n\n[CRITICAL: ABSOLUTELY FORBIDDEN - THESE WILL CAUSE IMMEDIATE REJECTION]\n\n" +
		"⚠️ NO VERTICAL DIVIDING LINES - ZERO TOLERANCE:\n" +
		"❌ NO white vertical line down the center\n" +
		"❌ NO colored vertical line separating the image\n" +
		"❌ NO border or separator dividing left and right\n" +
		"❌ NO panel division or comic book split layout\n" +
		"❌ The image must be ONE continuous scene without ANY vertical dividers\n\n" +
		"⚠️ NO DUAL/SPLIT COMPOSITION - THIS IS NOT A COMPARISON IMAGE:\n" +
		"❌ DO NOT show the same character twice (left side vs right side)\n" +
		"❌ DO NOT create before/after, comparison, or variation layouts\n" +
		"❌ DO NOT duplicate the subject on both sides with different colors/styles\n" +
		"❌ This is ONE SINGLE MOMENT with ONE CHARACTER in ONE UNIFIED SCENE\n" +
		"❌ Left side and right side must be PART OF THE SAME ENVIRONMENT, not separate panels\n\n" +
		"⚠️ SINGLE UNIFIED COMPOSITION ONLY:\n" +
		"✓ ONE continuous background that flows naturally across the entire frame\n" +
		"✓ ONE character in ONE pose at ONE moment in time\n" +
		"✓ NO repeating elements on left and right sides\n" +
		"✓ The entire image is ONE COHESIVE PHOTOGRAPH - not a collage or split screen\n" +
		"✓ Background elements (buildings, sky, ground) must be CONTINUOUS with no breaks or seams\n"

	if hasModel {
		// 모델 있는 케이스 - 드라마틱 패션 에디토리얼 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS]\n" +
			"🎯 Person's body proportions are PERFECT and NATURAL - ZERO tolerance for distortion\n" +
			"🎯 The subject is the STAR - everything else supports their presence\n" +
			"🎯 Dramatic composition with ENERGY and MOVEMENT\n" +
			"🎯 Environmental storytelling - what's the narrative of this moment?\n" +
			"🎯 ALL clothing and accessories worn/carried simultaneously\n" +
			"🎯 Single cohesive photograph - looks like ONE shot from ONE camera\n" +
			"🎯 Film photography aesthetic - not digital, not flat\n" +
			"🎯 Dynamic framing - use negative space creatively\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"❌ ANY distortion of the person's proportions (stretched, compressed, squashed)\n" +
			"❌ Person looking pasted, floating, or artificially placed\n" +
			"❌ Static, boring, catalog-style poses\n" +
			"❌ Centered, symmetrical composition without drama\n" +
			"❌ Flat lighting that doesn't create mood"
	} else if hasProducts {
		// 프로덕트 샷 케이스 - 오브젝트 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS]\n" +
			"🎯 Showcase the products as beautiful OBJECTS with perfect details\n" +
			"🎯 Artistic arrangement - creative composition like high-end product photography\n" +
			"🎯 Dramatic lighting that highlights textures and materials\n" +
			"🎯 Environmental storytelling through product placement\n" +
			"🎯 ALL items displayed clearly and beautifully\n" +
			"🎯 Single cohesive photograph - ONE shot from ONE camera\n" +
			"🎯 Film photography aesthetic - not digital, not flat\n" +
			"🎯 Dynamic framing - use negative space and depth creatively\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"❌ ANY people, models, or human figures in the frame\n" +
			"❌ Products looking pasted or artificially placed\n" +
			"❌ Boring, flat catalog-style layouts\n" +
			"❌ Cluttered composition without focal point\n" +
			"❌ Flat lighting that doesn't create depth"
	} else {
		// 배경만 있는 케이스 - 환경 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS]\n" +
			"🎯 Capture the pure atmosphere and mood of the location\n" +
			"🎯 Dramatic composition with depth and visual interest\n" +
			"🎯 Environmental storytelling - what story does this place tell?\n" +
			"🎯 Film photography aesthetic - not digital, not flat\n" +
			"🎯 Dynamic framing - use negative space and layers creatively\n\n" +
			"[FORBIDDEN]\n" +
			"❌ DO NOT add people, models, or products to the scene\n" +
			"❌ Flat, boring composition without depth"
	}

	// 16:9 비율 전용 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "16:9" {
		if hasModel {
			// 모델이 있는 16:9 케이스
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC WIDE SHOT - DRAMATIC STORYTELLING]\n" +
				"This is a WIDE ANGLE shot - use the horizontal space for powerful visual storytelling.\n\n" +
				"🎬 DRAMATIC WIDE COMPOSITION:\n" +
				"✓ Subject positioned off-center (rule of thirds) creating dynamic tension\n" +
				"✓ Use the WIDTH to show environmental context and atmosphere\n" +
				"✓ Layers of depth - foreground elements, subject, background scenery\n" +
				"✓ Leading lines guide the eye to the subject\n" +
				"✓ Negative space creates breathing room and drama\n\n" +
				"🎬 SUBJECT INTEGRITY IN WIDE FRAME:\n" +
				"⚠️ The wide frame is NOT an excuse to distort proportions\n" +
				"⚠️ Person maintains PERFECT natural proportions - just smaller in frame if needed\n" +
				"⚠️ Use the space to tell a STORY, not to force-fit the subject\n\n" +
				"🎬 CINEMATIC EXECUTION:\n" +
				"✓ Directional lighting creates mood across the wide frame\n" +
				"✓ Atmospheric perspective - distant elements are hazier\n" +
				"✓ Film grain and natural color grading\n" +
				"✓ Depth of field emphasizes the subject while showing environment\n\n" +
				"GOAL: A breathtaking wide shot from a high-budget fashion editorial - \n" +
				"like Annie Leibovitz or Steven Meisel capturing a MOMENT of drama and beauty."
		} else if hasProducts {
			// 프로덕트 샷 16:9 케이스
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC PRODUCT SHOT]\n" +
				"This is a WIDE ANGLE product shot - use the horizontal space for artistic storytelling.\n\n" +
				"🎬 DRAMATIC WIDE PRODUCT COMPOSITION:\n" +
				"✓ Products positioned creatively using the full width\n" +
				"✓ Use the WIDTH to show environmental context and atmosphere\n" +
				"✓ Layers of depth - foreground, products, background elements\n" +
				"✓ Leading lines guide the eye to the key products\n" +
				"✓ Negative space creates elegance and breathing room\n\n" +
				"🎬 CINEMATIC EXECUTION:\n" +
				"✓ Directional lighting creates drama and highlights textures\n" +
				"✓ Atmospheric perspective adds depth\n" +
				"✓ Film grain and natural color grading\n" +
				"✓ Depth of field emphasizes products while showing environment\n\n" +
				"GOAL: A stunning wide product shot like high-end editorial still life photography."
		} else {
			// 배경만 있는 16:9 케이스
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC WIDE LANDSCAPE SHOT]\n" +
				"This is a WIDE ANGLE environmental shot - showcase the location's grandeur.\n\n" +
				"🎬 DRAMATIC LANDSCAPE COMPOSITION:\n" +
				"✓ Use the full WIDTH to capture the environment's scale and atmosphere\n" +
				"✓ Layers of depth - foreground, midground, background elements\n" +
				"✓ Leading lines guide the eye through the scene\n" +
				"✓ Asymmetric composition creates visual tension and interest\n" +
				"✓ Negative space emphasizes the mood and emptiness (if appropriate)\n\n" +
				"🎬 CINEMATIC EXECUTION:\n" +
				"✓ Directional lighting creates mood and drama\n" +
				"✓ Atmospheric perspective - distant elements are hazier\n" +
				"✓ Film grain and natural color grading\n" +
				"✓ Depth of field adds dimension to the scene\n\n" +
				"GOAL: A stunning environmental shot that tells a story without people - \n" +
				"like a cinematic establishing shot from a high-budget film."
		}
	}

	// 최종 조합: 시네마틱 지시사항 → 참조 이미지 설명 → 구성 요구사항 → 핵심 규칙 → 16:9 특화
	finalPrompt := mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + criticalRules + aspectRatioInstruction

	if userPrompt != "" {
		finalPrompt += "\n\n[ADDITIONAL STYLING]\n" + userPrompt
	}

	return finalPrompt
}
