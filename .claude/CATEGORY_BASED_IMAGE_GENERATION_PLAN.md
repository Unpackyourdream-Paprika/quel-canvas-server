# 카테고리별 이미지 분리 전송 구조 변경 계획

## 📋 개요

Gemini API의 이미지 참조 최대 4장 제한을 활용하여, 이미지를 카테고리별로 분류하고 병합하여 전송합니다.
이를 통해 Gemini가 각 요소(모델, 의류, 악세사리, 배경)를 명확히 인식하도록 개선합니다.

---

## 🎯 목표

1. **명확한 구분**: 모델, 의류, 악세사리, 배경을 각각 별도 Part로 전송
2. **4장 제한 준수**: Gemini 최대 4장 제한 내에서 모든 정보 전달
3. **병합 자동화**: 백엔드에서 카테고리별 이미지 자동 병합
4. **동적 프롬프트**: 상황별로 적절한 프롬프트 자동 생성

---

## 📦 카테고리 분류

### **1. 의류 (Clothing)** - 병합 필요
```
Type 값:
- "top"    (상의: 셔츠, 티셔츠, 니트 등)
- "pants"  (하의: 바지, 치마 등)
- "outer"  (아우터: 재킷, 코트 등)

처리: 이 3개 type의 이미지들을 1장으로 병합
```

### **2. 악세사리 (Accessories)** - 병합 필요
```
Type 값:
- "shoes"     (신발)
- "bag"       (가방)
- "accessory" (기타 악세사리: 모자, 목걸이, 시계 등)

처리: 이 3개 type의 이미지들을 1장으로 병합
```

### **3. 모델 (Model)** - 단독 사용
```
Type 값:
- "model"  (얼굴 또는 전신 사진)

처리: 그대로 1장 사용 (병합 없음)
```

### **4. 배경 (Background)** - 단독 사용
```
Type 값:
- "bg"  (배경 이미지)

처리: 그대로 1장 사용 (병합 없음)
```

---

## 🔄 프론트엔드 변경 사항

### **기존 구조**
```json
{
  "job_input_data": {
    "uploadedAttachIds": [
      {"attachId": 100},
      {"attachId": 101},
      {"attachId": 102}
    ],
    "prompt": "...",
    "aspect-ratio": "16:9"
  }
}
```

### **변경 후 구조**
```json
{
  "job_input_data": {
    "uploadedAttachIds": [
      {"attachId": 100, "type": "model"},
      {"attachId": 101, "type": "top"},
      {"attachId": 102, "type": "pants"},
      {"attachId": 103, "type": "shoes"},
      {"attachId": 104, "type": "bag"},
      {"attachId": 105, "type": "bg"}
    ],
    "prompt": "standing pose, neutral lighting",
    "aspect-ratio": "16:9"
  }
}
```

### **프론트엔드 TODO**
1. ✅ `uploadedAttachIds` 배열의 각 객체에 **`type` 필드 추가**
2. ✅ 가능한 type 값:
   - `"model"` - 모델 사진
   - `"top"` - 상의
   - `"pants"` - 하의
   - `"outer"` - 아우터
   - `"shoes"` - 신발
   - `"bag"` - 가방
   - `"accessory"` - 악세사리
   - `"bg"` - 배경
3. ✅ 기존 머지 로직 **제거** (백엔드에서 처리)

---

## 🔧 백엔드 처리 흐름

### **1. 이미지 분류 및 다운로드**
```
uploadedAttachIds 순회:
  → attachId로 Storage에서 이미지 다운로드
  → type에 따라 카테고리별로 분류

결과:
- modelImages: []byte (최대 1장)
- clothingImages: [][]byte (top, pants, outer)
- accessoryImages: [][]byte (shoes, bag, accessory)
- bgImages: []byte (최대 1장)
```

### **2. 카테고리별 병합**
```
의류 병합:
  if len(clothingImages) > 1:
    mergedClothing = mergeImages(clothingImages)
  else:
    mergedClothing = clothingImages[0]

악세사리 병합:
  if len(accessoryImages) > 1:
    mergedAccessories = mergeImages(accessoryImages)
  else:
    mergedAccessories = accessoryImages[0]
```

