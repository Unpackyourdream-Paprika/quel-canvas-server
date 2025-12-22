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
	log.Printf("[Beauty Prompt] Model:%v, Product:%d, BG:%v",
		hasModel, len(categories.Product), hasBackground)

	// 케이스별 메인 지시사항
	var mainInstruction string
	if hasModel && hasProduct {
		// 모델 + 제품 → 코스메틱 CF/광고 샷
		mainInstruction = "[ARRI ALEXA 35 COSMETIC COMMERCIAL]\n\n" +
			"You are a cinematographer shooting a luxury cosmetic ad with ARRI ALEXA 35.\n" +
			"Create ONE cinematic photograph of a model holding a cosmetic product.\n\n" +
			"[PRIORITY 1 - MODEL]: Keep face identity. Apply cinematic lighting.\n" +
			"[PRIORITY 2 - PRODUCT]: Keep shape/color/logo. Redraw with scene lighting. Must look 3D.\n" +
			"[PRIORITY 3 - BACKGROUND]: Reference is just inspiration. Create any fitting environment.\n\n" +
			"[PHYSICS - ABSOLUTE RULES]:\n" +
			"- Product must be held in hand, resting on surface, or touching something. NEVER floating in air.\n" +
			"- Product size must be realistic - cosmetics are small, hand-held items.\n" +
			"- Gravity applies. Shadows exist. Reflections are real.\n" +
			"- Human anatomy must be correct - no extra fingers, no distorted limbs.\n\n" +
			"[UNIFIED LIGHTING]:\n" +
			"One light source for entire scene. Model, product, background - all same direction.\n" +
			"Product's original studio lighting must be replaced with scene lighting.\n\n" +
			"[ARRI ALEXA LOOK]:\n" +
			"Rich shadows, smooth highlights, organic skin tones, cinematic depth of field.\n" +
			"Film-like color grade, subtle grain, premium commercial quality.\n\n"
	} else if hasModel {
		// 모델만 있음 → 뷰티 포트레이트 (ARRI ALEXA 시네마틱)
		mainInstruction = "[ARRI ALEXA 35 CINEMATIC BEAUTY PORTRAIT]\n\n" +
			"You are shooting a high-end beauty editorial with ARRI ALEXA 35 camera.\n\n" +
			"[FACE IDENTITY PRESERVATION - CRITICAL]:\n" +
			"The model's face must be IDENTICAL to the reference - same eyes, nose, lips, face shape.\n" +
			"Same skin tone, same ethnicity, same age. The viewer must recognize this as the SAME PERSON.\n" +
			"Do not beautify or modify the face - keep it EXACTLY as reference.\n\n" +
			"[ARRI ALEXA FILM LOOK]:\n" +
			"- ARRI LogC to Rec709 with film print emulation\n" +
			"- Rich shadows, smooth highlight rolloff, organic skin tones\n" +
			"- Cinematic depth of field with natural bokeh\n" +
			"- Soft, flattering beauty lighting (butterfly or loop lighting)\n" +
			"- Subtle film grain, cohesive color grade\n\n"
	} else if hasProduct {
		// 프로덕트만 → 뷰티 프로덕트 (ARRI ALEXA 시네마틱)
		productCount := len(categories.Product)
		var productCountInstruction string

		switch productCount {
		case 1:
			productCountInstruction = "RECREATE the product EXACTLY as shown in the reference - same color, shape, packaging.\n"
		case 2:
			productCountInstruction = "RECREATE EXACTLY 2 products - both items with EXACT colors and shapes.\n"
		case 3:
			productCountInstruction = "RECREATE EXACTLY 3 products - all three items with EXACT colors and shapes.\n"
		case 4:
			productCountInstruction = "RECREATE EXACTLY 4 products - all four items with EXACT colors and shapes. Arrange naturally, NOT as a grid.\n"
		default:
			productCountInstruction = fmt.Sprintf("RECREATE EXACTLY %d products - ALL items with EXACT colors and shapes.\n", productCount)
		}

		mainInstruction = "[ARRI ALEXA 35 CINEMATIC PRODUCT PHOTOGRAPHY]\n\n" +
			"You are shooting a luxury cosmetic commercial with ARRI ALEXA 35 camera.\n" +
			"NO people or models - beauty products only.\n\n" +
			"[PRODUCT RECREATION - CRITICAL]:\n" +
			productCountInstruction +
			"Match colors, shapes, packaging designs EXACTLY from the reference.\n\n" +
			"[ARRI ALEXA FILM LOOK]:\n" +
			"- ARRI LogC to Rec709 with film print emulation\n" +
			"- Rich shadows, smooth highlights, cinematic depth\n" +
			"- Soft, diffused lighting highlighting product details\n" +
			"- Premium cosmetic brand photography style\n" +
			"- ONE UNIFIED PHOTOGRAPH, not a composite\n\n"
	} else {
		// 배경만 → 환경 포토그래피 (ARRI ALEXA 시네마틱)
		mainInstruction = "[ARRI ALEXA 35 CINEMATIC ENVIRONMENT]\n\n" +
			"You are shooting a beauty photography backdrop with ARRI ALEXA 35 camera.\n" +
			"NO people, models, or products - environment only.\n\n" +
			"[ARRI ALEXA FILM LOOK]:\n" +
			"- ARRI LogC to Rec709 with film print emulation\n" +
			"- Soft, flattering lighting suitable for beauty photography\n" +
			"- Clean, elegant composition with subtle depth\n" +
			"- Cinematic atmosphere perfect as a beauty backdrop\n\n"
	}

	var instructions []string
	imageIndex := 1

	// 각 카테고리별 명확한 설명 (Beauty-specific)
	if categories.Model != nil {
		if hasProduct {
			instructions = append(instructions,
				fmt.Sprintf("Image %d (MODEL): Face reference. Keep identity. Relight to match background.", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Image %d (MODEL): Face identity reference. Keep same face.", imageIndex))
		}
		imageIndex++
	}

	if len(categories.Product) > 0 {
		productCount := len(categories.Product)
		if hasModel {
			instructions = append(instructions,
				fmt.Sprintf("Image %d (PRODUCT): Shape and color reference ONLY. Redraw with background lighting.", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Image %d (PRODUCTS - %d items): Recreate exact products. Arrange naturally.", imageIndex, productCount))
		}
		imageIndex++
	}

	if categories.Background != nil {
		if hasModel && hasProduct {
			instructions = append(instructions,
				fmt.Sprintf("Image %d (BACKGROUND): Just inspiration. Create your own environment.", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Image %d (BACKGROUND): Atmosphere reference.", imageIndex))
		}
		imageIndex++
	}

	// 시네마틱 구성 지시사항
	var compositionInstruction string

	// 케이스 1: 모델 + 제품 → 코스메틱 CF 샷
	if hasModel && hasProduct {
		compositionInstruction = "\n[COMPOSITION]:\n" +
			"Model holds product naturally. Environment surrounds them.\n" +
			"Product has volume and depth - highlights, shadows, reflections.\n" +
			"Fill entire frame. No empty borders.\n\n"
	} else if hasModel {
		// 케이스 2: 모델만 → 뷰티 포트레이트
		compositionInstruction = "\n[BEAUTY PORTRAIT COMPOSITION]\n" +
			"Generate ONE photorealistic beauty portrait - face and shoulders.\n" +
			"Close-up composition with face filling 60-80% of frame.\n" +
			"Soft, flattering beauty lighting.\n"

		if hasBackground {
			compositionInstruction += "\n[ENVIRONMENT INTEGRATION]:\n" +
				"- Generate environment inspired by background reference\n" +
				"- Shallow depth of field: face sharp, background soft but recognizable\n" +
				"- Environmental light creates natural rim/fill light on model\n"
		} else {
			compositionInstruction += "\nClean studio background (white, grey, or neutral).\n"
		}
	} else if hasProduct {
		// 케이스 3: 제품만 → 프로덕트 샷
		compositionInstruction = "\n[PRODUCT PHOTOGRAPHY COMPOSITION]\n" +
			"Generate ONE photorealistic product photograph.\n" +
			"Recreate EXACT products from reference - NO people, NO hands.\n" +
			"Arrange products naturally, not as grid.\n"

		if hasBackground {
			compositionInstruction += "\n[ENVIRONMENT INTEGRATION]:\n" +
				"- Generate environment inspired by background reference\n" +
				"- Products cast realistic contact shadows on surface\n" +
				"- Environment light reflects naturally on product surfaces\n" +
				"- Background colors subtly bleed onto products for integration\n"
		} else {
			compositionInstruction += "\nStudio setting with soft, diffused lighting.\n"
		}
	} else if hasBackground {
		// 케이스 4: 배경만
		compositionInstruction = "\n[ENVIRONMENT PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic environment photograph.\n" +
			"NO people, NO products - pure atmospheric shot.\n"
	} else {
		compositionInstruction = "\n[CINEMATIC COMPOSITION]\n" +
			"Generate a high-quality photorealistic image.\n"
	}

	// 핵심 요구사항 - 간결하게
	var criticalRules string

	commonRules := "\n\n[OUTPUT - FILL ENTIRE FRAME]\n" +
		"Image must touch all 4 edges. NO empty space.\n" +
		"NO black bars on left/right. NO letterbox on top/bottom.\n" +
		"Content fills 100% of the canvas.\n"

	if hasModel && hasProduct {
		criticalRules = commonRules +
			"\n[CRITICAL]:\n" +
			"Product lighting = Background lighting = Model lighting.\n" +
			"Product must be 3D with proper highlights and shadows.\n" +
			"No flat, pasted, or sticker-like products.\n"
	} else if hasModel {
		criticalRules = commonRules +
			"\n[BEAUTY PORTRAIT]\n" +
			"- Close-up: face fills 60-80% of frame\n" +
			"- Natural facial features and skin texture\n"
	} else if hasProduct {
		criticalRules = commonRules +
			"\n[PRODUCT SHOT]\n" +
			"- Recreate EXACT products from reference\n" +
			"- NO people, NO hands\n" +
			"- Natural arrangement, not grid\n" +
			"- Realistic shadows and reflections\n"
	} else {
		criticalRules = commonRules +
			"\n[ENVIRONMENT]\n" +
			"- NO people, NO products\n"
	}

	// 16:9 비율 지시사항 - 간결하게
	var aspectRatioInstruction string
	if aspectRatio == "16:9" {
		aspectRatioInstruction = "\n\n[16:9 WIDE FORMAT]\n" +
			"Use horizontal space for cinematic composition.\n" +
			"Rule of thirds positioning, negative space for atmosphere.\n"
	}

	// 최종 조합
	finalPrompt := mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + criticalRules + aspectRatioInstruction

	if userPrompt != "" {
		finalPrompt += "\n\n[STYLING]\n" + userPrompt
	}

	return finalPrompt
}
