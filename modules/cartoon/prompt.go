package cartoon

import (
	"fmt"
	"strings"
)

// PromptCategories - 카테고리별 이미지 분류 구조체 (프롬프트 생성용)
type PromptCategories struct {
	Models      [][]byte // 캐릭터 이미지 배열 (최대 3명) - 웹툰/만화 캐릭터
	Clothing    [][]byte // 의상 이미지 배열 (상의, 하의, 겉옷)
	Accessories [][]byte // 소품/아이템 이미지 배열 (신발, 가방, 액세서리)
	Background  []byte   // 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Cartoon 모듈 전용 프롬프트 생성 (웹툰, 만화, 애니메이션)
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석을 위한 변수 정의
	hasModels := len(categories.Models) > 0
	modelCount := len(categories.Models)
	hasClothing := len(categories.Clothing) > 0
	hasAccessories := len(categories.Accessories) > 0
	hasBackground := categories.Background != nil

	// 웹툰 스타일 기본 지시사항 (간소화)
	var baseStyle string
	if modelCount <= 1 {
		baseStyle = "[WEBTOON/MANGA ILLUSTRATION STYLE]\n" +
			"Generate webtoon/manga style character illustration.\n" +
			"Style: Clean linework, vibrant saturated colors, cel-shading or flat shading.\n" +
			"2D comic art aesthetic - NOT photorealistic, NOT 3D render.\n\n"
	} else {
		baseStyle = fmt.Sprintf("[WEBTOON/MANGA ILLUSTRATION STYLE - %d CHARACTERS]\n"+
			"Generate webtoon/manga style illustration with MULTIPLE CHARACTERS.\n"+
			"Each CHARACTER must appear exactly as shown in their reference image.\n"+
			"Style: Clean linework, vibrant saturated colors, cel-shading or flat shading.\n"+
			"2D comic art aesthetic - NOT photorealistic, NOT 3D render.\n\n", modelCount)
	}

	// 참조 이미지 설명 - 다중 캐릭터 지원
	var instructions []string
	imageIndex := 1

	for i := range categories.Models {
		if len(categories.Models) == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (CHARACTER APPEARANCE): Use this person's face, hairstyle, body features as character reference", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (CHARACTER %d APPEARANCE): Use this person's face, hairstyle, body features as CHARACTER %d reference - MUST appear exactly as shown", imageIndex, i+1, i+1))
		}
		imageIndex++
	}

	if len(categories.Clothing) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (OUTFIT): Character wears ALL these garments", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (ITEMS): Character wears/carries ALL these items", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (SCENE BACKGROUND): Recreate this setting/atmosphere in webtoon background art style", imageIndex))
		imageIndex++
	}

	// 사용자 프롬프트 (캐릭터 묘사) - 최우선
	var characterDescription string
	if userPrompt != "" {
		characterDescription = "\n[CHARACTER DESCRIPTION - USER INPUT]\n" + userPrompt + "\n"
	}

	// 기본 구성 지시
	var compositionInstruction string
	if hasModels {
		if modelCount == 1 {
			compositionInstruction = "\n[COMPOSITION]\n" +
				"Generate ONE webtoon/manga character illustration.\n" +
				"Character wears complete outfit (all clothing + accessories).\n"
		} else {
			compositionInstruction = fmt.Sprintf("\n[COMPOSITION - %d CHARACTERS]\n"+
				"Generate ONE webtoon/manga illustration with %d DISTINCT CHARACTERS.\n"+
				"Each character MUST appear exactly as shown in their reference image.\n"+
				"Characters interact naturally within the same scene.\n", modelCount, modelCount)
		}
		if hasBackground {
			compositionInstruction += "Character(s) positioned naturally in the scene background.\n"
		}
	} else if hasClothing || hasAccessories {
		compositionInstruction = "\n[COMPOSITION]\n" +
			"Generate webtoon-style item/prop illustration.\n" +
			"⚠️ NO characters - items only.\n"
	} else if hasBackground {
		compositionInstruction = "\n[COMPOSITION]\n" +
			"Generate webtoon-style background scene.\n" +
			"⚠️ NO characters - background only.\n"
	}

	// 필수 금지사항 (최소화)
	criticalRules := "\n[CRITICAL REQUIREMENTS]\n" +
		"✓ Webtoon/manga illustration style (clean lines, vibrant colors, cel-shading)\n" +
		"✓ ONE unified comic panel - no split/collage layouts\n" +
		"✓ Character wears all referenced clothing/items\n" +
		"❌ NO photorealistic rendering\n" +
		"❌ NO vertical dividing lines or panel splits\n"

	// Aspect ratio 기본 지시
	var aspectRatioHint string
	if aspectRatio == "9:16" {
		aspectRatioHint = "\n[FORMAT: 9:16 Vertical] Show character in vertical webtoon panel composition.\n"
	} else if aspectRatio == "16:9" {
		aspectRatioHint = "\n[FORMAT: 16:9 Wide] Wide webtoon scene composition.\n"
	} else {
		aspectRatioHint = "\n[FORMAT: Square] Balanced webtoon character composition.\n"
	}

	// 최종 조합: 스타일 → 참조 이미지 → 캐릭터 묘사(사용자) → 구성 → 금지사항 → 비율
	finalPrompt := baseStyle +
		strings.Join(instructions, "\n") +
		characterDescription +
		compositionInstruction +
		criticalRules +
		aspectRatioHint

	return finalPrompt
}