### **3. Gemini Part 배열 구성**
```
parts := []*genai.Part{}

// 순서 중요: Model → Clothing → Accessories → Background
if modelImage != nil:
  parts.append(modelImagePart)

if mergedClothing != nil:
  parts.append(clothingImagePart)

if mergedAccessories != nil:
  parts.append(accessoriesImagePart)

if bgImage != nil:
  parts.append(bgImagePart)

// 마지막에 프롬프트
parts.append(dynamicPromptPart)
```

### **4. 동적 프롬프트 생성**
```go
상황별 프롬프트 예시:

[모델 + 의류 + 악세사리 + 배경 모두 있음]
"Image 1: Model's face and body reference
Image 2: Clothing items (tops, pants, outerwear) to wear
Image 3: Accessories (shoes, bags, jewelry) to wear/carry
Image 4: Background environment setting
Generate a professional fashion photograph of the model wearing
all the clothing and accessories in the specified background."

[모델 없음 - 의류 + 악세사리만]
"Image 1: Clothing items to wear
Image 2: Accessories to wear/carry
Generate a professional fashion photograph of a model wearing
all these items."

[의류만 있음]
"Image 1: Clothing items
Generate a professional fashion photograph of a model wearing these clothes."
```

---

## 📊 상황별 Part 구성 예시

| 보유 이미지 | Part 구성 | 총 이미지 수 |
|------------|----------|------------|
| 모델 + 의류 + 악세사리 + 배경 | Model + Clothing + Accessories + BG | 4장 |
| 모델 + 의류 + 악세사리 | Model + Clothing + Accessories | 3장 |
| 모델 + 의류 + 배경 | Model + Clothing + BG | 3장 |
| 모델 + 의류 | Model + Clothing | 2장 |
| 의류 + 악세사리 | Clothing + Accessories | 2장 |
| 의류만 | Clothing | 1장 |

---

## 🔨 백엔드 구현 상세

### **함수 1: 이미지 분류**
```go
func classifyImages(uploadedAttachIds []AttachObject) (*ImageCategories, error) {
  categories := &ImageCategories{
    Clothing:    [][]byte{},
    Accessories: [][]byte{},
  }

  clothingTypes := map[string]bool{"top": true, "pants": true, "outer": true}
  accessoryTypes := map[string]bool{"shoes": true, "bag": true, "accessory": true}

  for _, attach := range uploadedAttachIds {
    imageData := downloadFromStorage(attach.AttachID)

    switch attach.Type {
    case "model":
      categories.Model = imageData
    case "bg":
      categories.Background = imageData
    default:
      if clothingTypes[attach.Type] {
        categories.Clothing = append(categories.Clothing, imageData)
      } else if accessoryTypes[attach.Type] {
        categories.Accessories = append(categories.Accessories, imageData)
      }
    }
  }

  return categories, nil
}
```

### **함수 2: 이미지 병합**
```go
func mergeImages(images [][]byte) ([]byte, error) {
  // PNG 디코드
  decodedImages := []image.Image{}
  for _, imgData := range images {
    img, _ := png.Decode(bytes.NewReader(imgData))
    decodedImages = append(decodedImages, img)
  }

  // 가로로 나란히 배치하여 병합
  totalWidth := 0
  maxHeight := 0
  for _, img := range decodedImages {
    bounds := img.Bounds()
    totalWidth += bounds.Dx()
    if bounds.Dy() > maxHeight {
      maxHeight = bounds.Dy()
    }
  }

  // 새 이미지 생성
  merged := image.NewRGBA(image.Rect(0, 0, totalWidth, maxHeight))

  // 이미지 배치
  xOffset := 0
  for _, img := range decodedImages {
    draw.Draw(merged, image.Rect(xOffset, 0, xOffset+img.Bounds().Dx(), maxHeight),
              img, image.Point{0, 0}, draw.Src)
    xOffset += img.Bounds().Dx()
  }

  // PNG 인코딩
  var buf bytes.Buffer
  png.Encode(&buf, merged)
  return buf.Bytes(), nil
}
```

