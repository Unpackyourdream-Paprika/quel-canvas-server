package landingdemo

import (
	"fmt"
	"strings"
)

// BuildDynamicPrompt - 카테고리별 동적 프롬프트 생성 (fashion 모듈과 동일한 방식)
func BuildDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
	hasModel := categories.Model != nil
	hasClothing := len(categories.Clothing) > 0
	hasAccessories := len(categories.Accessories) > 0
	hasProducts := hasClothing || hasAccessories
	hasBackground := categories.Background != nil

	var mainInstruction string
	if hasModel {
		mainInstruction = "[FASHION PHOTOGRAPHER'S DRAMATIC COMPOSITION]\n" +
			"You are a world-class fashion photographer shooting an editorial campaign.\n" +
			"The PERSON is the HERO - their natural proportions are SACRED and CANNOT be distorted.\n" +
			"The environment serves the subject, NOT the other way around.\n\n" +
			"⚠️ CRITICAL MODEL REQUIREMENTS:\n" +
			"• The MODEL REFERENCE IMAGE shows the EXACT person to use\n" +
			"• Copy their FACE, BODY SHAPE, SKIN TONE, HAIR precisely\n" +
			"• This specific person must be recognizable in the output\n\n" +
			"Create ONE photorealistic photograph with DRAMATIC CINEMATIC STORYTELLING:\n" +
			"• The model wears ALL clothing and accessories in ONE complete outfit\n" +
			"• Dynamic pose and angle - NOT static or stiff\n" +
			"• Environmental storytelling - use the location for drama\n" +
			"• Directional lighting creates mood and depth\n" +
			"• This is a MOMENT full of energy and narrative\n\n"
	} else if hasProducts {
		mainInstruction = "[CINEMATIC PRODUCT PHOTOGRAPHER'S APPROACH]\n" +
			"You are a world-class product photographer creating editorial-style still life.\n" +
			"⚠️ CRITICAL: NO people or models in this shot - products only.\n\n" +
			"Create ONE photorealistic photograph with ARTISTIC STORYTELLING:\n" +
			"• Artistic arrangement of all items - creative composition\n" +
			"• Dramatic lighting that highlights textures and materials\n\n"
	} else {
		mainInstruction = "[CINEMATIC ENVIRONMENTAL PHOTOGRAPHER'S APPROACH]\n" +
			"Create ONE photorealistic photograph with ATMOSPHERIC STORYTELLING.\n\n"
	}

	var instructions []string
	imageIndex := 1

	if categories.Model != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (MODEL): ⚠️ CRITICAL - This person's face, body shape, skin tone, and physical features - use EXACTLY this appearance. The generated person MUST look like THIS specific individual.", imageIndex))
		imageIndex++
	}

	if len(categories.Clothing) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (CLOTHING): ALL visible garments - tops, bottoms, dresses, outerwear. The person MUST wear EVERY piece shown here", imageIndex))
		imageIndex++
	}

	if len(categories.Accessories) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (ACCESSORIES): ALL items - shoes, bags, jewelry. Use ONLY items visible in reference", imageIndex))
		imageIndex++
	}

	if categories.Background != nil {
		instructions = append(instructions,
			fmt.Sprintf("Reference Image %d (LOCATION INSPIRATION): This shows the MOOD and ATMOSPHERE you should recreate - NOT a background to paste. Generate a NEW environment inspired by this reference.", imageIndex))
	}

	var compositionInstruction string
	if hasModel {
		compositionInstruction = "\n[FASHION EDITORIAL COMPOSITION]\n" +
			"Generate ONE photorealistic film photograph showing the referenced model wearing the complete outfit.\n" +
			"This is a high-end fashion editorial shoot with the model as the star."
		if hasBackground {
			compositionInstruction += " Shot on location with environmental storytelling."
		}
	} else if hasProducts {
		compositionInstruction = "\n[PRODUCT PHOTOGRAPHY]\n" +
			"Generate ONE photorealistic product photograph. ⚠️ NO people or models."
	}

	criticalRules := "\n\n[NON-NEGOTIABLE REQUIREMENTS]\n" +
		"🎯 Person's body proportions are PERFECT and NATURAL - ZERO tolerance for distortion\n" +
		"🎯 The subject is the STAR - everything else supports their presence\n" +
		"🎯 ALL clothing and accessories worn/carried simultaneously\n" +
		"🎯 Single cohesive photograph - ONE shot from ONE camera\n" +
		"🎯 Film photography aesthetic - not digital, not flat\n\n" +
		"[FORBIDDEN]\n" +
		"❌ ANY distortion of the person's proportions\n" +
		"❌ Person looking pasted, floating, or artificially placed\n" +
		"❌ Split-screen, collage, or multiple separate images\n" +
		"❌ Flat lighting that doesn't create mood"

	// 16:9 비율 전용 추가 지시사항
	var aspectRatioInstruction string
	if aspectRatio == "16:9" && hasModel {
		aspectRatioInstruction = "\n\n[16:9 CINEMATIC WIDE SHOT - DRAMATIC STORYTELLING]\n" +
			"This is a WIDE ANGLE shot - use the horizontal space for powerful visual storytelling.\n\n" +
			"⚠️ CRITICAL FRAME REQUIREMENTS:\n" +
			"• The image MUST fill the ENTIRE 16:9 frame edge-to-edge\n" +
			"• NO black bars, NO letterboxing, NO empty margins on any side\n" +
			"• The scene content must extend to ALL four edges of the frame\n" +
			"• Generate a TRUE 16:9 widescreen image, not a cropped or padded image\n\n" +
			"🎬 DRAMATIC WIDE COMPOSITION:\n" +
			"✓ Subject positioned off-center (rule of thirds) creating dynamic tension\n" +
			"✓ Use the WIDTH to show environmental context and atmosphere\n" +
			"✓ Layers of depth - foreground elements, subject, background scenery\n" +
			"✓ Leading lines guide the eye to the subject\n" +
			"✓ Environment extends naturally to fill the wide frame\n\n" +
			"🎬 SUBJECT INTEGRITY IN WIDE FRAME:\n" +
			"⚠️ The wide frame is NOT an excuse to distort proportions\n" +
			"⚠️ Person maintains PERFECT natural proportions - just smaller in frame if needed\n" +
			"⚠️ Use the space to tell a STORY, not to force-fit the subject\n\n" +
			"🎬 CINEMATIC EXECUTION:\n" +
			"✓ Directional lighting creates mood across the wide frame\n" +
			"✓ Film grain and natural color grading\n" +
			"✓ Depth of field emphasizes the subject while showing environment\n\n" +
			"GOAL: A breathtaking wide shot from a high-budget fashion editorial - \n" +
			"like Annie Leibovitz or Steven Meisel capturing a MOMENT of drama and beauty."
	}

	finalPrompt := mainInstruction + strings.Join(instructions, "\n") + compositionInstruction + criticalRules + aspectRatioInstruction

	if userPrompt != "" {
		finalPrompt += "\n\n[ADDITIONAL STYLING]\n" + userPrompt
	}

	return finalPrompt
}
