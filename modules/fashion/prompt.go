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
		mainInstruction = "[FASHION EDITORIAL PHOTOGRAPHER]\n" +
			"You are a fashion photographer shooting an editorial campaign.\n" +
			"This is SOLO FASHION MODEL photography - ONLY ONE PERSON in the frame.\n" +
			"The PERSON is the HERO - their natural proportions are SACRED.\n\n" +
			"Create ONE photorealistic photograph:\n" +
			"• ONLY ONE MODEL - solo fashion shoot\n" +
			"• FULL BODY SHOT - model's ENTIRE body from head to TOE visible\n" +
			"• FEET MUST BE VISIBLE - both feet and shoes completely in frame\n" +
			"• SERIOUS FACIAL EXPRESSION - stern/fierce/intense gaze, NO SMILING\n" +
			"• STRONG POSTURE - elongated body lines, poised stance\n" +
			"• The model wears ALL clothing and accessories\n" +
			"• Use the EXACT background from the reference image\n\n"
	} else if hasProducts {
		// 프로덕트만 → 프로덕트 포토그래피
		mainInstruction = "[PRODUCT PHOTOGRAPHER]\n" +
			"You are a product photographer creating still life.\n" +
			"The PRODUCTS are the STARS.\n" +
			"⚠️ CRITICAL: NO people or models in this shot - products only.\n\n" +
			"Create ONE photorealistic photograph:\n" +
			"• Artistic arrangement of all items\n" +
			"• Good lighting that highlights textures\n" +
			"• Use the EXACT background from the reference if provided\n\n"
	} else {
		// 배경만 → 환경 포토그래피
		mainInstruction = "[ENVIRONMENTAL PHOTOGRAPHER]\n" +
			"You are a photographer capturing atmosphere.\n" +
			"⚠️ CRITICAL: NO people, models, or products in this shot - environment only.\n\n" +
			"Create ONE photorealistic photograph of the referenced environment.\n\n"
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
			fmt.Sprintf("Reference Image %d (CLOTHING): ALL visible garments - tops, bottoms, dresses, outerwear. The person MUST wear EVERY piece shown here", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (ACCESSORIES): ALL items - shoes, bags, hats, glasses, jewelry. The person MUST wear/carry EVERY item shown here", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (BACKGROUND - MUST USE EXACTLY): ⚠️ CRITICAL: You MUST use this EXACT background. If it is a white/gray studio, use a WHITE/GRAY STUDIO. If it is an outdoor location, use that EXACT outdoor location. DO NOT invent a different background. The background must match the reference image 100%%.", imageIndex))
		imageIndex++
	}

	// 구성 지시사항
	var compositionInstruction string

	// 케이스 1: 모델 이미지가 있는 경우
	if hasModel {
		compositionInstruction = "\n[FASHION EDITORIAL COMPOSITION]\n" +
			"Generate ONE photorealistic photograph showing the referenced model wearing the complete outfit."
	} else if hasProducts {
		// 케이스 2: 모델 없이 의상/액세서리만
		compositionInstruction = "\n[PRODUCT PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic product photograph showcasing the clothing and accessories as OBJECTS.\n" +
			"⚠️ DO NOT add any people, models, or human figures.\n"

		if hasBackground {
			compositionInstruction += "The products are placed naturally within the referenced environment."
		} else {
			compositionInstruction += "Create a studio product shot with professional lighting."
		}
	} else if hasBackground {
		// 케이스 3: 배경만
		compositionInstruction = "\n[ENVIRONMENTAL PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic photograph of the referenced environment.\n" +
			"⚠️ DO NOT add any people, models, or products to this scene."
	} else {
		// 케이스 4: 아무것도 없는 경우
		compositionInstruction = "\n[COMPOSITION]\n" +
			"Generate a high-quality photorealistic image based on the references provided."
	}

	// 배경 관련 지시사항 - 모델이 있을 때만 추가
	if hasModel && hasBackground {
		// 모델 + 배경 케이스 → 배경 레퍼런스에 집중
		compositionInstruction += " in the EXACT background from the reference image.\n\n" +
			"[BACKGROUND - MUST MATCH REFERENCE]\n" +
			"⚠️ CRITICAL: The background MUST match the reference image EXACTLY.\n" +
			"⚠️ If the reference shows a WHITE STUDIO, use a WHITE STUDIO.\n" +
			"⚠️ If the reference shows a GRAY STUDIO, use a GRAY STUDIO.\n" +
			"⚠️ If the reference shows an outdoor location, use that EXACT location.\n" +
			"⚠️ DO NOT invent backgrounds. DO NOT add locations not in the reference.\n\n" +
			"[SUBJECT INTEGRATION]\n" +
			"✓ Place the subject naturally in the referenced background\n" +
			"✓ Lighting must match the background reference\n" +
			"✓ Natural shadows consistent with the background\n" +
			"✓ The subject and background must look like ONE unified photograph"
	} else if hasModel && !hasBackground {
		// 모델만 있고 배경 없음 → 기본 스튜디오
		compositionInstruction += " in a clean studio setting with professional lighting."
	}

	// 핵심 요구사항
	var criticalRules string

	// 공통 금지사항
	commonForbidden := "\n\n[CRITICAL: FORBIDDEN]\n\n" +
		"⚠️ NO SPLIT/DUAL COMPOSITION:\n" +
		"❌ NO vertical dividing lines\n" +
		"❌ NO left-right split layouts\n" +
		"❌ NO duplicate subject on both sides\n" +
		"❌ NO grid or collage\n" +
		"❌ ONE continuous scene only\n\n" +
		"⚠️ ONLY ONE PERSON:\n" +
		"❌ NO multiple models\n" +
		"❌ NO background people\n" +
		"❌ This is SOLO photography\n\n" +
		"[REQUIRED]:\n" +
		"✓ ONE single photograph\n" +
		"✓ ONE unified moment\n" +
		"✓ Fill entire frame - NO empty margins\n" +
		"✓ Natural asymmetric composition\n"

	if hasModel {
		// 모델 있는 케이스
		criticalRules = commonForbidden + "\n[FASHION EDITORIAL REQUIREMENTS]\n" +
			"🎯 ONLY ONE MODEL in the photograph\n" +
			"🎯 SERIOUS FACIAL EXPRESSION - fierce/stern/intense (NO SMILING)\n" +
			"🎯 FULL BODY SHOT - head to TOE visible\n" +
			"🎯 FEET MUST BE VISIBLE - both feet in frame\n" +
			"🎯 ALL clothing and accessories worn\n" +
			"🎯 Use EXACT background from reference\n\n" +
			"[FORBIDDEN]\n" +
			"❌ SMILING - model must be serious\n" +
			"❌ CROPPED FEET - feet must be visible\n" +
			"❌ WRONG BACKGROUND - must match reference exactly\n" +
			"❌ Multiple people\n" +
			"❌ Distorted proportions"
	} else if hasProducts {
		// 프로덕트 샷 케이스
		criticalRules = commonForbidden + "\n[PRODUCT REQUIREMENTS]\n" +
			"🎯 Showcase products beautifully\n" +
			"🎯 Good lighting\n" +
			"🎯 ALL items displayed clearly\n" +
			"🎯 Use EXACT background from reference\n\n" +
			"[FORBIDDEN]\n" +
			"❌ ANY people or models\n" +
			"❌ Products looking pasted"
	} else {
		// 배경만 있는 케이스
		criticalRules = commonForbidden + "\n[ENVIRONMENT REQUIREMENTS]\n" +
			"🎯 Capture the atmosphere of the location\n\n" +
			"[FORBIDDEN]\n" +
			"❌ DO NOT add people or products"
	}

	// aspect ratio별 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "9:16" {
		if hasModel {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL FORMAT]\n" +
				"✓ Model's ENTIRE BODY from head to TOE must fit\n" +
				"✓ FEET MUST BE VISIBLE at bottom\n" +
				"✓ Leave space below feet\n" +
				"✓ Use EXACT background from reference"
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL PRODUCT SHOT]\n" +
				"✓ Products arranged vertically\n" +
				"✓ Use EXACT background from reference"
		} else {
			aspectRatioInstruction = "\n\n[9:16 VERTICAL SHOT]\n" +
				"✓ Use the HEIGHT to capture vertical elements"
		}
	} else if aspectRatio == "16:9" {
		if hasModel {
			aspectRatioInstruction = "\n\n[16:9 WIDE FORMAT]\n" +
				"✓ Model's ENTIRE BODY from head to TOE must be visible\n" +
				"✓ FEET MUST BE VISIBLE at bottom\n" +
				"✓ Subject positioned using rule of thirds\n" +
				"✓ Use EXACT background from reference\n\n" +
				"⚠️ BACKGROUND RULE:\n" +
				"⚠️ If reference shows WHITE/GRAY STUDIO, use WHITE/GRAY STUDIO\n" +
				"⚠️ If reference shows outdoor location, use that EXACT location\n" +
				"⚠️ DO NOT invent locations not in reference"
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[16:9 WIDE PRODUCT SHOT]\n" +
				"✓ Products positioned using the full width\n" +
				"✓ Use EXACT background from reference"
		} else {
			aspectRatioInstruction = "\n\n[16:9 WIDE SHOT]\n" +
				"✓ Use the full WIDTH to capture the environment"
		}
	} else {
		// 1:1 및 기타 비율
		if hasModel {
			aspectRatioInstruction = "\n\n[SQUARE FORMAT]\n" +
				"✓ Model's ENTIRE BODY from head to TOE must fit\n" +
				"✓ FEET MUST BE VISIBLE at bottom\n" +
				"✓ Balanced composition\n" +
				"✓ Use EXACT background from reference"
		} else if hasProducts {
			aspectRatioInstruction = "\n\n[SQUARE PRODUCT SHOT]\n" +
				"✓ Balanced product arrangement\n" +
				"✓ Use EXACT background from reference"
		} else {
			aspectRatioInstruction = "\n\n[SQUARE SHOT]\n" +
				"✓ Balanced composition"
		}
	}

	// ⚠️ 최우선 지시사항
	criticalHeader := "⚠️ CRITICAL REQUIREMENTS ⚠️\n\n" +
		"[MANDATORY - FEET VISIBLE]:\n" +
		"🚨 BOTH FEET MUST APPEAR IN FRAME\n" +
		"🚨 DO NOT CROP AT ANKLES OR CALVES\n" +
		"🚨 FULL BODY means HEAD TO TOE\n\n" +
		"[MANDATORY - FACIAL EXPRESSION]:\n" +
		"🚨 MODEL MUST NOT SMILE\n" +
		"🚨 SERIOUS/STERN/FIERCE expression only\n" +
		"🚨 NO happy expression, NO grin, NO teeth showing\n\n" +
		"[MANDATORY - BACKGROUND]:\n" +
		"🚨 USE EXACT BACKGROUND FROM REFERENCE\n" +
		"🚨 If reference is WHITE STUDIO, use WHITE STUDIO\n" +
		"🚨 If reference is GRAY STUDIO, use GRAY STUDIO\n" +
		"🚨 DO NOT invent outdoor/urban/nature locations\n\n" +
		"[FORBIDDEN]:\n" +
		"❌ NO split layouts, NO grid, NO collage\n" +
		"❌ NO multiple people\n" +
		"❌ NO smiling\n" +
		"❌ NO cropped feet\n" +
		"❌ NO wrong background\n\n"

	// 최종 조합
	var finalPrompt string

	if userPrompt != "" {
		finalPrompt = criticalHeader + "[USER REQUEST]\n" + userPrompt + "\n\n"
	} else {
		finalPrompt = criticalHeader
	}

	// 카테고리별 스타일 가이드
	categoryStyleGuide := "\n\n[STYLE GUIDE]\n" +
		"Fashion photography style. Professional lighting. High-end editorial composition.\n\n" +
		"[TECHNICAL]\n" +
		"Fill entire frame. NO empty margins. NO letterboxing.\n"

	finalPrompt += mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + categoryStyleGuide + criticalRules + aspectRatioInstruction

	return finalPrompt
}