### **함수 3: 동적 프롬프트 생성**
```go
func generateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
  var instructions []string
  imageIndex := 1

  if categories.Model != nil {
    instructions = append(instructions,
      fmt.Sprintf("Image %d: Model's face and body reference", imageIndex))
    imageIndex++
  }

  if len(categories.Clothing) > 0 {
    instructions = append(instructions,
      fmt.Sprintf("Image %d: Clothing items (tops, pants, outerwear) to wear", imageIndex))
    imageIndex++
  }

  if len(categories.Accessories) > 0 {
    instructions = append(instructions,
      fmt.Sprintf("Image %d: Accessories (shoes, bags, jewelry) to wear/carry", imageIndex))
    imageIndex++
  }

  if categories.Background != nil {
    instructions = append(instructions,
      fmt.Sprintf("Image %d: Background environment setting", imageIndex))
    imageIndex++
  }

  // 기본 지시사항
  baseInstruction := "Generate a professional fashion photograph"

  if categories.Model != nil {
    baseInstruction += " of the model wearing all the clothing"
    if len(categories.Accessories) > 0 {
      baseInstruction += " and accessories"
    }
  } else {
    baseInstruction += " of a model wearing the clothing"
    if len(categories.Accessories) > 0 {
      baseInstruction += " and accessories"
    }
  }

  if categories.Background != nil {
    baseInstruction += " in the specified background environment."
  } else {
    baseInstruction += " in a clean studio setting."
  }

  // 최종 조합
  finalPrompt := strings.Join(instructions, "\n") + "\n\n" + baseInstruction

  if userPrompt != "" {
    finalPrompt += "\n\nAdditional styling: " + userPrompt
  }

  return finalPrompt
}
```

---

## 🎬 전체 흐름 요약

```
1. 프론트 → 백엔드
   uploadedAttachIds (with type field)

2. 백엔드 처리
   ├─ classifyImages()
   │  └─ type별로 이미지 분류 및 다운로드
   │
   ├─ mergeImages()
   │  ├─ Clothing 이미지들 병합
   │  └─ Accessories 이미지들 병합
   │
   ├─ buildGeminiParts()
   │  └─ Model → Clothing → Accessories → BG 순서로 Part 구성
   │
   ├─ generateDynamicPrompt()
   │  └─ 보유한 이미지 상황에 맞는 프롬프트 생성
   │
   └─ GenerateContent()
      └─ Gemini API 호출

3. Gemini → 백엔드
   생성된 이미지 반환

4. 백엔드 → 프론트
   결과 이미지 전달
```

---

## ✅ 체크리스트

### 프론트엔드
- [ ] `uploadedAttachIds` 각 객체에 `type` 필드 추가
- [ ] 이미지 업로드 시 올바른 type 값 설정
- [ ] 기존 이미지 머지 로직 제거

### 백엔드
- [ ] `classifyImages()` 함수 구현
- [ ] `mergeImages()` 함수 구현
- [ ] `generateDynamicPrompt()` 함수 구현
- [ ] `GenerateImageWithGeminiMultiple()` 함수 수정
- [ ] Pipeline worker 로직 수정
- [ ] 테스트 및 검증

---

## 🔍 예상 효과

1. ✅ **상품 분리 문제 해결**: 의류/악세사리를 별도 Part로 전송하여 Gemini가 명확히 인식
2. ✅ **여백 문제 개선**: 각 요소를 명확히 구분하여 프레이밍 품질 향상 기대
3. ✅ **유연성 증가**: 모델, 배경 있음/없음 상황 모두 대응 가능
4. ✅ **확장성**: 향후 새로운 카테고리 추가 용이

---

## 📝 참고사항

- Gemini API 이미지 참조 최대: **4장**
- 현재 사용 모델: `gemini-2.5-flash-image`
- Temperature: 0.7
- 이미지 병합 시 가로 배치 (side-by-side)
- 프롬프트 최대 길이 제한 없음 (하지만 간결할수록 좋음)
