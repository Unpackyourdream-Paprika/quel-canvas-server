package beauty

import (
	"fmt"
	"log"
	"strings"
)

// PromptCategories - Beauty 카테고리별 이미지 분류 구조체 (화장품 전용)
// 프론트 type: model, product, background
type PromptCategories struct {
	Model      []byte   // 모델 이미지 (최대 1장) - Beauty에서는 인물 뷰티 샷용
	Product    [][]byte // 화장품/제품 이미지 배열 (lipstick, cream, bottle 등) - Beauty 전용
	Background []byte   // 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Beauty 모듈 전용 프롬프트 생성
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasModel := categories.Model != nil
	hasProduct := len(categories.Product) > 0 // Beauty 전용: Product 필드 직접 확인
	hasBackground := categories.Background != nil

	// 디버그 로그 추가
	log.Printf("🔍 [Beauty Prompt] Model:%v, Product:%d, BG:%v",
		hasModel, len(categories.Product), hasBackground)

	// 케이스별 메인 지시사항
	var mainInstruction string
	if hasModel {
		// 모델 있음 → 뷰티 포트레이트 (FACE IDENTITY가 최우선)
		mainInstruction = "🚨🚨🚨 ABSOLUTE PRIORITY #1: FACE IDENTITY PRESERVATION 🚨🚨🚨\n\n" +
			"[FACE IDENTITY - THIS IS THE MOST IMPORTANT RULE]:\n" +
			"🚨 YOU MUST CLONE THE EXACT FACE FROM THE MODEL REFERENCE IMAGE\n" +
			"🚨 THE PERSON'S FACE MUST BE IDENTICAL - NOT SIMILAR, BUT IDENTICAL\n" +
			"🚨 COPY: Same eyes shape, same nose shape, same lips shape, same face shape\n" +
			"🚨 COPY: Same skin tone, same ethnicity, same age appearance\n" +
			"🚨 COPY: Same eyebrows, same cheekbones, same jawline, same chin\n" +
			"🚨 COPY: Same hair color, same hair style, same hair texture\n" +
			"🚨 IF THE MODEL IS ASIAN, THE RESULT MUST BE THE SAME ASIAN PERSON\n" +
			"🚨 IF THE MODEL IS CAUCASIAN, THE RESULT MUST BE THE SAME CAUCASIAN PERSON\n" +
			"🚨 DO NOT CREATE A DIFFERENT PERSON - USE THE EXACT SAME PERSON\n" +
			"🚨 DO NOT BEAUTIFY OR MODIFY THE FACE - KEEP IT EXACTLY AS REFERENCE\n" +
			"🚨 THE VIEWER SHOULD RECOGNIZE THIS AS THE SAME INDIVIDUAL\n\n" +
			"[BEAUTY PHOTOGRAPHER'S CLOSE-UP PORTRAIT]\n" +
			"You are a world-class beauty photographer specializing in cosmetic editorial and makeup photography.\n\n" +
			"Create ONE photorealistic beauty photograph with FLAWLESS SKIN DETAIL:\n" +
			"• Soft, flattering lighting for beauty photography (butterfly or loop lighting)\n" +
			"• Professional studio beauty photography composition\n" +
			"• High-end cosmetic editorial quality\n\n"
	} else if hasProduct {
		// 프로덕트만 → 뷰티 프로덕트 (화장품/제품) - 개수에 따라 동적 프롬프트
		productCount := len(categories.Product)
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
					"⚠️ YOU MUST RECREATE ALL PRODUCTS visible in the reference grid EXACTLY.\n" +
					"⚠️ Do not select just one. Recreate the entire set with EXACT colors, shapes, and packaging.\n"
			} else {
				// Allow flexibility if it might be a grid but not explicitly stated,
				// but prioritize single product if it looks like one.
				productCountInstruction = "⚠️ CRITICAL: RECREATE the product(s) EXACTLY as shown in the reference.\n" +
					"⚠️ If the reference is a GRID of multiple items, RECREATE ALL OF THEM with exact colors and shapes.\n" +
					"⚠️ If it is a single item, recreate exactly that one product with matching colors and packaging.\n"
			}
		case 2:
			productCountInstruction = "⚠️ CRITICAL: RECREATE EXACTLY 2 (TWO) products - both items from the reference must appear with EXACT colors and shapes.\n" +
				"⚠️ DO NOT add extra products. DO NOT omit any. DO NOT change colors or packaging. EXACTLY 2 products.\n"
		case 3:
			productCountInstruction = "⚠️ CRITICAL: RECREATE EXACTLY 3 (THREE) products - all three items from the reference must appear with EXACT colors and shapes.\n" +
				"⚠️ DO NOT add extra products. DO NOT omit any. DO NOT change colors or packaging. EXACTLY 3 products.\n"
		case 4:
			productCountInstruction = "⚠️ CRITICAL: RECREATE EXACTLY 4 (FOUR) products - all four items from the reference must appear with EXACT colors and shapes.\n" +
				"⚠️ DO NOT add extra products. DO NOT omit any. DO NOT change colors or packaging. EXACTLY 4 products.\n" +
				"⚠️ ARRANGE them naturally in the scene (e.g., a group composition), NOT as a 2x2 grid.\n"
		default:
			productCountInstruction = fmt.Sprintf("⚠️ CRITICAL: RECREATE EXACTLY %d products - ALL items from the reference must appear with EXACT colors and shapes.\n"+
				"⚠️ DO NOT add extra products. DO NOT omit any. DO NOT change colors or packaging. EXACTLY %d products.\n", productCount, productCount)
		}

		mainInstruction = "[BEAUTY PRODUCT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class cosmetic product photographer.\n" +
			"The BEAUTY PRODUCTS from the reference are the STARS - you must RECREATE them EXACTLY.\n" +
			"⚠️ CRITICAL: NO people or models in this shot - beauty products only.\n" +
			"⚠️ CRITICAL: DO NOT invent new products. RECREATE the EXACT products from the reference image.\n" +
			"⚠️ CRITICAL: Match colors, shapes, packaging designs, and labels EXACTLY from the reference.\n" +
			productCountInstruction +
			"\nCreate ONE photorealistic photograph with COSMETIC ELEGANCE:\n" +
			"• RECREATE the exact products from the reference (matching colors, shapes, packaging)\n" +
			"• Arrange them artistically in a natural composition (NOT a grid)\n" +
			"• Soft, diffused lighting that highlights product details\n" +
			"• Premium cosmetic brand photography style\n" +
			"• Clean, elegant composition\n" +
			"• This is high-end beauty product photography showing the EXACT referenced products\n\n"
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
			fmt.Sprintf("Reference Image %d (MODEL - FACE IDENTITY SOURCE):\n"+
				"🚨🚨🚨 THIS PERSON'S FACE IS SACRED - YOU MUST CLONE IT EXACTLY 🚨🚨🚨\n\n"+
				"[FACE CLONING REQUIREMENTS - MANDATORY]:\n"+
				"• CLONE this exact face - the result must show THE SAME PERSON\n"+
				"• CLONE: Eye shape, eye color, eye size, eye spacing\n"+
				"• CLONE: Nose shape, nose size, nostril shape\n"+
				"• CLONE: Lip shape, lip thickness, lip color\n"+
				"• CLONE: Face shape (round/oval/square/heart)\n"+
				"• CLONE: Cheekbone position, jawline, chin shape\n"+
				"• CLONE: Eyebrow shape, eyebrow thickness\n"+
				"• CLONE: Skin tone, skin texture, any freckles/moles\n"+
				"• CLONE: Hair color, hair style, hair length, hair texture\n"+
				"• CLONE: Ethnicity - if Asian, result must be the SAME Asian person\n"+
				"• CLONE: Age appearance - if young, result must look the same age\n\n"+
				"⚠️ SKIN TONE PRESERVATION: The model's skin tone must match the reference EXACTLY.\n"+
				"DO NOT let product colors affect the model's skin tone.\n\n"+
				"[IDENTITY CHECK]: A friend of this person should INSTANTLY recognize them in the output\n\n"+
				"⚠️ IGNORE FROM THIS MODEL IMAGE (USE ONLY FOR FACE/BODY):\n"+
				"❌ IGNORE the background in this model photo - use ONLY the separate BACKGROUND reference\n"+
				"❌ IGNORE any clothing/accessories in this model photo\n"+
				"❌ This model image is ONLY for FACE and BODY reference - NOTHING else", imageIndex))
		imageIndex++
	}

	if len(categories.Product) > 0 {
		productCount := len(categories.Product)
		if hasModel {
			// 모델 + 제품: 제품을 들고 있는 CF 샷
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (PRODUCT TO HOLD): This is the EXACT product the model must HOLD in the shot. Recreate this product EXACTLY - same shape, same color, same packaging, same labels. The model should elegantly hold or present this product like a cosmetic CF/commercial. ⚠️ NATURAL INTEGRATION: The product must look NATURALLY held - proper shadows on hand, realistic lighting matching the scene, natural reflections. DO NOT paste the product like a sticker. The product must be rendered as part of the SAME 3D scene with consistent lighting, shadows, and depth.", imageIndex))
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
					countDesc = "The reference shows multiple products in a grid. You MUST recreate EXACTLY these same products - same colors, same shapes, same packaging designs. Show ALL of them arranged naturally together."
				} else {
					countDesc = "The reference shows the EXACT product you must recreate. Copy this product's appearance EXACTLY - same color, same shape, same packaging, same label design. Show it naturally in the scene."
				}
			case 2:
				countDesc = "The reference shows 2 products (in a grid). You MUST recreate these TWO EXACT products - same colors, same shapes, same packaging. Arrange these TWO products naturally together in the scene. DO NOT copy the grid layout, but DO copy the products exactly."
			case 3:
				countDesc = "The reference shows 3 products (in a grid). You MUST recreate these THREE EXACT products - same colors, same shapes, same packaging. Arrange these THREE products naturally together as a group. DO NOT copy the grid layout, but DO copy the products exactly."
			case 4:
				countDesc = "The reference shows 4 products (in a grid). You MUST recreate these FOUR EXACT products - same colors, same shapes, same packaging. Arrange these FOUR products naturally together as a group. DO NOT copy the grid layout, but DO copy the products exactly."
			default:
				countDesc = fmt.Sprintf("The reference shows %d products. You MUST recreate ALL %d EXACT products - same colors, same shapes, same packaging. Arrange ALL %d products naturally together in the scene. DO NOT copy the grid layout, but DO copy the products exactly.", productCount, productCount, productCount)
			}
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (BEAUTY PRODUCTS - %d ITEMS TO RECREATE EXACTLY): %s ⚠️ CRITICAL: These are the EXACT cosmetic products you must RECREATE in the new scene. DO NOT invent new products. DO NOT change colors, shapes, or packaging designs. COPY these products EXACTLY as they appear in the reference, then place them in the new scene with premium cosmetic photography style.", imageIndex, productCount, countDesc))
		}
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
	} else if hasProduct {
		// 케이스 2: 모델 없이 제품만 → 뷰티 프로덕트 샷 (화장품/코스메틱)
		compositionInstruction = "\n[BEAUTY PRODUCT PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic beauty product photograph showcasing cosmetics and beauty items as OBJECTS.\n" +
			"⚠️ CRITICAL: RECREATE the EXACT products from the reference image - same colors, same shapes, same packaging.\n" +
			"⚠️ CRITICAL: DO NOT invent new products or change the product designs.\n" +
			"⚠️ CRITICAL: DO NOT add any people, models, or human figures.\n" +
			"⚠️ CRITICAL: DO NOT add hands, fingers, or any body parts holding products.\n" +
			"⚠️ CRITICAL: NO human faces, NO portraits, NO makeup application shots - PRODUCTS ONLY.\n" +
			"⚠️ RECREATE the exact products from the reference, then arrange them artistically in a new scene.\n" +
			"⚠️ USE ONLY the provided product references; do NOT invent extra products or variants."

		if hasBackground {
			compositionInstruction += "\n\n[PRODUCT RECREATION + BACKGROUND INTEGRATION]\n" +
				"Step 1: RECREATE the beauty products EXACTLY from the reference (colors, shapes, packaging).\n" +
				"Step 2: Place these recreated products in a FULLY RE-RENDERED 3D ENVIRONMENT inspired by the background reference.\n" +
				"⚠️ CRITICAL: The background reference is ONLY for mood, colors, and texture. IT IS NOT A TEMPLATE.\n" +
				"⚠️ YOU HAVE FULL CREATIVE FREEDOM to change the background layout, geometry, and perspective to best fit the products.\n" +
				"⚠️ DO NOT try to match the reference background's shape or object placement. CREATE A NEW SCENE.\n" +
				"⚠️ GLOBAL ILLUMINATION: The light source from the generated environment must interact realistically with the products.\n" +
				"⚠️ AMBIENT OCCLUSION: Create deep, realistic contact shadows where the products touch the surface to avoid the 'floating sticker' look.\n" +
				"⚠️ LIGHT WRAP: Let the background light softly wrap around the product edges to blend them naturally into the scene.\n" +
				"⚠️ COLOR BLEED: Allow the background colors (e.g., green from leaves) to subtly reflect on the product surfaces for true integration.\n" +
				"⚠️ The EXACT products from reference and the new background must be rendered TOGETHER as one single 3D scene.\n" +
				"This is a completely NEW photograph where the background is re-created to perfectly fit the EXACT products from reference."
		} else {
			compositionInstruction += "\n\nCreate a stunning studio beauty product shot with soft, diffused lighting and clean composition.\n" +
				"RECREATE the exact cosmetic items from the reference (colors, shapes, packaging), then arrange them artistically - flat lay, clean display, or elegantly positioned with beauty editorial aesthetic.\n" +
				"Think premium beauty brand campaigns (Estée Lauder, La Mer, Tom Ford Beauty) - pure product elegance, zero human presence.\n" +
				"⚠️ Remember: Copy the EXACT products from reference, do NOT invent new ones."
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
		"⚠️ NO vertical or horizontal dividing lines.\n\n" +
		"[ABSOLUTELY FORBIDDEN - IMAGE WILL BE REJECTED]:\n" +
		"- NO left-right split, NO side-by-side layout, NO duplicate subject on both sides\n" +
		"- NO grid, NO collage, NO comparison view, NO before/after layout\n" +
		"- NO vertical dividing line, NO center split, NO symmetrical duplication\n" +
		"- NO white/gray borders, NO letterboxing, NO empty margins on any side\n" +
		"- NO multiple identical poses, NO mirrored images, NO panel divisions\n" +
		"- NO vertical portrait orientation with side margins\n\n" +
		"[REQUIRED - MUST GENERATE THIS WAY]:\n" +
		"- ONE single continuous photograph taken with ONE camera shutter\n" +
		"- ONE unified moment in time - NOT two or more moments combined\n" +
		"- FILL entire frame edge-to-edge with NO empty space\n" +
		"- Natural asymmetric composition - left side MUST be different from right side\n" +
		"- Professional editorial style - real single-shot photography only\n" +
		func() string {
			productCount := len(categories.Product)
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
	} else if hasProduct {
		// 뷰티 프로덕트 샷 케이스 - 화장품 촬영 규칙
		criticalRules = commonForbidden + "\n[BEAUTY PRODUCT RULES]\n" +
			"🎯 RECREATE the EXACT products from reference - match colors, shapes, packaging PRECISELY.\n" +
			"🎯 DO NOT invent new products. DO NOT change product designs or colors.\n" +
			"🎯 SHOWCASE recreated products as premium objects. NO people/hands/faces.\n" +
			"🎯 Artistic, elegant arrangement. Soft, diffused lighting.\n" +
			"🎯 Products must sit naturally in the scene (shadows, reflections).\n" +
			"🎯 DO NOT copy the grid layout from the reference. Group them naturally.\n" +
			"🎯 NO sticker effect. Lighting on products MUST match the background.\n" +
			"🎯 RE-GENERATE the background. Do not use it as a static image.\n" +
			"🎯 MISSING PRODUCTS ARE UNACCEPTABLE. Count them before finalizing.\n" +
			"🎯 CHANGED PRODUCT COLORS ARE UNACCEPTABLE. Match the reference exactly.\n"
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
		} else if hasProduct {
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

	// 카테고리별 고정 스타일 가이드
	categoryStyleGuide := "\n\n[BEAUTY PHOTOGRAPHY STYLE GUIDE]\n" +
		"Beauty product photography, cosmetic packaging shot, professional product lighting, clean background, high-end commercial photography, luxury cosmetic brand style, focus on product texture and packaging details, NO people, NO human faces, product only\n\n" +
		"[TECHNICAL CONSTRAINTS]\n" +
		"ABSOLUTELY NO VERTICAL COMPOSITION. ABSOLUTELY NO SIDE MARGINS. ABSOLUTELY NO WHITE/GRAY BARS ON LEFT OR RIGHT. Fill entire width from left edge to right edge. NO letterboxing. NO pillarboxing. NO empty sides.\n"

	// 최종 조합: 시네마틱 지시사항 → 참조 이미지 설명 → 구성 요구사항 → 카테고리 스타일 → 핵심 규칙 → 16:9 특화
	finalPrompt := mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + categoryStyleGuide + criticalRules + aspectRatioInstruction

	if userPrompt != "" {
		finalPrompt += "\n\n[ADDITIONAL STYLING]\n" + userPrompt
	}

	return finalPrompt
}
