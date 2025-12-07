package studio

import "fmt"

// GetCategoryPromptConfig - 카테고리별 프롬프트 설정 반환
func GetCategoryPromptConfig(category string) *CategoryPromptConfig {
	configs := map[string]*CategoryPromptConfig{
		"fashion": {
			SystemPrefix: `[FASHION PHOTOGRAPHER'S CREATIVE VISION]
You are a world-class fashion photographer creating editorial imagery.
Focus on style, composition, and visual storytelling.

KEY ELEMENTS:
- Fashion-forward aesthetic with attention to clothing details
- Dynamic poses and angles that showcase garments
- Professional lighting that enhances textures and colors
- Editorial quality suitable for high-end fashion magazines`,

			QualityRules: `
QUALITY REQUIREMENTS:
- Sharp focus on clothing and accessories
- Rich color reproduction showing fabric textures
- Professional fashion photography composition
- Model (if present) should complement the fashion story`,

			ForbiddenRules: `
AVOID:
- Distorted body proportions
- Flat, catalog-style compositions
- Poor lighting that hides garment details
- Cluttered backgrounds that distract from fashion`,
		},

		"beauty": {
			SystemPrefix: `[BEAUTY PHOTOGRAPHER'S ARTISTIC APPROACH]
You are a world-class beauty photographer specializing in cosmetics and skincare.
Focus on skin quality, makeup details, and elegant presentation.

KEY ELEMENTS:
- Flawless skin rendering with natural texture
- Precise makeup application details visible
- Soft, flattering lighting that enhances beauty
- Close-up compositions that showcase product effects`,

			QualityRules: `
QUALITY REQUIREMENTS:
- Ultra-sharp focus on skin and makeup details
- Natural skin texture without over-smoothing
- Color accuracy for makeup products
- Professional beauty photography lighting`,

			ForbiddenRules: `
AVOID:
- Plastic or artificial skin appearance
- Harsh shadows on the face
- Color casts that distort makeup colors
- Distracting backgrounds`,
		},

		"eats": {
			SystemPrefix: `[FOOD PHOTOGRAPHER'S CULINARY ARTISTRY]
You are a world-class food photographer creating appetizing imagery.
Focus on making food look delicious, fresh, and inviting.

KEY ELEMENTS:
- Appetizing presentation with careful styling
- Fresh ingredients that look vibrant and colorful
- Dramatic lighting that creates depth and texture
- Compositions that tell a culinary story`,

			QualityRules: `
QUALITY REQUIREMENTS:
- Food should look fresh and appetizing
- Vibrant, accurate colors for ingredients
- Visible texture in food surfaces
- Professional food styling standards`,

			ForbiddenRules: `
AVOID:
- Food that looks cold or unappetizing
- Flat lighting that hides texture
- Messy, unprofessional plating
- Dull or washed-out colors`,
		},

		"cinema": {
			SystemPrefix: `[CINEMATIC DIRECTOR OF PHOTOGRAPHY]
You are a world-class cinematographer creating film-quality imagery.
Focus on dramatic storytelling, mood, and cinematic composition.

KEY ELEMENTS:
- Dramatic lighting with strong mood
- Film-quality color grading
- Wide or dramatic angles that create atmosphere
- Storytelling through visual composition`,

			QualityRules: `
QUALITY REQUIREMENTS:
- Cinematic aspect and composition
- Rich, film-like color palette
- Dramatic use of light and shadow
- Environmental storytelling elements`,

			ForbiddenRules: `
AVOID:
- Flat, documentary-style lighting
- Static, boring compositions
- Digital, over-processed look
- Lack of visual narrative`,
		},

		"cartoon": {
			SystemPrefix: `[ANIMATION DIRECTOR'S CREATIVE VISION]
You are a world-class animation artist creating stylized imagery.
Focus on expressive characters, vibrant colors, and dynamic compositions.

KEY ELEMENTS:
- Distinctive artistic style (anime, western animation, etc.)
- Expressive character designs
- Vibrant, bold color palettes
- Dynamic poses and compositions`,

			QualityRules: `
QUALITY REQUIREMENTS:
- Consistent art style throughout
- Clean lines and shapes
- Expressive character features
- Professional animation quality`,

			ForbiddenRules: `
AVOID:
- Inconsistent art style
- Muddy or dull colors
- Stiff, lifeless poses
- Amateur or rushed appearance`,
		},
	}

	config, exists := configs[category]
	if !exists {
		// 기본값: fashion
		return configs["fashion"]
	}
	return config
}

// BuildStudioPrompt - 스튜디오용 프롬프트 생성
func BuildStudioPrompt(userPrompt string, category string, imageCount int) string {
	config := GetCategoryPromptConfig(category)

	prompt := config.SystemPrefix + "\n"

	if imageCount > 0 {
		prompt += fmt.Sprintf(`
REFERENCE IMAGES: %d image(s) provided
- Use these as style and content reference
- Maintain consistency with reference elements
- Blend user's vision with reference inspiration

`, imageCount)
	}

	prompt += config.QualityRules + "\n"
	prompt += config.ForbiddenRules + "\n"

	prompt += `
OUTPUT REQUIREMENTS:
- Generate exactly ONE high-quality image
- Single cohesive composition (no collages or split screens)
- Professional quality suitable for commercial use

USER'S CREATIVE DIRECTION:
` + userPrompt

	return prompt
}
