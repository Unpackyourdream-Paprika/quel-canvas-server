package cartoon

import (
	"fmt"
	"strings"
)

// PromptCategories - 카테고리별 이미지 분류 구조체 (프롬프트 생성용)
type PromptCategories struct {
	Character  [][]byte // 캐릭터 이미지 배열 (최대 3명) - 웹툰/만화 캐릭터
	Prop       [][]byte // 소품/아이템 이미지 배열
	Background []byte   // 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Cartoon 모듈 전용 프롬프트 생성
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석
	hasCharacter := len(categories.Character) > 0
	characterCount := len(categories.Character)
	hasProp := len(categories.Prop) > 0
	hasBackground := categories.Background != nil

	// 메인 지시사항 - 간소화
	var mainInstruction string
	if hasCharacter {
		if characterCount == 1 {
			mainInstruction = "[IMAGE GENERATION]\nCreate ONE unified illustration with the character.\n\n"
		} else {
			mainInstruction = fmt.Sprintf("[IMAGE GENERATION - %d CHARACTERS]\n"+
				"Create ONE unified illustration with %d DISTINCT CHARACTERS.\n"+
				"Each character MUST appear exactly as shown in their reference.\n\n", characterCount, characterCount)
		}
	} else if hasProp {
		mainInstruction = "[PRODUCT IMAGE]\nCreate ONE illustration showcasing the products. NO people.\n\n"
	} else {
		mainInstruction = "[ENVIRONMENT IMAGE]\nCreate ONE illustration of the environment. NO people or products.\n\n"
	}

	// 참조 이미지 설명 - 간소화
	var instructions []string
	imageIndex := 1

	for i := range categories.Character {
		if len(categories.Character) == 1 {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (CHARACTER): Use this character's appearance exactly.", imageIndex))
		} else {
			instructions = append(instructions,
				fmt.Sprintf("Reference Image %d (CHARACTER %d): Use this character's appearance exactly.", imageIndex, i+1))
		}
		imageIndex++
	}

	if len(categories.Prop) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (PROPS): Include all these items.", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (BACKGROUND): Use this exact background.", imageIndex))
		imageIndex++
	}

	// 사용자 프롬프트 - 최우선
	var userDescription string
	if userPrompt != "" {
		userDescription = "\n[USER REQUEST]\n" + userPrompt + "\n"
	}

	// 구성 지시 - 간소화
	var compositionInstruction string
	if hasCharacter {
		compositionInstruction = "\n[COMPOSITION]\nGenerate ONE illustration with the character(s)."
		if hasBackground {
			compositionInstruction += " Use the exact background from reference."
		}
	} else if hasProp {
		compositionInstruction = "\n[COMPOSITION]\nGenerate ONE product illustration. NO people."
	} else if hasBackground {
		compositionInstruction = "\n[COMPOSITION]\nGenerate ONE environment illustration. NO people or products."
	}

	// 최소한의 요구사항
	criticalRules := "\n\n[REQUIREMENTS]\n" +
		"• ONE single unified image\n" +
		"• NO split-screen or collage\n" +
		"• Character integrated naturally (not pasted)\n"

	// 최종 조합
	finalPrompt := mainInstruction +
		strings.Join(instructions, "\n") +
		userDescription +
		compositionInstruction +
		criticalRules

	return finalPrompt
}
