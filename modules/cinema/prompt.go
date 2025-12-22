package cinema

import (
	"fmt"
	"strings"
)

// PromptCategories - Cinema 모듈 전용 이미지 분류 구조체 (프롬프트 생성용)
// 프론트 type: actor, face, top, pants, outer, prop, background
type PromptCategories struct {
	Actor      [][]byte // Actor/Face 이미지 배열 (최대 3명)
	Clothing   [][]byte // 의류 이미지 배열 (top, pants, outer)
	Prop       [][]byte // Prop (소품) 이미지 배열
	Background []byte   // 배경 이미지 (최대 1장)
}

// GenerateDynamicPrompt - Cinema 모듈 전용 프롬프트 생성 (ARRI ALEXA 스타일)
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	// 케이스 분석 (프론트 type 기준)
	hasActor := len(categories.Actor) > 0
	actorCount := len(categories.Actor)
	hasClothing := len(categories.Clothing) > 0
	hasProp := len(categories.Prop) > 0
	hasProducts := hasClothing || hasProp
	hasBackground := categories.Background != nil

	var promptBuilder strings.Builder

	// 최우선 규칙 - 통합된 씬
	if hasActor {
		promptBuilder.WriteString("=== CREATE ONE UNIFIED SCENE ===\n\n")
		promptBuilder.WriteString("[THE GOAL]\n")
		promptBuilder.WriteString("Place the actor INTO the environment. They must look like they BELONG there.\n")
		promptBuilder.WriteString("The environment's light must fall on the actor's face and body.\n")
		promptBuilder.WriteString("The actor must cast shadows in the environment.\n\n")
		promptBuilder.WriteString("[FACE - KEEP IDENTITY]\n")
		promptBuilder.WriteString("Same face features (eyes, nose, lips, face shape).\n")
		promptBuilder.WriteString("BUT the face must be LIT by the environment's lighting.\n")
		promptBuilder.WriteString("If background has warm orange light, face has warm orange light.\n")
		promptBuilder.WriteString("If background has blue neon, face reflects blue neon.\n\n")
		promptBuilder.WriteString("[BODY - CORRECT PROPORTIONS]\n")
		promptBuilder.WriteString("Natural human proportions. No stretched or distorted limbs.\n\n")
		promptBuilder.WriteString("=================================\n\n")
	}

	// 메인 지시사항
	promptBuilder.WriteString("[ARRI ALEXA 35 CINEMATIC FILM STILL]\n\n")
	promptBuilder.WriteString("You are a cinematographer shooting a film scene with ARRI ALEXA 35.\n")
	if hasActor {
		promptBuilder.WriteString(fmt.Sprintf("Create ONE cinematic photograph with EXACTLY %d person(s).\n\n", actorCount))
	} else if hasProducts {
		promptBuilder.WriteString("Create ONE cinematic product shot. NO people.\n\n")
	} else {
		promptBuilder.WriteString("Create ONE cinematic environment shot. NO people, NO products.\n\n")
	}

	// 우선순위
	if hasActor {
		promptBuilder.WriteString("[PRIORITY 1 - FACE]: IDENTICAL to reference. No changes allowed.\n")
		promptBuilder.WriteString("[PRIORITY 2 - BODY]: Natural human proportions. No distortion.\n")
	}
	if hasProducts {
		promptBuilder.WriteString("[PRIORITY 3 - CLOTHING/PROPS]: Keep design. Redraw with scene lighting.\n")
	}
	promptBuilder.WriteString("[PRIORITY LAST - BACKGROUND]: Do NOT copy. Create a completely NEW environment. Reference is only for mood.\n\n")

	// 통일된 조명
	promptBuilder.WriteString("[UNIFIED LIGHTING]:\n")
	promptBuilder.WriteString("One light source for entire scene. Actor, props, background - all same direction.\n")
	promptBuilder.WriteString("Clothing/prop's original lighting must be replaced with scene lighting.\n\n")

	// ARRI ALEXA 룩
	promptBuilder.WriteString("[ARRI ALEXA LOOK]:\n")
	promptBuilder.WriteString("Rich shadows, smooth highlights, organic skin tones, cinematic depth of field.\n")
	promptBuilder.WriteString("Film-like color grade, subtle grain, premium cinema quality.\n\n")

	// 레퍼런스 이미지 설명
	promptBuilder.WriteString("[REFERENCE IMAGES]:\n")
	imageIndex := 1
	if hasActor {
		for i := 0; i < actorCount; i++ {
			promptBuilder.WriteString(fmt.Sprintf("Image %d (ACTOR %d): FACE is sacred - keep EXACTLY. Body proportions must be correct.\n", imageIndex, i+1))
			imageIndex++
		}
	}
	if hasClothing {
		promptBuilder.WriteString(fmt.Sprintf("Image %d (CLOTHING): Design reference. Redraw with scene lighting.\n", imageIndex))
		imageIndex++
	}
	if hasProp {
		promptBuilder.WriteString(fmt.Sprintf("Image %d (PROP): Shape reference. Redraw with scene lighting.\n", imageIndex))
		imageIndex++
	}
	if hasBackground {
		promptBuilder.WriteString(fmt.Sprintf("Image %d (BACKGROUND): DO NOT COPY THIS. Create a NEW environment. Only use for mood/feeling.\n", imageIndex))
	}
	promptBuilder.WriteString("\n")

	// 출력 요구사항
	promptBuilder.WriteString("[OUTPUT - FILL ENTIRE FRAME]:\n")
	promptBuilder.WriteString("Image must touch all 4 edges. NO empty space.\n")
	promptBuilder.WriteString("NO black bars on left/right. NO letterbox on top/bottom.\n")
	promptBuilder.WriteString("Content fills 100% of the canvas.\n\n")

	// 금지사항 (간결하게)
	promptBuilder.WriteString("[FORBIDDEN - INSTANT REJECTION]:\n")
	promptBuilder.WriteString("- ANY face modification (different eyes, nose, lips, face shape)\n")
	promptBuilder.WriteString("- ANY body distortion (stretched legs, wrong head size, unnatural proportions)\n")
	promptBuilder.WriteString("- Split screens, collages, borders, multiple panels\n")
	promptBuilder.WriteString("- Cartoon, illustration, 3D render style\n")
	promptBuilder.WriteString("- Floating objects, physics violations\n\n")

	// 16:9 비율
	if aspectRatio == "16:9" {
		promptBuilder.WriteString("[16:9 WIDE FORMAT]:\n")
		promptBuilder.WriteString("Use horizontal space for cinematic composition.\n\n")
	}

	// 사용자 프롬프트
	if userPrompt != "" {
		promptBuilder.WriteString("[STYLING]:\n")
		promptBuilder.WriteString(userPrompt)
	}

	return promptBuilder.String()
}
