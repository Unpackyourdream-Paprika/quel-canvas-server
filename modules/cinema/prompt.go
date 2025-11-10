package cinema

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

// GenerateDynamicPrompt - Cinema 모듈 전용 프롬프트 생성
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasModel := categories.Model != nil
	hasClothing := len(categories.Clothing) > 0
	hasAccessories := len(categories.Accessories) > 0
	hasProducts := hasClothing || hasAccessories
	hasBackground := categories.Background != nil

	// 케이스별 메인 지시사항 - Cinema 전용
	var mainInstruction string
	if hasModel {
		// 모델 있음 → 영화 장면 / 시네마틱 프레임
		mainInstruction = "⚠️ ABSOLUTE PHOTOREALISM REQUIREMENT - THIS IS NOT OPTIONAL:\n" +
			"Generate a 100% PHOTOREALISTIC image that looks like it was captured by a REAL CAMERA.\n" +
			"• ZERO artistic interpretation - pure photography\n" +
			"• ZERO illustration, painting, or stylized rendering\n" +
			"• Must look INDISTINGUISHABLE from a real photograph taken on film set\n" +
			"• Real skin texture, real fabric texture, real lighting physics\n" +
			"• If someone cannot tell this from a real photo, you succeeded\n\n" +
			"[CINEMA DIRECTOR'S DRAMATIC FRAME]\n" +
			"You are a world-class cinematographer shooting a high-budget film scene.\n" +
			"The CHARACTER is the emotional center - their natural proportions and presence drive the narrative.\n" +
			"Every frame tells a story through composition, lighting, and atmosphere.\n\n" +
			"Create ONE photorealistic cinematic film frame with DRAMATIC STORYTELLING:\n" +
			"• The character exists in a specific moment of the narrative\n" +
			"• Camera angle and framing create emotional impact\n" +
			"• Environmental storytelling - location reveals character and mood\n" +
			"• Cinematic lighting creates depth, drama, and atmosphere\n" +
			"• This is a FILM STILL from a high-production movie scene\n\n"
	} else if hasProducts {
		// 프로덕트만 → 영화 소품 / 시네마틱 오브젝트
		mainInstruction = "[CINEMATIC PROP PHOTOGRAPHER'S APPROACH]\n" +
			"You are a cinematic prop photographer creating dramatic still life for film production.\n" +
			"The OBJECTS are narrative elements - they tell a story through presence and arrangement.\n" +
			"⚠️ CRITICAL: NO people or characters in this shot - objects only.\n\n" +
			"Create ONE photorealistic cinematic still life with NARRATIVE WEIGHT:\n" +
			"• Objects arranged to suggest story and context\n" +
			"• Dramatic film lighting that creates mood and mystery\n" +
			"• Environmental context suggests a larger narrative\n" +
			"• Directional lighting creates cinematic depth\n" +
			"• This is a KEY PROP SHOT from a film production\n\n"
	} else {
		// 배경만 → 영화 로케이션 / 시네마틱 환경
		mainInstruction = "[CINEMATIC LOCATION SCOUT'S APPROACH]\n" +
			"You are a cinematographer capturing an establishing shot for a film.\n" +
			"The LOCATION is a character itself - it sets tone, mood, and narrative context.\n" +
			"⚠️ CRITICAL: NO people or objects in this shot - pure environment.\n\n" +
			"Create ONE photorealistic cinematic establishing shot with ATMOSPHERIC PRESENCE:\n" +
			"• Dramatic composition that establishes the world of the film\n" +
			"• Layers of depth create cinematic scale\n" +
			"• Film lighting creates mood, time of day, and emotional tone\n" +
			"• This is an ESTABLISHING SHOT from a high-budget film\n\n"
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

	// 시네마틱 구성 지시사항 - Cinema 전용
	var compositionInstruction string

	// 케이스 1: 모델 이미지가 있는 경우 → 영화 장면의 캐릭터
	if hasModel {
		compositionInstruction = "\n[CINEMATIC FILM SCENE COMPOSITION]\n" +
			"Generate ONE photorealistic film frame showing the referenced character in a dramatic moment.\n" +
			"This is a HIGH-BUDGET MOVIE SCENE with the character as the emotional center of the narrative.\n" +
			"Film production quality with cinematic lighting, color grading, and composition."
	} else if hasProducts {
		// 케이스 2: 모델 없이 오브젝트만 → 영화 소품 샷
		compositionInstruction = "\n[CINEMATIC PROP SHOT]\n" +
			"Generate ONE photorealistic film still showcasing the objects as KEY NARRATIVE PROPS.\n" +
			"⚠️ DO NOT add any people, characters, or human figures.\n" +
			"⚠️ Display the items as if they are important props in a film scene.\n"

		if hasBackground {
			compositionInstruction += "The props are placed naturally within the cinematic environment - " +
				"as if arranged by a production designer for a key film moment.\n" +
				"The objects tell a story through their placement and interaction with the space."
		} else {
			compositionInstruction += "Create a dramatic studio prop shot with cinematic lighting and composition.\n" +
				"The objects are arranged to suggest narrative weight and story context."
		}
	} else if hasBackground {
		// 케이스 3: 배경만 → 영화 로케이션 샷
		compositionInstruction = "\n[CINEMATIC ESTABLISHING SHOT]\n" +
			"Generate ONE photorealistic film establishing shot of the referenced location.\n" +
			"⚠️ DO NOT add any people, characters, or props to this scene.\n" +
			"Focus on capturing the cinematic atmosphere, mood, and environmental storytelling of the location itself."
	} else {
		// 케이스 4: 아무것도 없는 경우 (에러 케이스)
		compositionInstruction = "\n[CINEMATIC FILM FRAME]\n" +
			"Generate a high-quality photorealistic cinematic image based on the references provided."
	}

	// 배경 관련 지시사항 - 캐릭터가 있을 때만 추가
	if hasModel && hasBackground {
		// 캐릭터 + 배경 케이스 → 영화 장면 환경 통합 지시사항
		compositionInstruction += " shot on cinematic location with narrative environmental storytelling.\n\n" +
			"[CINEMATOGRAPHER'S APPROACH TO LOCATION]\n" +
			"The director CHOSE this environment to serve the story and character moment.\n" +
			"🎬 Use the location reference as INSPIRATION ONLY:\n" +
			"   • Recreate the mood, atmosphere, and cinematic tone\n" +
			"   • Generate a NEW film-quality scene - do NOT paste or overlay the reference\n" +
			"   • The location is a NARRATIVE STAGE that reveals character and story\n\n" +
			"[ABSOLUTE PRIORITY: CHARACTER INTEGRITY]\n" +
			"⚠️ CRITICAL: The character's body proportions are NATURAL and UNTOUCHABLE\n" +
			"⚠️ DO NOT distort, stretch, compress, or alter the character to fit the frame\n" +
			"⚠️ The environment supports the character - NEVER overwhelms them\n\n" +
			"[CINEMATIC ENVIRONMENTAL INTEGRATION]\n" +
			"✓ Character positioned naturally in the scene (standing, moving, interacting)\n" +
			"✓ Realistic spatial relationship with natural shadows and lighting\n" +
			"✓ Environmental elements create CINEMATIC DEPTH - foreground/midground/background layers\n" +
			"✓ Directional film lighting creates mood, drama, and atmosphere\n" +
			"✓ Environmental light wraps naturally around the character\n" +
			"✓ Atmospheric perspective adds film production depth\n" +
			"✓ Shot composition tells a NARRATIVE - this is a moment in a larger story\n\n" +
			"[TECHNICAL FILM EXECUTION]\n" +
			"✓ Single camera angle - this is ONE film frame from ONE take\n" +
			"✓ Film production aesthetic with cinematic color grading\n" +
			"✓ Cinematic composition rules - rule of thirds, leading lines, dynamic framing\n" +
			"✓ Depth of field creates focus and separates character from environment\n" +
			"✓ The environment and character exist in the SAME CINEMATIC REALITY"
	} else if hasModel && !hasBackground {
		// 캐릭터만 있고 배경 없음 → 스튜디오 촬영
		compositionInstruction += " in a controlled studio environment with professional cinematic film lighting."
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
		// 캐릭터 있는 케이스 - 시네마틱 영화 장면 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE CINEMATIC REQUIREMENTS]\n" +
			"🎯 Character's body proportions are NATURAL and REALISTIC - ZERO tolerance for distortion\n" +
			"🎯 The character is the EMOTIONAL CENTER - the narrative revolves around them\n" +
			"🎯 Cinematic composition with DRAMA and EMOTIONAL WEIGHT\n" +
			"🎯 Environmental storytelling - what is happening in this narrative moment?\n" +
			"🎯 Character action and emotion drive the scene - not posing, but ACTING\n" +
			"🎯 Single film frame - looks like ONE shot from ONE cinematic take\n" +
			"🎯 Film production aesthetic with cinematic color grading - not snapshot, not selfie\n" +
			"🎯 Dynamic cinematic framing - use negative space and composition for storytelling\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE CINEMATIC FRAME]\n" +
			"❌ ANY distortion of the character's proportions (stretched, compressed, squashed)\n" +
			"❌ Character looking pasted, floating, or artificially composited\n" +
			"❌ Static, stiff, portrait-style poses - this is a FILM SCENE, not a photoshoot\n" +
			"❌ Centered, flat, boring composition without cinematic drama\n" +
			"❌ Flat lighting that doesn't create film-quality mood and atmosphere"
	} else if hasProducts {
		// 오브젝트 샷 케이스 - 시네마틱 소품 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE CINEMATIC PROP REQUIREMENTS]\n" +
			"🎯 Showcase the objects as NARRATIVE PROPS with story weight\n" +
			"🎯 Cinematic arrangement - composition suggests film production value\n" +
			"🎯 Dramatic film lighting that creates mood and mystery\n" +
			"🎯 Environmental storytelling through prop placement and context\n" +
			"🎯 ALL items displayed with narrative purpose\n" +
			"🎯 Single film still - ONE shot from ONE cinematic frame\n" +
			"🎯 Film production aesthetic with cinematic color grading\n" +
			"🎯 Dynamic cinematic framing - use depth and negative space for storytelling\n\n" +
			"[FORBIDDEN - THESE WILL RUIN THE CINEMATIC PROP SHOT]\n" +
			"❌ ANY people, characters, or human figures in the frame\n" +
			"❌ Props looking pasted, floating, or artificially placed\n" +
			"❌ Boring, flat, catalog-style product layouts\n" +
			"❌ Cluttered composition without cinematic focal point\n" +
			"❌ Flat lighting that doesn't create film-quality depth and drama"
	} else {
		// 배경만 있는 케이스 - 시네마틱 로케이션 촬영 규칙
		criticalRules = commonForbidden + "\n[NON-NEGOTIABLE CINEMATIC LOCATION REQUIREMENTS]\n" +
			"🎯 Capture the pure cinematic atmosphere and narrative mood of the location\n" +
			"🎯 Dramatic film composition with depth and visual storytelling\n" +
			"🎯 Environmental storytelling - what narrative does this place suggest?\n" +
			"🎯 Film production aesthetic with cinematic color grading\n" +
			"🎯 Dynamic cinematic framing - use layers and negative space for depth\n\n" +
			"[FORBIDDEN]\n" +
			"❌ DO NOT add people, characters, or props to this establishing shot\n" +
			"❌ Flat, boring snapshot composition without cinematic drama"
	}

	// 16:9 비율 전용 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "16:9" {
		if hasModel {
			// 캐릭터 있는 16:9 케이스 - 시네마스코프 와이드샷
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC WIDE SHOT - FILM NARRATIVE STORYTELLING]\n" +
				"This is a WIDESCREEN FILM FRAME - use the horizontal space for powerful cinematic narrative.\n\n" +
				"🎬 DRAMATIC CINEMATIC WIDE COMPOSITION:\n" +
				"✓ Character positioned off-center (rule of thirds) creating cinematic tension\n" +
				"✓ Use the WIDESCREEN FORMAT to show narrative context and atmosphere\n" +
				"✓ Layers of depth - foreground elements, character, background environment\n" +
				"✓ Leading lines guide the eye to the character and story\n" +
				"✓ Negative space creates cinematic breathing room and emotional weight\n\n" +
				"🎬 CHARACTER INTEGRITY IN WIDESCREEN:\n" +
				"⚠️ The widescreen frame is NOT an excuse to distort proportions\n" +
				"⚠️ Character maintains NATURAL realistic proportions - scale to environment naturally\n" +
				"⚠️ Use the space to tell a NARRATIVE STORY, not to force-fit the character\n\n" +
				"🎬 FILM PRODUCTION EXECUTION:\n" +
				"✓ Cinematic lighting creates mood and drama across the widescreen frame\n" +
				"✓ Atmospheric perspective - distant elements create depth\n" +
				"✓ Film grain and cinematic color grading\n" +
				"✓ Depth of field emphasizes the character while establishing environment\n\n" +
				"GOAL: A breathtaking widescreen shot from a high-budget film production - \n" +
				"like Roger Deakins or Emmanuel Lubezki capturing a CINEMATIC MOMENT of narrative drama."
		} else if hasProducts {
			// 소품 샷 16:9 케이스 - 영화 소품
			aspectRatioInstruction = "\n\n[16:9 CINEMATIC PROP SHOT]\n" +
				"This is a WIDESCREEN PROP FRAME - use the horizontal space for narrative storytelling.\n\n" +
				"🎬 DRAMATIC WIDE PROP COMPOSITION:\n" +
				"✓ Props positioned cinematically using the full widescreen width\n" +
				"✓ Use the WIDESCREEN FORMAT to show narrative context and story atmosphere\n" +
				"✓ Layers of depth - foreground, props, background narrative elements\n" +
				"✓ Leading lines guide the eye to the key story props\n" +
				"✓ Negative space creates cinematic weight and narrative breathing room\n\n" +
				"🎬 FILM PRODUCTION EXECUTION:\n" +
				"✓ Cinematic lighting creates drama and reveals story details\n" +
				"✓ Atmospheric perspective adds film production depth\n" +
				"✓ Film grain and cinematic color grading\n" +
				"✓ Depth of field emphasizes narrative props while showing environment\n\n" +
				"GOAL: A stunning widescreen prop shot like high-budget film production still photography."
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
