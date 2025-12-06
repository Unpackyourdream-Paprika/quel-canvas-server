package cinema

import (
	"fmt"
	"strings"
)

// PromptCategories - 카테고리별 이미지 분류 구조체 (프롬프트 생성용)
type PromptCategories struct {
	Models      [][]byte // 모델 이미지 배열 (최대 3명)
	Clothing    [][]byte // 의류 이미지 배열 (top, pants, outer)
	Accessories [][]byte // 악세사리 이미지 배열 (shoes, bag, accessory)
	Background  []byte   // 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Cinema 모듈 전용 프롬프트 생성 (간소화 및 명확화 버전)
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석
	hasModels := len(categories.Models) > 0
	modelCount := len(categories.Models)
	hasClothing := len(categories.Clothing) > 0
	hasAccessories := len(categories.Accessories) > 0
	hasProducts := hasClothing || hasAccessories
	hasBackground := categories.Background != nil

	var promptBuilder strings.Builder

	// 1. [TASK DEFINITION] - 명확한 목표 설정
	promptBuilder.WriteString("[TASK]\n")
	if hasModels {
		promptBuilder.WriteString("Generate a photorealistic cinematic film still.\n")
		promptBuilder.WriteString(fmt.Sprintf("The scene MUST contain EXACTLY %d person(s).\n", modelCount))
	} else if hasProducts {
		promptBuilder.WriteString("Generate a photorealistic cinematic product shot.\n")
		promptBuilder.WriteString("NO people allowed. Focus on the objects.\n")
	} else {
		promptBuilder.WriteString("Generate a photorealistic cinematic environment shot.\n")
		promptBuilder.WriteString("NO people or specific products allowed. Focus on the location.\n")
	}
	promptBuilder.WriteString("\n")

	// 2. [SUBJECTS] - 인물/모델 상세 지시 (담백하게)
	if hasModels {
		promptBuilder.WriteString("[SUBJECTS - CRITICAL]\n")
		for i := range categories.Models {
			idx := i + 1
			promptBuilder.WriteString(fmt.Sprintf("%d. Person %d: Use Reference Image %d.\n", idx, idx, idx))
			promptBuilder.WriteString("   - FACE: Copy the face, age, gender, and ethnicity EXACTLY.\n")
			promptBuilder.WriteString("   - BODY: Observe the full body structure/shape in the reference and maintain it.\n")
			if modelCount > 1 {
				promptBuilder.WriteString("   - INTERACTION: Natural interaction with other subjects in the scene.\n")
			}
		}
		promptBuilder.WriteString("\n")
	}

	// 3. [ATTIRE & PROPS] - 의상 및 소품
	if hasClothing || hasAccessories {
		promptBuilder.WriteString("[ATTIRE & PROPS]\n")
		if hasClothing {
			promptBuilder.WriteString("- Wear the clothing shown in the Clothing Reference Images.\n")
		}
		if hasAccessories {
			promptBuilder.WriteString("- Include the accessories shown in the Accessory Reference Images.\n")
		}
		promptBuilder.WriteString("\n")
	}

	// 4. [ENVIRONMENT] - 배경 및 조명
	promptBuilder.WriteString("[ENVIRONMENT & LIGHTING]\n")
	if hasBackground {
		promptBuilder.WriteString("- LOCATION: Use the Background Reference Image as the location.\n")
		promptBuilder.WriteString("- LIGHTING: Match the lighting direction and mood of the background.\n")
		promptBuilder.WriteString("- INTEGRATION: Subjects must cast realistic shadows and interact with the environment.\n")
	} else {
		promptBuilder.WriteString("- LOCATION: Cinematic setting appropriate for the subject.\n")
		promptBuilder.WriteString("- LIGHTING: Professional cinematic lighting (rim light, key light, atmospheric).\n")
	}
	promptBuilder.WriteString("\n")

	// 5. [STYLE & COMPOSITION] - 스타일 (간결하게)
	promptBuilder.WriteString("[STYLE]\n")
	promptBuilder.WriteString("- 100% Photorealistic, 8k resolution, highly detailed.\n")
	promptBuilder.WriteString("- Film photography aesthetic (fine grain, natural colors).\n")
	if aspectRatio == "16:9" {
		promptBuilder.WriteString("- Wide cinematic aspect ratio. Use the width for atmospheric depth.\n")
	}
	promptBuilder.WriteString("\n")

	// 카테고리별 고정 스타일 가이드
	promptBuilder.WriteString("[CINEMATIC PHOTOGRAPHY STYLE GUIDE]\n")
	promptBuilder.WriteString("Cinematic scene. Natural lighting. Emotional depth. Film grain. Anamorphic lens. Professional composition.\n\n")
	promptBuilder.WriteString("[TECHNICAL CONSTRAINTS]\n")
	promptBuilder.WriteString("ABSOLUTELY NO VERTICAL COMPOSITION. ABSOLUTELY NO SIDE MARGINS. ABSOLUTELY NO WHITE/GRAY BARS ON LEFT OR RIGHT. Fill entire width from left edge to right edge. NO letterboxing. NO pillarboxing. NO empty sides.\n\n")

	// 6. [NEGATIVE CONSTRAINTS] - 절대 금지 사항 (핵심만)
	promptBuilder.WriteString("[STRICT NEGATIVE CONSTRAINTS]\n")
	promptBuilder.WriteString("- NO distorted faces or bodies.\n")
	promptBuilder.WriteString("- NO missing people (Must have exactly the number specified).\n")
	promptBuilder.WriteString("- NO extra people (Do not add random crowds).\n")
	promptBuilder.WriteString("- NO split screens, borders, or collage layouts.\n")
	promptBuilder.WriteString("- NO cartoon, illustration, or 3D render style. Must be PHOTO-REAL.\n\n")
	promptBuilder.WriteString("[ABSOLUTELY FORBIDDEN - IMAGE WILL BE REJECTED]:\n")
	promptBuilder.WriteString("- NO left-right split, NO side-by-side layout, NO duplicate subject on both sides\n")
	promptBuilder.WriteString("- NO grid, NO collage, NO comparison view, NO before/after layout\n")
	promptBuilder.WriteString("- NO vertical dividing line, NO center split, NO symmetrical duplication\n")
	promptBuilder.WriteString("- NO white/gray borders, NO letterboxing, NO empty margins on any side\n")
	promptBuilder.WriteString("- NO multiple identical poses, NO mirrored images, NO panel divisions\n")
	promptBuilder.WriteString("- NO vertical portrait orientation with side margins\n\n")
	promptBuilder.WriteString("[REQUIRED - MUST GENERATE THIS WAY]:\n")
	promptBuilder.WriteString("- ONE single continuous photograph taken with ONE camera shutter\n")
	promptBuilder.WriteString("- ONE unified moment in time - NOT two or more moments combined\n")
	promptBuilder.WriteString("- FILL entire frame edge-to-edge with NO empty space\n")
	promptBuilder.WriteString("- Natural asymmetric composition - left side MUST be different from right side\n")
	promptBuilder.WriteString("- Professional editorial style - real single-shot photography only\n")

	// 7. [USER INSTRUCTION] - 사용자 입력 (최우선 적용)
	if userPrompt != "" {
		promptBuilder.WriteString("\n[ADDITIONAL INSTRUCTION]\n")
		promptBuilder.WriteString(userPrompt)
	}

	return promptBuilder.String()
}
