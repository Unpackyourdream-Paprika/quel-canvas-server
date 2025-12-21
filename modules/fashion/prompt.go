package fashion

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
		mainInstruction = "[HIGH-FASHION EDITORIAL PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class fashion photographer shooting a high-end editorial campaign.\n" +
			"This is SOLO FASHION MODEL photography - ONLY ONE PERSON in the frame.\n" +
			"The PERSON is the HERO - their natural proportions are SACRED and CANNOT be distorted.\n" +
			"The environment serves the subject, NOT the other way around.\n\n" +
			"Create ONE photorealistic photograph with HIGH-FASHION EDITORIAL STYLE:\n" +
			"• ONLY ONE MODEL - this is a solo fashion editorial shoot\n" +
			"• FULL BODY SHOT - model's ENTIRE body from head to TOE visible in frame\n" +
			"• FEET MUST BE VISIBLE - both feet and shoes completely in frame, NOT cut off\n" +
			"• CHIC and SOPHISTICATED fashion model pose - confident, elegant, striking\n" +
			"• SERIOUS FACIAL EXPRESSION - stern/fierce/intense gaze, stoic attitude, NO SMILING EVER\n" +
			"• Model's face is SERIOUS - closed mouth or slightly parted, intense eyes, editorial confidence\n" +
			"• ABSOLUTELY NO SMILING - this is critical (model must look stern, fierce, or neutral)\n" +
			"• STRONG POSTURE - elongated body lines, poised stance, dynamic angles\n" +
			"• The model wears ALL clothing and accessories in ONE complete outfit\n" +
			"• Fashion model attitude - NOT casual snapshot, NOT relaxed candid style\n" +
			"• Vogue/Harper's Bazaar editorial aesthetic - high fashion, not lifestyle photography\n" +
			"• Environmental storytelling - use the location for drama and visual impact\n" +
			"• Directional lighting creates mood, depth, and sculpts the model's features\n" +
			"• This is a MOMENT of high-fashion drama and editorial sophistication\n\n"
	} else if hasProducts {
		// 프로덕트만 → 프로덕트 포토그래피
		mainInstruction = "[CINEMATIC PRODUCT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class product photographer creating editorial-style still life.\n" +
			"The PRODUCTS are the STARS - showcase them as beautiful objects with perfect details.\n" +
			"CRITICAL: NO people or models in this shot - products only.\n\n" +
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
			"CRITICAL: NO people, models, or products in this shot - environment only.\n\n" +
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
			fmt.Sprintf("Reference Image %d (BACKGROUND/LOCATION): This is the EXACT environment/setting to use. Place the subject naturally within this specific location. Use the actual background elements, colors, lighting, and atmosphere from this reference image", imageIndex))
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
			"DO NOT add any people, models, or human figures.\n" +
			"Display the items artistically arranged - like high-end product photography.\n"

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
			"DO NOT add any people, models, or products to this scene.\n" +
			"Focus on capturing the atmosphere, lighting, and mood of the location itself."
	} else {
		// 케이스 4: 아무것도 없는 경우 (에러 케이스)
		compositionInstruction = "\n[CINEMATIC COMPOSITION]\n" +
			"Generate a high-quality photorealistic image based on the references provided."
	}

	// 배경 관련 지시사항 - 모델이 있을 때만 추가
	if hasModel && hasBackground {
		// 모델 + 배경 케이스 → 환경 통합 지시사항
		compositionInstruction += " shot on location with the referenced background environment.\n\n" +
			"[BACKGROUND INTEGRATION]\n" +
			"The referenced background image shows the EXACT setting to use.\n" +
			"Use the background reference as the ACTUAL location:\n" +
			"   - Place the subject within THIS specific environment\n" +
			"   - Use the actual colors, lighting, and atmosphere from the background reference\n" +
			"   - The background should look like the reference image - use its elements, style, and mood\n" +
			"   - Integrate the subject naturally into THIS location\n\n" +
			"[ABSOLUTE PRIORITY: SUBJECT INTEGRITY]\n" +
			"CRITICAL: The person's body proportions are UNTOUCHABLE\n" +
			"DO NOT distort, stretch, compress, or alter the person to fit the frame\n" +
			"The person must look natural and correctly proportioned in this environment\n\n" +
			"[DRAMATIC ENVIRONMENTAL INTEGRATION]\n" +
			"- Subject positioned naturally in the referenced space (standing, sitting, moving)\n" +
			"- Realistic ground contact with natural shadows\n" +
			"- Background elements from the reference create DEPTH\n" +
			"- Lighting matches the background reference's lighting direction\n" +
			"- Environmental light wraps around the subject naturally\n" +
			"- Atmospheric perspective adds cinematic depth\n" +
			"- Shot composition tells a STORY within this specific location\n\n" +
			"[TECHNICAL EXECUTION]\n" +
			"- Single camera angle - this is ONE photograph\n" +
			"- Film photography aesthetic with natural color grading\n" +
			"- Rule of thirds or dynamic asymmetric composition\n" +
			"- Depth of field focuses attention on the subject while showing the background\n" +
			"- The environment and subject look like they exist in the SAME REALITY"
	} else if hasModel && !hasBackground {
		// 모델만 있고 배경 없음 → 스튜디오
		compositionInstruction += " in a cinematic studio setting with professional film lighting."
	}
	// 프로덕트 샷이나 배경만 있는 케이스는 위에서 이미 처리됨

	// 핵심 요구사항 - 케이스별로 다르게
	var criticalRules string

	// 공통 금지사항 - 모든 케이스에 적용
	commonForbidden := "\n\n[CRITICAL: ABSOLUTELY FORBIDDEN]\n\n" +
		"NO VERTICAL DIVIDING LINES - ZERO TOLERANCE:\n" +
		"- NO white vertical line down the center\n" +
		"- NO colored vertical line separating the image\n" +
		"- NO border or separator dividing left and right\n" +
		"- NO panel division or comic book split layout\n" +
		"- The image must be ONE continuous scene without ANY vertical dividers\n\n" +
		"NO DUAL/SPLIT COMPOSITION - THIS IS NOT A COMPARISON IMAGE:\n" +
		"- DO NOT show the same character twice (left side vs right side)\n" +
		"- DO NOT show TWO different people (one on left, one on right)\n" +
		"- DO NOT create before/after, comparison, or variation layouts\n" +
		"- DO NOT duplicate the subject on both sides with different colors/styles\n" +
		"- This is ONE SINGLE MOMENT with ONE CHARACTER in ONE UNIFIED SCENE\n" +
		"- Left side and right side must be PART OF THE SAME ENVIRONMENT, not separate panels\n\n" +
		"ONLY ONE PERSON MAXIMUM:\n" +
		"- DO NOT show multiple models, friends, or people together\n" +
		"- DO NOT show background people or crowds visible in the frame\n" +
		"- This is SOLO photography - if there's a model, they are ALONE\n\n" +
		"SINGLE UNIFIED COMPOSITION ONLY:\n" +
		"- ONE continuous background that flows naturally across the entire frame\n" +
		"- ONE character in ONE pose at ONE moment in time\n" +
		"- NO repeating elements on left and right sides\n" +
		"- The entire image is ONE COHESIVE PHOTOGRAPH - not a collage or split screen\n" +
		"- Background elements (buildings, sky, ground) must be CONTINUOUS with no breaks or seams\n\n" +
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
		"- Professional editorial style - real single-shot photography only\n"

	if hasModel {
		// 모델 있는 케이스 - 드라마틱 패션 에디토리얼 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS - HIGH-FASHION EDITORIAL]\n" +
			"- ONLY ONE MODEL in the photograph - this is a solo fashion editorial\n" +
			"- HIGH-FASHION MODEL ATTITUDE - chic, sophisticated, confident, striking\n" +
			"- PROFESSIONAL FASHION POSE - elongated lines, strong posture, editorial stance\n" +
			"- SERIOUS FACIAL EXPRESSION MANDATORY - fierce/stern/intense gaze, stoic face (NEVER SMILING)\n" +
			"- Model's face shows INTENSITY - serious eyes, closed or slightly parted mouth, NO smile\n" +
			"- ABSOLUTELY NO SMILING - this will ruin the editorial aesthetic (high fashion = serious)\n" +
			"- Person's body proportions are PERFECT and NATURAL - ZERO tolerance for distortion\n" +
			"- FULL BODY SHOT MANDATORY - model's ENTIRE BODY must be visible from head to TOE\n" +
			"- FEET MUST BE VISIBLE - both feet and toes MUST appear in the frame (critical for full shot)\n" +
			"- DO NOT crop at ankles or calves - show complete legs down to the shoes and feet\n" +
			"- The subject is the STAR - everything else supports their presence\n" +
			"- Vogue/Harper's Bazaar aesthetic - high fashion editorial, NOT lifestyle photography\n" +
			"- Dramatic composition with ENERGY and MOVEMENT\n" +
			"- Environmental storytelling - what's the narrative of this moment?\n" +
			"- ALL clothing and accessories worn/carried simultaneously\n" +
			"- Single cohesive photograph - looks like ONE shot from ONE camera\n" +
			"- Film photography aesthetic - not digital, not flat\n" +
			"- Dynamic framing - use negative space creatively\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"- TWO or more people in the frame - this is NOT a group shot\n" +
			"- Multiple models, friends, or background people visible\n" +
			"- CROPPING AT ANKLES/CALVES - the model's feet MUST be visible in the frame\n" +
			"- CUT OFF FEET - both feet and shoes must appear completely in the photograph\n" +
			"- Bottom of frame cutting through legs - leave space below the feet\n" +
			"- ANY distortion of the person's proportions (stretched, compressed, squashed)\n" +
			"- Person looking pasted, floating, or artificially placed\n" +
			"- Casual, relaxed poses - this is HIGH FASHION, not lifestyle photography\n" +
			"- Static, boring, catalog-style poses without editorial attitude\n" +
			"- SMILING OR HAPPY EXPRESSION - model must be serious/fierce (NOT friendly, NOT smiling)\n" +
			"- Teeth showing in a smile - mouth should be closed or neutral\n" +
			"- Cheerful, joyful, or pleasant facial expression - this is editorial, not lifestyle\n" +
			"- Centered, symmetrical composition without drama\n" +
			"- Flat lighting that doesn't create mood"
	} else if hasProducts {
		// 프로덕트 샷 케이스 - 오브젝트 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS]\n" +
			"- Showcase the products as beautiful OBJECTS with perfect details\n" +
			"- Artistic arrangement - creative composition like high-end product photography\n" +
			"- Dramatic lighting that highlights textures and materials\n" +
			"- Environmental storytelling through product placement\n" +
			"- ALL items displayed clearly and beautifully\n" +
			"- Single cohesive photograph - ONE shot from ONE camera\n" +
			"- Film photography aesthetic - not digital, not flat\n" +
			"- Dynamic framing - use negative space and depth creatively\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE SHOT]\n" +
			"- ANY people, models, or human figures in the frame\n" +
			"- Products looking pasted or artificially placed\n" +
			"- Boring, flat catalog-style layouts\n" +
			"- Cluttered composition without focal point\n" +
			"- Flat lighting that doesn't create depth"
	} else {
		// 배경만 있는 케이스 - 환경 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE REQUIREMENTS]\n" +
			"- Capture the pure atmosphere and mood of the location\n" +
			"- Dramatic composition with depth and visual interest\n" +
			"- Environmental storytelling - what story does this place tell?\n" +
			"- Film photography aesthetic - not digital, not flat\n" +
			"- Dynamic framing - use negative space and layers creatively\n\n" +
			"[FORBIDDEN]\n" +
			"- DO NOT add people, models, or products to the scene\n" +
			"- Flat, boring composition without depth"
	}

	// aspect ratio별 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "9:16" {
		if hasModel {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL FASHION EDITORIAL - FULL BODY PORTRAIT]\n" +
				"This is a VERTICAL PORTRAIT format - perfect for showcasing the model's full body.\n\n" +
				"VERTICAL FULL BODY COMPOSITION:\n" +
				"- CRITICAL: Model's ENTIRE BODY from head to TOE must fit in the vertical frame\n" +
				"- FEET MUST BE VISIBLE - both feet and shoes completely in frame at the bottom\n" +
				"- Leave space BELOW the feet - do NOT crop at ankles or calves\n" +
				"- Use the HEIGHT to show the model's full silhouette and outfit\n" +
				"- Model positioned with room at top (hair/head space) and bottom (feet with ground)\n" +
				"- Dynamic vertical pose - elongated lines, fashion model stance\n" +
				"- Background provides context without overwhelming the subject\n\n" +
				"FRAMING REQUIREMENTS:\n" +
				"- Top of frame: room above head (not cropping hair)\n" +
				"- Bottom of frame: model's feet FULLY VISIBLE with space below\n" +
				"- This is a FULL BODY shot - show complete outfit from head to toe\n" +
				"- Model should occupy 60-75% of frame height - enough to see all details\n\n" +
				"FASHION EDITORIAL EXECUTION:\n" +
				"- Directional lighting sculpts the model's features and outfit\n" +
				"- Film photography aesthetic with natural color grading\n" +
				"- Depth of field emphasizes the model while showing environment\n" +
				"- Rule of thirds or dynamic composition - NOT centered\n\n" +
				"GOAL: A stunning vertical fashion editorial like Vogue or Harper's Bazaar - \n" +
				"capturing the model's complete look from head to toe with high-fashion sophistication."
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL PRODUCT SHOT]\n" +
				"This is a VERTICAL format product shot - use the height for elegant storytelling.\n\n" +
				"VERTICAL PRODUCT COMPOSITION:\n" +
				"- Products arranged to utilize the vertical space creatively\n" +
				"- Layers of depth from top to bottom\n" +
				"- Leading lines guide the eye through the composition\n" +
				"- Negative space creates elegance and breathing room\n\n" +
				"EXECUTION:\n" +
				"- Directional lighting creates drama and highlights textures\n" +
				"- Film grain and natural color grading\n" +
				"- Depth of field emphasizes products\n\n" +
				"GOAL: A stunning vertical product shot like high-end editorial still life photography."
		} else {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL LANDSCAPE SHOT]\n" +
				"This is a VERTICAL environmental shot - showcase the location's height and atmosphere.\n\n" +
				"VERTICAL COMPOSITION:\n" +
				"- Use the HEIGHT to capture vertical elements and scale\n" +
				"- Layers of depth from foreground to background\n" +
				"- Asymmetric composition creates visual interest\n\n" +
				"EXECUTION:\n" +
				"- Directional lighting creates mood and drama\n" +
				"- Film grain and natural color grading\n\n" +
				"GOAL: A stunning vertical environmental shot."
		}
	} else if aspectRatio == "16:9" {
		if hasModel {
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC WIDE SHOT - DRAMATIC STORYTELLING]\n" +
				"This is a WIDE ANGLE shot - use the horizontal space for powerful visual storytelling.\n\n" +
				"DRAMATIC WIDE COMPOSITION:\n" +
				"- CRITICAL: Model's ENTIRE BODY from head to TOE must be visible in the wide frame\n" +
				"- FEET MUST BE VISIBLE - both feet and shoes completely in frame at the bottom\n" +
				"- Leave space BELOW the feet - do NOT crop at ankles or calves\n" +
				"- Subject positioned off-center (rule of thirds) creating dynamic tension\n" +
				"- Use the WIDTH to show environmental context and atmosphere\n" +
				"- Layers of depth - foreground elements, subject, background scenery\n" +
				"- Leading lines guide the eye to the subject\n" +
				"- Negative space creates breathing room and drama\n\n" +
				"SUBJECT INTEGRITY IN WIDE FRAME:\n" +
				"- The wide frame is NOT an excuse to distort proportions\n" +
				"- Person maintains PERFECT natural proportions - just smaller in frame if needed\n" +
				"- FULL BODY shot - show complete outfit from head to toe with feet visible\n" +
				"- Use the space to tell a STORY, not to force-fit the subject\n\n" +
				"CINEMATIC EXECUTION:\n" +
				"- Directional lighting creates mood across the wide frame\n" +
				"- Atmospheric perspective - distant elements are hazier\n" +
				"- Film grain and natural color grading\n" +
				"- Depth of field emphasizes the subject while showing environment\n\n" +
				"GOAL: A breathtaking wide shot from a high-budget fashion editorial - \n" +
				"like Annie Leibovitz or Steven Meisel capturing a MOMENT of drama and beauty."
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC PRODUCT SHOT]\n" +
				"This is a WIDE ANGLE product shot - use the horizontal space for artistic storytelling.\n\n" +
				"DRAMATIC WIDE PRODUCT COMPOSITION:\n" +
				"- Products positioned creatively using the full width\n" +
				"- Use the WIDTH to show environmental context and atmosphere\n" +
				"- Layers of depth - foreground, products, background elements\n" +
				"- Leading lines guide the eye to the key products\n" +
				"- Negative space creates elegance and breathing room\n\n" +
				"CINEMATIC EXECUTION:\n" +
				"- Directional lighting creates drama and highlights textures\n" +
				"- Atmospheric perspective adds depth\n" +
				"- Film grain and natural color grading\n" +
				"- Depth of field emphasizes products while showing environment\n\n" +
				"GOAL: A stunning wide product shot like high-end editorial still life photography."
		} else {
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC WIDE LANDSCAPE SHOT]\n" +
				"This is a WIDE ANGLE environmental shot - showcase the location's grandeur.\n\n" +
				"DRAMATIC LANDSCAPE COMPOSITION:\n" +
				"- Use the full WIDTH to capture the environment's scale and atmosphere\n" +
				"- Layers of depth - foreground, midground, background elements\n" +
				"- Leading lines guide the eye through the scene\n" +
				"- Asymmetric composition creates visual tension and interest\n" +
				"- Negative space emphasizes the mood and emptiness (if appropriate)\n\n" +
				"CINEMATIC EXECUTION:\n" +
				"- Directional lighting creates mood and drama\n" +
				"- Atmospheric perspective - distant elements are hazier\n" +
				"- Film grain and natural color grading\n" +
				"- Depth of field adds dimension to the scene\n\n" +
				"GOAL: A stunning environmental shot that tells a story without people - \n" +
				"like a cinematic establishing shot from a high-budget film."
		}
	} else {
		if hasModel {
			aspectRatioInstruction = "\n\n[1:1 SQUARE FASHION EDITORIAL - FULL BODY PORTRAIT]\n" +
				"This is a SQUARE format - perfect for balanced fashion editorial composition.\n\n" +
				"SQUARE FULL BODY COMPOSITION:\n" +
				"- CRITICAL: Model's ENTIRE BODY from head to TOE must fit in the square frame\n" +
				"- FEET MUST BE VISIBLE - both feet and shoes completely in frame at the bottom\n" +
				"- Leave space BELOW the feet - do NOT crop at ankles or calves\n" +
				"- Balanced composition utilizing the square format\n" +
				"- Model positioned with room at top and bottom for full body visibility\n" +
				"- Dynamic pose - fashion model stance with editorial confidence\n" +
				"- Background provides context without overwhelming the subject\n\n" +
				"FRAMING REQUIREMENTS:\n" +
				"- Top of frame: room above head (not cropping hair)\n" +
				"- Bottom of frame: model's feet FULLY VISIBLE with space below\n" +
				"- This is a FULL BODY shot - show complete outfit from head to toe\n" +
				"- Model should occupy appropriate frame space - enough to see all details\n\n" +
				"FASHION EDITORIAL EXECUTION:\n" +
				"- Directional lighting sculpts the model's features and outfit\n" +
				"- Film photography aesthetic with natural color grading\n" +
				"- Depth of field emphasizes the model while showing environment\n" +
				"- Dynamic composition - NOT static or centered\n\n" +
				"GOAL: A stunning square fashion editorial showcasing the model's complete look from head to toe."
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[1:1 SQUARE PRODUCT SHOT]\n" +
				"This is a SQUARE format product shot - balanced and elegant.\n\n" +
				"SQUARE PRODUCT COMPOSITION:\n" +
				"- Products arranged to utilize the square space creatively\n" +
				"- Balanced composition with artistic arrangement\n" +
				"- Negative space creates elegance\n\n" +
				"EXECUTION:\n" +
				"- Directional lighting creates drama and highlights textures\n" +
				"- Film grain and natural color grading\n\n" +
				"GOAL: A stunning square product shot."
		} else {
			aspectRatioInstruction = "\n\n[1:1 SQUARE LANDSCAPE SHOT]\n" +
				"This is a SQUARE environmental shot - balanced composition.\n\n" +
				"SQUARE COMPOSITION:\n" +
				"- Balanced framing utilizing the square format\n" +
				"- Layers of depth create visual interest\n\n" +
				"EXECUTION:\n" +
				"- Directional lighting creates mood\n" +
				"- Film grain and natural color grading\n\n" +
				"GOAL: A stunning square environmental shot."
		}
	}

	// 최우선 지시사항 - 맨 앞에 배치
	criticalHeader := "[CRITICAL REQUIREMENTS - ABSOLUTE PRIORITY - IMAGE WILL BE REJECTED IF NOT FOLLOWED]\n\n" +
		"[MANDATORY - FEET MUST BE VISIBLE]:\n" +
		"- BOTH FEET MUST APPEAR COMPLETELY IN THE FRAME - NO EXCEPTIONS\n" +
		"- DO NOT CROP AT ANKLES, CALVES, OR KNEES\n" +
		"- LEAVE SPACE BELOW THE FEET - show ground/floor beneath the shoes\n" +
		"- FULL BODY means HEAD TO TOE - every part of the body must be visible\n" +
		"- Bottom edge of frame MUST be BELOW the model's feet, NOT cutting through legs\n\n" +
		"[MANDATORY - FACIAL EXPRESSION - ABSOLUTE REQUIREMENT]:\n" +
		"- MODEL MUST NOT SMILE - THIS IS NON-NEGOTIABLE\n" +
		"- ZERO TOLERANCE for smiling - image will be REJECTED if model is smiling\n" +
		"- NO happy expression whatsoever - NO grin, NO teeth showing, NO friendly smile\n" +
		"- NO slight smile, NO subtle smile, NO hint of smile - NONE AT ALL\n" +
		"- REQUIRED EXPRESSION: Serious, stern, fierce, intense, or stoic ONLY\n" +
		"- Model should look like a professional runway model - INTENSE gaze, NOT happy\n" +
		"- Think Vogue/Harper's Bazaar editorial - models are FIERCE and SERIOUS, never cheerful\n" +
		"- Mouth should be CLOSED or slightly parted - NEVER showing teeth in a smile\n" +
		"- Eyes should be INTENSE and FOCUSED - serious editorial confidence\n\n" +
		"[NEGATIVE PROMPT - ABSOLUTELY FORBIDDEN FACIAL EXPRESSIONS]:\n" +
		"- SMILING - model is smiling, happy smile, friendly smile, subtle smile, slight smile\n" +
		"- GRINNING - model is grinning, showing teeth, big smile, wide smile\n" +
		"- HAPPY EXPRESSION - cheerful look, joyful expression, pleasant smile\n" +
		"- CASUAL FRIENDLY FACE - relaxed smile, candid smile, lifestyle photography smile\n" +
		"- TEETH VISIBLE IN SMILE - any teeth showing from smiling\n\n" +
		"[FORBIDDEN - IMAGE WILL BE REJECTED]:\n" +
		"- NO left-right split, NO side-by-side layout, NO duplicate subject on both sides\n" +
		"- NO grid, NO collage, NO comparison view, NO before/after layout\n" +
		"- NO vertical dividing line, NO center split, NO symmetrical duplication\n" +
		"- NO white/gray borders, NO letterboxing, NO empty margins on any side\n" +
		"- NO multiple identical poses, NO mirrored images, NO panel divisions\n" +
		"- NO separate product shots arranged in a grid or catalog layout\n" +
		"- ONLY ONE PERSON in the photograph - NO multiple models, NO two people, NO groups\n" +
		"- NO SMILING - model must have serious/fierce fashion expression (CRITICAL)\n" +
		"- NO CROPPED FEET - both feet must be fully visible in frame\n\n" +
		"[REQUIRED - MUST GENERATE THIS WAY]:\n" +
		"- ONE single continuous photograph taken with ONE camera shutter\n" +
		"- ONE unified moment in time - NOT two or more moments combined\n" +
		"- ONLY ONE MODEL in the entire frame - this is a solo fashion editorial\n" +
		"- MODEL'S FEET FULLY VISIBLE with space below them\n" +
		"- SERIOUS/STERN/FIERCE expression - stern face, serious eyes, intense gaze\n" +
		"- MODEL'S FACE shows editorial confidence - NOT happiness, NOT friendliness\n" +
		"- FILL entire frame edge-to-edge with NO empty space\n" +
		"- Natural asymmetric composition - left side MUST be different from right side\n" +
		"- Professional editorial style - real single-shot photography only\n\n"

	// 최종 조합: 콜라주 방지 최우선 -> 시네마틱 지시사항 -> 참조 이미지 설명 -> 구성 요구사항 -> 핵심 규칙 -> aspect ratio 특화
	var finalPrompt string

	// 크리티컬 요구사항을 맨 앞에 배치
	if userPrompt != "" {
		finalPrompt = criticalHeader + "[ADDITIONAL STYLING]\n" + userPrompt + "\n\n"
	} else {
		finalPrompt = criticalHeader
	}

	// 카테고리별 고정 스타일 가이드
	categoryStyleGuide := "\n\n[FASHION PHOTOGRAPHY STYLE GUIDE]\n" +
		"Fashion photography style. Professional runway lighting. High-end fashion editorial composition. Model wearing designer clothing.\n\n" +
		"[TECHNICAL CONSTRAINTS]\n" +
		"ABSOLUTELY NO VERTICAL COMPOSITION. ABSOLUTELY NO SIDE MARGINS. ABSOLUTELY NO WHITE/GRAY BARS ON LEFT OR RIGHT. Fill entire width from left edge to right edge. NO letterboxing. NO pillarboxing. NO empty sides.\n"

	// 나머지 지시사항들
	finalPrompt += mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + categoryStyleGuide + criticalRules + aspectRatioInstruction

	return finalPrompt
}
