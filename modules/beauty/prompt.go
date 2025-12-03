package beauty

import (
	"fmt"
	"log"
	"strings"
)

// ImageCategories - Beauty 카테고리별 이미지 분류 구조체 (화장품 전용)
type PromptCategories struct {
	Model       []byte   // 모델 이미지 (최대 1장) - Beauty에서는 인물 뷰티 샷용
	Products    [][]byte // 화장품/제품 이미지 배열 (lipstick, cream, bottle 등) - Beauty 전용
	Accessories [][]byte // 악세사리 이미지 배열 (brush, tool 등) - Beauty 보조 도구
	Background  []byte   // 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Beauty 모듈 전용 프롬프트 생성
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasModel := categories.Model != nil
	hasProducts := len(categories.Products) > 0 // Beauty 전용: Products 필드 직접 확인
	hasBackground := categories.Background != nil

	// 디버그 로그 추가
	log.Printf("🔍 [Beauty Prompt] Model:%v, Products:%d, Accessories:%d, BG:%v",
		hasModel, len(categories.Products), len(categories.Accessories), hasBackground)

	// 케이스별 메인 지시사항
	var mainInstruction string
	if hasModel {
		// 모델 있음 → 뷰티 포트레이트 (얼굴 클로즈업)
		mainInstruction = "[BEAUTY PHOTOGRAPHER'S CLOSE-UP PORTRAIT]\n" +
			"You are a world-class beauty photographer specializing in cosmetic editorial and makeup photography.\n" +
			"The FACE is the HERO - skin texture, makeup details, and facial features are SACRED.\n" +
			"⚠️ CRITICAL: This is a BEAUTY SHOT, NOT a fashion shot.\n" +
			"⚠️ MANDATORY: CLOSE-UP PORTRAIT ONLY - face and shoulders composition.\n" +
			"⚠️ FORBIDDEN: NO full body shots, NO fashion model poses, NO runway looks.\n\n" +
			"Create ONE photorealistic beauty photograph with FLAWLESS SKIN DETAIL:\n" +
			"• CLOSE-UP PORTRAIT: Face fills most of the frame (head and shoulders only)\n" +
			"• Focus on facial features, skin texture, makeup details\n" +
			"• Soft, flattering lighting for beauty photography (butterfly or loop lighting)\n" +
			"• Professional studio beauty photography composition\n" +
			"• High-end cosmetic editorial quality\n" +
			"• This is about BEAUTY and MAKEUP, not fashion or outfits\n\n"
	} else if hasProducts {
		// 프로덕트만 → 뷰티 프로덕트 (화장품/제품) - 개수에 따라 동적 프롬프트
		productCount := len(categories.Products)
		var productCountInstruction string

		// Check if user prompt indicates a grid or multiple products (for pre-merged inputs)
		isGridInput := false
		lowerPrompt := strings.ToLower(userPrompt)
		if strings.Contains(lowerPrompt, "grid") || 
		   strings.Contains(lowerPrompt, "4 products") || 
		   strings.Contains(lowerPrompt, "four products") ||
		   strings.Contains(lowerPrompt, "multiple products") {
			isGridInput = true
		}

		switch productCount {
		case 1:
			if isGridInput {
				productCountInstruction = "⚠️ CRITICAL: The reference image is a GRID containing MULTIPLE products.\n" +
					"⚠️ YOU MUST SHOW ALL PRODUCTS visible in the reference grid.\n" +
					"⚠️ Do not select just one. Show the entire set as presented.\n"
			} else {
				// Allow flexibility if it might be a grid but not explicitly stated, 
				// but prioritize single product if it looks like one.
				productCountInstruction = "⚠️ CRITICAL: Show the product(s) exactly as shown in the reference.\n" +
					"⚠️ If the reference is a GRID of multiple items, SHOW ALL OF THEM.\n" +
					"⚠️ If it is a single item, show exactly one.\n"
			}
		case 2:
			productCountInstruction = "⚠️ CRITICAL: Show EXACTLY 2 (TWO) products - both items from the reference must appear.\n" +
				"⚠️ DO NOT add extra products. DO NOT omit any. EXACTLY 2 products.\n"
		case 3:
			productCountInstruction = "⚠️ CRITICAL: Show EXACTLY 3 (THREE) products - all three items from the reference must appear.\n" +
				"⚠️ DO NOT add extra products. DO NOT omit any. EXACTLY 3 products.\n"
		case 4:
			productCountInstruction = "⚠️ CRITICAL: Show EXACTLY 4 (FOUR) products - all four items from the reference must appear.\n" +
				"⚠️ DO NOT add extra products. DO NOT omit any. EXACTLY 4 products.\n" +
				"⚠️ ARRANGE them naturally in the scene (e.g., a group composition), NOT as a 2x2 grid.\n"
		default:
			productCountInstruction = fmt.Sprintf("⚠️ CRITICAL: Show EXACTLY %d products - ALL items from the reference must appear.\n"+
				"⚠️ DO NOT add extra products. DO NOT omit any. EXACTLY %d products.\n", productCount, productCount)
		}

		mainInstruction = "[BEAUTY PRODUCT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class cosmetic product photographer.\n" +
			"The BEAUTY PRODUCTS are the STARS - showcase them as premium cosmetics.\n" +
			"⚠️ CRITICAL: NO people or models in this shot - beauty products only.\n" +
			productCountInstruction +
			"\nCreate ONE photorealistic photograph with COSMETIC ELEGANCE:\n" +
			"• Artistic arrangement of beauty products (lipsticks, makeup, skincare)\n" +
			"• Soft, diffused lighting that highlights product details\n" +
			"• Premium cosmetic brand photography style\n" +
			"• Clean, elegant composition\n" +
			"• This is high-end beauty product photography\n\n"
	} else {
		// 배경만 → 환경 포토그래피
		mainInstruction = "[BEAUTY ENVIRONMENT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a photographer capturing a serene beauty photography backdrop.\n" +
			"The LOCATION creates a MOOD for beauty photography - soft, elegant, clean.\n" +
			"⚠️ CRITICAL: NO people, models, or products in this shot - environment only.\n\n" +
			"Create ONE photorealistic photograph with SOFT ATMOSPHERIC MOOD:\n" +
			"• Soft, flattering lighting suitable for beauty photography\n" +
			"• Clean, elegant composition\n" +
			"• Subtle depth and layers\n" +
			"• This creates a perfect backdrop for beauty shots\n\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명 (Beauty-specific)
	if categories.Model != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (MODEL FACE): This person's FACE, facial features, skin tone, bone structure, and expression - use EXACTLY this appearance. Focus on face and shoulders only for beauty closeup", imageIndex))
		imageIndex++
	}

	if len(categories.Products) > 0 {
		productCount := len(categories.Products)
		if hasModel {
			// 모델 + 제품: 메이크업 레퍼런스로 사용
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (MAKEUP/COSMETIC REFERENCE): These beauty products show the makeup style and color palette to apply to the model's face - lipstick shade, eyeshadow tones, skin finish. Use these as inspiration for the model's makeup look, NOT as products to place in the shot", imageIndex))
		} else {
			// 제품만: 순수 제품 촬영 - 개수 명시
			var countDesc string
			// Check if user prompt indicates a grid or multiple products
			lowerPrompt := strings.ToLower(userPrompt)
			isGridInput := strings.Contains(lowerPrompt, "grid") ||
				strings.Contains(lowerPrompt, "4 products") ||
				strings.Contains(lowerPrompt, "four products") ||
				strings.Contains(lowerPrompt, "multiple products")
			switch productCount {
			case 1:
				if isGridInput {
					countDesc = "The reference shows multiple products in a grid. Show ALL of them arranged naturally together."
				} else {
					countDesc = "The reference shows the product. Show it naturally in the scene."
				}
			case 2:
				countDesc = "The reference shows 2 products (in a grid). Arrange these TWO products naturally together in the scene. DO NOT copy the grid layout."
			case 3:
				countDesc = "The reference shows 3 products (in a grid). Arrange these THREE products naturally together as a group. DO NOT copy the grid layout."
			case 4:
				countDesc = "The reference shows 4 products (in a grid). Arrange these FOUR products naturally together as a group. DO NOT copy the grid layout."
			default:
				countDesc = fmt.Sprintf("The reference shows %d products. Arrange ALL %d products naturally together in the scene. DO NOT copy the grid layout.", productCount, productCount)
			}
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (BEAUTY PRODUCTS - %d ITEMS): %s These are cosmetic items to showcase as the main subject. Display ONLY these products with premium cosmetic photography style. These are OBJECTS to be photographed, not makeup to apply.", imageIndex, productCount, countDesc))
		}
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (BEAUTY ACCESSORIES): Visible accessories in closeup (earrings, necklace, headpiece) that complement the beauty portrait - include ONLY items visible in head and shoulders frame", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (ENVIRONMENT STYLE GUIDE): Use this image as a STYLE REFERENCE to GENERATE a new matching environment. Do not copy it pixel-for-pixel. Re-create this atmosphere in 3D.", imageIndex))
		imageIndex++
	}

	// 시네마틱 구성 지시사항
	var compositionInstruction string

	// 케이스 1: 모델 이미지가 있는 경우 → 뷰티 클로즈업 (얼굴 중심)
	if hasModel {
		compositionInstruction = "\n[BEAUTY CLOSE-UP PORTRAIT COMPOSITION]\n" +
			"Generate ONE photorealistic beauty portrait showing the referenced model's FACE AND SHOULDERS ONLY.\n" +
			"⚠️ CRITICAL: This is a BEAUTY SHOT, NOT a fashion or full body shot.\n" +
			"⚠️ MANDATORY: CLOSE-UP composition - face fills 60-80% of the frame.\n" +
			"⚠️ FORBIDDEN: NO full body, NO outfit showcase, NO fashion poses.\n\n" +
			"Focus on:\n" +
			"• Facial features and expressions\n" +
			"• Skin texture and quality\n" +
			"• Makeup details (eyes, lips, cheeks)\n" +
			"• Head and shoulders composition only\n" +
			"• Soft, flattering beauty lighting\n" +
			"This is high-end cosmetic editorial photography with the face as the star."
	} else if hasProducts {
		// 케이스 2: 모델 없이 제품만 → 뷰티 프로덕트 샷 (화장품/코스메틱)
		compositionInstruction = "\n[BEAUTY PRODUCT PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic beauty product photograph showcasing cosmetics and beauty items as OBJECTS.\n" +
			"⚠️ CRITICAL: DO NOT add any people, models, or human figures.\n" +
			"⚠️ CRITICAL: DO NOT add hands, fingers, or any body parts holding products.\n" +
			"⚠️ CRITICAL: NO human faces, NO portraits, NO makeup application shots - PRODUCTS ONLY.\n" +
			"⚠️ Display the beauty products artistically arranged - like high-end cosmetic advertising photography.\n" +
			"⚠️ USE ONLY the provided product references; do NOT invent extra products or variants."

		if hasBackground {
			compositionInstruction += "The beauty products are placed in a FULLY RE-RENDERED 3D ENVIRONMENT inspired by the background reference.\n" +
				"⚠️ CRITICAL: The background reference is ONLY for mood, colors, and texture. IT IS NOT A TEMPLATE.\n" +
				"⚠️ YOU HAVE FULL CREATIVE FREEDOM to change the background layout, geometry, and perspective to best fit the products.\n" +
				"⚠️ DO NOT try to match the reference background's shape or object placement. CREATE A NEW SCENE.\n" +
				"⚠️ GLOBAL ILLUMINATION: The light source from the generated environment must interact realistically with the products.\n" +
				"⚠️ AMBIENT OCCLUSION: Create deep, realistic contact shadows where the products touch the surface to avoid the 'floating sticker' look.\n" +
				"⚠️ LIGHT WRAP: Let the background light softly wrap around the product edges to blend them naturally into the scene.\n" +
				"⚠️ COLOR BLEED: Allow the background colors (e.g., green from leaves) to subtly reflect on the product surfaces for true integration.\n" +
				"⚠️ The products and the new background must be rendered TOGETHER as one single 3D scene.\n" +
				"This is a completely NEW photograph where the background is re-created to perfectly fit the products."
		} else {
			compositionInstruction += "Create a stunning studio beauty product shot with soft, diffused lighting and clean composition.\n" +
				"The cosmetic items are arranged artistically - flat lay, clean display, or elegantly positioned with beauty editorial aesthetic.\n" +
				"Think premium beauty brand campaigns (Estée Lauder, La Mer, Tom Ford Beauty) - pure product elegance, zero human presence."
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
		// 모델 + 배경 케이스 → 뷰티 환경 통합
		compositionInstruction += " shot on location with the referenced background environment.\n\n" +
			"[BEAUTY PORTRAIT WITH BACKGROUND]\n" +
			"The referenced background image shows the EXACT setting to use.\n" +
			"⚠️ CRITICAL: Even with a background, this is still a CLOSE-UP BEAUTY PORTRAIT.\n" +
			"⚠️ MANDATORY: Face and shoulders composition - NOT full body.\n\n" +
			"🎬 Use the background reference as the ACTUAL location:\n" +
			"   • Use the actual colors, elements, and atmosphere from the background reference\n" +
			"   • Background should be SOFT and OUT OF FOCUS (shallow depth of field)\n" +
			"   • Face remains the PRIMARY FOCUS - background is secondary but matches the reference\n" +
			"   • The blurred background should still show recognizable elements from the reference image\n\n" +
			"[BEAUTY PORTRAIT PRIORITY]\n" +
			"⚠️ CRITICAL: The face fills 60-80% of the frame\n" +
			"⚠️ Background is BLURRED (shallow depth) but matches the reference image's colors and elements\n" +
			"⚠️ Soft, flattering lighting from the environment\n\n" +
			"[BEAUTY PORTRAIT EXECUTION]\n" +
			"✓ Close-up composition - head and shoulders only\n" +
			"✓ Shallow depth of field - face is sharp, background is soft but recognizable from reference\n" +
			"✓ Soft, diffused lighting flatters the skin\n" +
			"✓ Environmental light creates subtle rim or fill light\n" +
			"✓ Background colors and mood match the reference, just out of focus\n\n" +
			"[TECHNICAL EXECUTION]\n" +
			"✓ Beauty photography lens (85mm-135mm equivalent)\n" +
			"✓ Shallow depth of field (f/2.8 or wider)\n" +
			"✓ Soft, natural color grading for skin tones\n" +
			"✓ Focus on eyes and facial features\n" +
			"✓ This is BEAUTY EDITORIAL with a specific background setting"
	} else if hasModel && !hasBackground {
		// 모델만 있고 배경 없음 → 뷰티 스튜디오
		compositionInstruction += " in a professional beauty studio with soft, flattering lighting.\n" +
			"Clean background (white, grey, or neutral) to emphasize the face."
	}
	// 프로덕트 샷이나 배경만 있는 케이스는 위에서 이미 처리됨

	// 핵심 요구사항 - 케이스별로 다르게
	var criticalRules string

	// 공통 금지사항 - 간결하게 통합
	commonForbidden := "\n\n[CRITICAL: SINGLE UNIFIED SCENE ONLY]\n" +
		"⚠️ NO SPLIT SCREENS, NO GRIDS, NO COLLAGES.\n" +
		"⚠️ ONE continuous composition with ONE background.\n" +
		"⚠️ NO vertical or horizontal dividing lines.\n" +
		func() string {
			productCount := len(categories.Products)
			if productCount > 0 {
				return fmt.Sprintf("⚠️ ABSOLUTE RULE: The reference contains EXACTLY %d products. YOU MUST SHOW ALL %d PRODUCTS.\n⚠️ COUNT THEM: 1, 2, ... %d. IF ANY ARE MISSING, THE IMAGE IS WRONG.\n⚠️ Do not add extra products. Do not remove any.\n", productCount, productCount, productCount)
			}
			return ""
		}()

	if hasModel {
		// 모델 있는 케이스 - 뷰티 클로즈업 규칙
		criticalRules = commonForbidden + "\n[BEAUTY PORTRAIT RULES]\n" +
			"🎯 CLOSE-UP PORTRAIT ONLY (Face & Shoulders). Face fills 60-80% of frame.\n" +
			"🎯 NO full body shots. NO fashion poses.\n" +
			"🎯 Perfect, natural facial features and skin texture.\n" +
			"🎯 Soft, flattering beauty lighting.\n"
	} else if hasProducts {
		// 뷰티 프로덕트 샷 케이스 - 화장품 촬영 규칙
		criticalRules = commonForbidden + "\n[BEAUTY PRODUCT RULES]\n" +
			"🎯 SHOWCASE products as premium objects. NO people/hands/faces.\n" +
			"🎯 Artistic, elegant arrangement. Soft, diffused lighting.\n" +
			"🎯 Products must sit naturally in the scene (shadows, reflections).\n" +
			"🎯 DO NOT copy the grid layout from the reference. Group them naturally.\n" +
			"🎯 NO sticker effect. Lighting on products MUST match the background.\n" +
			"🎯 RE-GENERATE the background. Do not use it as a static image.\n" +
			"🎯 MISSING PRODUCTS ARE UNACCEPTABLE. Count them before finalizing.\n"
	} else {
		// 배경만 있는 케이스
		criticalRules = commonForbidden + "\n[ENVIRONMENT RULES]\n" +
			"🎯 Capture atmosphere and mood. NO people/products.\n"
	}

	// 16:9 비율 전용 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "16:9" {
		if hasModel {
			// 모델이 있는 16:9 케이스 - 뷰티에서도 여전히 얼굴 클로즈업
			aspectRatioInstruction = "\n\n[16:9 BEAUTY PORTRAIT - WIDE FORMAT CLOSEUP]\n" +
				"⚠️ CRITICAL: Even in 16:9, this is STILL A BEAUTY CLOSEUP PORTRAIT.\n" +
				"The wide format provides horizontal space for creative framing, but the face remains the STAR.\n\n" +
				"🎬 16:9 BEAUTY COMPOSITION:\n" +
				"✓ Face and shoulders CLOSEUP - positioned creatively in the wide frame\n" +
				"✓ Subject positioned off-center (rule of thirds) for elegant composition\n" +
				"✓ Use the WIDTH for negative space and atmospheric background (soft and blurred)\n" +
				"✓ Face fills 60-80% of the frame vertically, even in wide format\n" +
				"✓ Horizontal space allows for directional lighting and mood\n\n" +
				"🎬 BEAUTY PORTRAIT INTEGRITY IN WIDE FRAME:\n" +
				"⚠️ The wide frame is NOT an excuse for full body shots\n" +
				"⚠️ Face maintains PERFECT natural proportions and remains the focal point\n" +
				"⚠️ Background is SOFT and OUT OF FOCUS, providing atmosphere only\n" +
				"⚠️ This is BEAUTY PHOTOGRAPHY, not fashion or environmental portraiture\n\n" +
				"🎬 BEAUTY EXECUTION IN 16:9:\n" +
				"✓ Soft, flattering beauty lighting (butterfly, loop, or Rembrandt)\n" +
				"✓ Shallow depth of field - face sharp, background soft\n" +
				"✓ Horizontal space used for elegant negative space and mood\n" +
				"✓ Natural color grading for skin tones\n\n" +
				"GOAL: A stunning wide-format beauty portrait like Peter Lindbergh or Patrick Demarchelier - \n" +
				"elegant closeup with horizontal breathing room, NOT a full body fashion shot."
		} else if hasProducts {
			// 뷰티 프로덕트 샷 16:9 케이스
			aspectRatioInstruction = "\n\n[16:9 BEAUTY PRODUCT SHOT]\n" +
				"This is a WIDE ANGLE beauty product shot - use the horizontal space for elegant cosmetic advertising.\n\n" +
				"🎬 ELEGANT WIDE BEAUTY PRODUCT COMPOSITION:\n" +
				"✓ Cosmetic products positioned elegantly using the full width\n" +
				"✓ Use the WIDTH for clean negative space and sophisticated aesthetic\n" +
				"✓ Soft, diffused lighting typical of beauty product photography\n" +
				"✓ Minimalist composition with focus on product details\n" +
				"✓ Negative space creates luxury and breathing room\n\n" +
				"🎬 BEAUTY PRODUCT EXECUTION:\n" +
				"✓ Soft lighting highlights product packaging and textures\n" +
				"✓ Clean, elegant aesthetic like high-end cosmetic ads\n" +
				"✓ Natural color grading for product accuracy\n" +
				"✓ Shallow depth of field emphasizes key products\n\n" +
				"GOAL: A stunning wide beauty product shot like Estée Lauder or Chanel advertising - clean, elegant, sophisticated."
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
