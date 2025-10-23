# Camera Angle & Shot Type 조합 로직 변경 사양서

## 📋 변경 개요

**목적**: REPEAT 노드에서 선택된 여러 Camera Angle과 Shot Type의 모든 조합을 생성할 수 있도록 시스템 수정

**현재 문제점**:
- REPEAT 노드에서 Front, Side, Profile, Back (4개) + Middle, Full (2개) 선택 시
- 실제로는 1개 조합(예: Front + Middle)만 처리됨
- 8가지 조합(4 × 2)이 생성되어야 하지만, 동일한 프롬프트로 반복만 됨

**변경 후**:
- 선택된 모든 조합을 Frontend에서 명시적으로 생성
- Go Server가 각 조합마다 다른 프롬프트로 이미지 생성

---

## 🔄 변경 시점 (Timeline)

### Phase 1: Frontend 수정 (Next.js)
**파일**: `src/app/visual/page.tsx`
**시점**: Go Server 배포 전에 먼저 작업 가능 (하위 호환성 유지)

### Phase 2: Go Server 수정
**파일**:
- `modules/generate-image/model.go`
- `modules/generate-image/worker.go`

**시점**: Frontend 수정 완료 후 배포

### Phase 3: 통합 테스트
**시점**: 양쪽 모두 배포 완료 후

---

## 📊 데이터 구조 변경

### 변경 전 (Current)

#### Frontend → Go Server
```javascript
jobInputData: {
  prompt: "Front view, middle shot. 여성 모델이...",  // 이미 조합된 프롬프트
  mergedImageAttachId: 123,
  individualImageAttachIds: [1, 2, 3],
  cameraAngle: "front",      // 단일 값
  shotType: "middle",        // 단일 값
  quantity: 5,               // 5개만 생성
  userId: "user123"
}
```

#### Go Server (model.go)
```go
type JobInputData struct {
    Prompt                   string   `json:"prompt"`
    MergedImageAttachID      int      `json:"mergedImageAttachId"`
    IndividualImageAttachIDs []int    `json:"individualImageAttachIds"`
    CameraAngle              string   `json:"cameraAngle"`
    ShotType                 string   `json:"shotType"`
    Quantity                 int      `json:"quantity"`
    UserID                   string   `json:"userId"`
}
```

---

### 변경 후 (New)

#### Frontend → Go Server
```javascript
jobInputData: {
  basePrompt: "여성 모델이...",  // angle/shot 제외된 순수 프롬프트
  mergedImageAttachId: 123,
  individualImageAttachIds: [1, 2, 3],
  combinations: [
    { angle: "front", shot: "middle", quantity: 5 },
    { angle: "front", shot: "full", quantity: 5 },
    { angle: "side", shot: "middle", quantity: 5 },
    { angle: "side", shot: "full", quantity: 5 },
    { angle: "profile", shot: "middle", quantity: 5 },
    { angle: "profile", shot: "full", quantity: 5 },
    { angle: "back", shot: "middle", quantity: 5 },
    { angle: "back", shot: "full", quantity: 5 }
  ],  // 8개 조합 × 5개 = 총 40개 이미지
  userId: "user123"
}
```

#### Go Server (model.go)
```go
type JobInputData struct {
    BasePrompt               string        `json:"basePrompt"`  // 변경
    MergedImageAttachID      int           `json:"mergedImageAttachId"`
    IndividualImageAttachIDs []int         `json:"individualImageAttachIds"`
    Combinations             []Combination `json:"combinations"` // 추가
    UserID                   string        `json:"userId"`
}

type Combination struct {
    Angle    string `json:"angle"`    // "front", "side", "profile", "back"
    Shot     string `json:"shot"`     // "tight", "middle", "full"
    Quantity int    `json:"quantity"` // 해당 조합 생성 개수
}
```

---

## 🔨 구체적인 수정 내용

### 1. Frontend 수정 (src/app/visual/page.tsx)

#### 위치: Line 3070-3090 근처 (Job 생성 부분)

**변경 전**:
```javascript
const jobId = await createAndEnqueueJob(productionId || "", {
  stageName,
  totalImages: finalSettings.quantity,
  jobInputData: {
    prompt: enhancedPrompt,  // "Front view, middle shot. 여성 모델이..."
    mergedImageAttachId: groupImageData.mergedImageAttachId,
    individualImageAttachIds: groupImageData.individualImages.map(img => img.attachId),
    cameraAngle: finalSettings.cameraAngle,
    shotType: finalSettings.shotType,
    quantity: finalSettings.quantity,
    userId: jobUserId,
  },
});
```

**변경 후**:
```javascript
// 조합 생성
const selectedAngles = Array.isArray(repeatNode.data?.selectedAngles)
  ? repeatNode.data.selectedAngles
  : [finalSettings.cameraAngle];

const selectedShots = Array.isArray(repeatNode.data?.selectedShots)
  ? repeatNode.data.selectedShots
  : [finalSettings.shotType];

const combinations = [];
for (const angle of selectedAngles) {
  for (const shot of selectedShots) {
    combinations.push({
      angle: angle,
      shot: shot,
      quantity: finalSettings.quantity
    });
  }
}

const totalImages = combinations.length * finalSettings.quantity;

console.log(`✅ 생성된 조합: ${combinations.length}개`);
console.log(`✅ 총 이미지 수: ${totalImages}개`);
console.log(`✅ 조합 상세:`, combinations);

const jobId = await createAndEnqueueJob(productionId || "", {
  stageName,
  totalImages: totalImages,  // 변경: 조합 수 × quantity
  jobInputData: {
    basePrompt: promptTextData.text,  // 순수 프롬프트만 (angle/shot 제외)
    mergedImageAttachId: groupImageData.mergedImageAttachId,
    individualImageAttachIds: groupImageData.individualImages.map(img => img.attachId),
    combinations: combinations,  // 조합 배열
    userId: jobUserId,
  },
});
```

---

### 2. Go Server 수정

#### 2-1. model.go

**파일 위치**: `modules/generate-image/model.go`

**변경 전**:
```go
type JobInputData struct {
    Prompt                   string   `json:"prompt"`
    MergedImageAttachID      int      `json:"mergedImageAttachId"`
    IndividualImageAttachIDs []int    `json:"individualImageAttachIds"`
    CameraAngle              string   `json:"cameraAngle"`
    ShotType                 string   `json:"shotType"`
    Quantity                 int      `json:"quantity"`
    UserID                   string   `json:"userId"`
}
```

**변경 후**:
```go
type JobInputData struct {
    BasePrompt               string        `json:"basePrompt"`  // 변경
    MergedImageAttachID      int           `json:"mergedImageAttachId"`
    IndividualImageAttachIDs []int         `json:"individualImageAttachIds"`
    Combinations             []Combination `json:"combinations"` // 추가
    UserID                   string        `json:"userId"`

    // 하위 호환성을 위해 유지 (deprecated)
    Prompt       string `json:"prompt"`
    CameraAngle  string `json:"cameraAngle"`
    ShotType     string `json:"shotType"`
    Quantity     int    `json:"quantity"`
}

type Combination struct {
    Angle    string `json:"angle"`
    Shot     string `json:"shot"`
    Quantity int    `json:"quantity"`
}
```

---

#### 2-2. worker.go - processSingleBatch 함수

**파일 위치**: `modules/generate-image/worker.go`
**함수**: `processSingleBatch` (Line 90-200 근처)

**변경 전**:
```go
func processSingleBatch(ctx context.Context, service *Service, job *ProductionJob) {
    // Phase 1: Input Data 추출
    mergedImageAttachID, ok := job.JobInputData["mergedImageAttachId"].(float64)
    if !ok {
        log.Printf("❌ Failed to get mergedImageAttachId")
        service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
        return
    }

    prompt, ok := job.JobInputData["prompt"].(string)
    if !ok {
        log.Printf("❌ Failed to get prompt")
        service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
        return
    }

    quantity := job.TotalImages
    userID, _ := job.JobInputData["userId"].(string)

    // ... (이미지 다운로드)

    // Phase 4: 이미지 생성 루프
    for i := 0; i < quantity; i++ {
        log.Printf("🎨 Generating image %d/%d...", i+1, quantity)

        // Gemini API 호출
        generatedBase64, err := service.GenerateImageWithGemini(ctx, base64Image, prompt)

        // ... (업로드 및 저장)
    }
}
```

**변경 후**:
```go
func processSingleBatch(ctx context.Context, service *Service, job *ProductionJob) {
    log.Printf("🚀 Starting Single Batch processing for job: %s", job.JobID)

    // Phase 1: Input Data 추출
    mergedImageAttachID, ok := job.JobInputData["mergedImageAttachId"].(float64)
    if !ok {
        log.Printf("❌ Failed to get mergedImageAttachId")
        service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
        return
    }

    basePrompt, ok := job.JobInputData["basePrompt"].(string)
    if !ok {
        log.Printf("❌ Failed to get basePrompt")
        service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
        return
    }

    // Combinations 배열 추출
    combinationsRaw, ok := job.JobInputData["combinations"].([]interface{})
    if !ok {
        log.Printf("❌ Failed to get combinations array")
        service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
        return
    }

    userID, _ := job.JobInputData["userId"].(string)

    log.Printf("📦 Input Data: AttachID=%d, BasePrompt=%s, Combinations=%d, UserID=%s",
        int(mergedImageAttachID), basePrompt, len(combinationsRaw), userID)

    // Phase 2: Status 업데이트
    if err := service.UpdateJobStatus(ctx, job.JobID, StatusProcessing); err != nil {
        log.Printf("❌ Failed to update job status: %v", err)
        return
    }

    if job.ProductionID != nil {
        if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, StatusProcessing); err != nil {
            log.Printf("⚠️  Failed to update production status: %v", err)
        }
    }

    // Phase 3: 입력 이미지 다운로드 및 Base64 변환
    imageData, err := service.DownloadImageFromStorage(int(mergedImageAttachID))
    if err != nil {
        log.Printf("❌ Failed to download image: %v", err)
        service.UpdateJobStatus(ctx, job.JobID, StatusFailed)
        return
    }

    base64Image := service.ConvertImageToBase64(imageData)
    log.Printf("✅ Input image prepared (Base64 length: %d)", len(base64Image))

    // Phase 4: Combinations 순회하며 이미지 생성
    generatedAttachIds := []int{}
    completedCount := 0

    // Camera Angle 매핑
    cameraAngleTextMap := map[string]string{
        "front":   "Front view",
        "side":    "Side view",
        "profile": "Profile view",
        "back":    "Back view",
    }

    // Shot Type 매핑
    shotTypeTextMap := map[string]string{
        "tight":  "tight shot, close-up",
        "middle": "middle shot, medium distance",
        "full":   "full body shot, full length",
    }

    for comboIdx, comboRaw := range combinationsRaw {
        combo := comboRaw.(map[string]interface{})
        angle := combo["angle"].(string)
        shot := combo["shot"].(string)
        quantity := int(combo["quantity"].(float64))

        log.Printf("🎯 Combination %d/%d: angle=%s, shot=%s, quantity=%d",
            comboIdx+1, len(combinationsRaw), angle, shot, quantity)

        // 조합별 프롬프트 생성
        cameraAngleText := cameraAngleTextMap[angle]
        if cameraAngleText == "" {
            cameraAngleText = "Front view" // 기본값
        }

        shotTypeText := shotTypeTextMap[shot]
        if shotTypeText == "" {
            shotTypeText = "full body shot" // 기본값
        }

        enhancedPrompt := cameraAngleText + ", " + shotTypeText + ". " + basePrompt +
            ". IMPORTANT: No split layouts, no grid layouts, no separate product shots. " +
            "Each image must be a single unified composition with the model wearing/using all items."

        log.Printf("📝 Enhanced Prompt: %s", enhancedPrompt[:min(100, len(enhancedPrompt))])

        // 해당 조합의 quantity만큼 생성
        for i := 0; i < quantity; i++ {
            log.Printf("🎨 Generating image %d/%d for combination [%s + %s]...",
                i+1, quantity, angle, shot)

            // Gemini API 호출
            generatedBase64, err := service.GenerateImageWithGemini(ctx, base64Image, enhancedPrompt)
            if err != nil {
                log.Printf("❌ Gemini API failed for image %d: %v", i+1, err)
                continue
            }

            // Base64 → []byte 변환
            generatedImageData, err := base64DecodeString(generatedBase64)
            if err != nil {
                log.Printf("❌ Failed to decode generated image %d: %v", i+1, err)
                continue
            }

            // Storage 업로드
            filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
            if err != nil {
                log.Printf("❌ Failed to upload image %d: %v", i+1, err)
                continue
            }

            // Attach 레코드 생성
            attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
            if err != nil {
                log.Printf("❌ Failed to create attach record %d: %v", i+1, err)
                continue
            }

            // 크레딧 차감
            if job.ProductionID != nil && userID != "" {
                go func(attachID int, prodID string) {
                    if err := service.DeductCredits(context.Background(), userID, prodID, []int{attachID}); err != nil {
                        log.Printf("⚠️  Failed to deduct credits for attach %d: %v", attachID, err)
                    }
                }(attachID, *job.ProductionID)
            }

            // 성공 카운트 및 ID 수집
            generatedAttachIds = append(generatedAttachIds, attachID)
            completedCount++

            log.Printf("✅ Image %d/%d completed for [%s + %s]: AttachID=%d",
                i+1, quantity, angle, shot, attachID)

            // 진행 상황 업데이트
            if err := service.UpdateJobProgress(ctx, job.JobID, completedCount, generatedAttachIds); err != nil {
                log.Printf("⚠️  Failed to update progress: %v", err)
            }
        }

        log.Printf("✅ Combination %d/%d completed: %d images generated",
            comboIdx+1, len(combinationsRaw), quantity)
    }

    // Phase 5: 최종 완료 처리
    finalStatus := StatusCompleted
    if completedCount == 0 {
        finalStatus = StatusFailed
    }

    log.Printf("🏁 Job %s finished: %d/%d images completed", job.JobID, completedCount, job.TotalImages)

    // Job 상태 업데이트
    if err := service.UpdateJobStatus(ctx, job.JobID, finalStatus); err != nil {
        log.Printf("❌ Failed to update final job status: %v", err)
    }

    // Production 업데이트
    if job.ProductionID != nil {
        if err := service.UpdateProductionPhotoStatus(ctx, *job.ProductionID, finalStatus); err != nil {
            log.Printf("⚠️  Failed to update final production status: %v", err)
        }

        if len(generatedAttachIds) > 0 {
            if err := service.UpdateProductionAttachIds(ctx, *job.ProductionID, generatedAttachIds); err != nil {
                log.Printf("⚠️  Failed to update production attach_ids: %v", err)
            }
        }
    }

    log.Printf("✅ Single Batch processing completed for job: %s", job.JobID)
}

// Helper function
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

---

## 🧪 테스트 시나리오

### 테스트 케이스 1: 단일 조합
**입력**:
- Front (1개) + Middle (1개)
- Quantity: 5

**예상 결과**:
- 1개 조합 × 5개 = 5개 이미지 생성
- 프롬프트: "Front view, middle shot. ..."

---

### 테스트 케이스 2: 다중 조합 (2×2)
**입력**:
- Front, Side (2개) + Middle, Full (2개)
- Quantity: 5

**예상 결과**:
- 4개 조합 × 5개 = 20개 이미지 생성
- 프롬프트:
  1. "Front view, middle shot. ..."
  2. "Front view, full body shot. ..."
  3. "Side view, middle shot. ..."
  4. "Side view, full body shot. ..."

---

### 테스트 케이스 3: 전체 조합 (4×2)
**입력**:
- Front, Side, Profile, Back (4개) + Middle, Full (2개)
- Quantity: 5

**예상 결과**:
- 8개 조합 × 5개 = 40개 이미지 생성
- 각 조합마다 다른 프롬프트

---

## ⚠️ 주의사항

### 1. 하위 호환성
- 기존 단일 조합 방식도 계속 지원
- `model.go`에서 deprecated 필드 유지
- 새로운 `combinations` 필드가 없으면 기존 방식으로 fallback

### 2. 배포 순서
1. **먼저**: Frontend 배포 (새 데이터 구조로 전송)
2. **나중**: Go Server 배포 (새 데이터 구조 처리)
3. 중간에 잠깐 동안 호환성 문제 발생 가능 → **동시 배포 권장**

### 3. 데이터베이스
- `quel_production_jobs` 테이블의 `job_input_data`는 JSONB 타입
- 스키마 변경 불필요
- 자유롭게 구조 변경 가능

### 4. 로그 확인
- Go Server 로그에서 조합 처리 과정 확인:
  ```
  🎯 Combination 1/8: angle=front, shot=middle, quantity=5
  📝 Enhanced Prompt: Front view, middle shot. 여성 모델이...
  🎨 Generating image 1/5 for combination [front + middle]...
  ✅ Image 1/5 completed for [front + middle]: AttachID=456
  ```

---

## 📝 Go Server Claude에게 전달할 내용

이 문서를 Go Server 담당자에게 전달하면서 다음 사항을 강조:

1. **변경 이유**: 다중 Camera Angle & Shot Type 조합 지원
2. **변경 파일**: `model.go`, `worker.go` (2개 파일만)
3. **핵심 변경**:
   - `combinations` 배열 추가
   - 각 조합마다 다른 프롬프트 생성
   - 조합별 이미지 생성 루프
4. **테스트**: 위 테스트 케이스로 검증 필요
5. **배포**: Frontend와 동시 배포 권장

---

## 📌 체크리스트

### Frontend (Next.js)
- [ ] `selectedAngles` 배열 추출
- [ ] `selectedShots` 배열 추출
- [ ] `combinations` 배열 생성 (이중 for 루프)
- [ ] `totalImages` 계산 (조합 수 × quantity)
- [ ] `jobInputData` 구조 변경
- [ ] 로그 추가 (조합 개수, 총 이미지 수)

### Go Server
- [ ] `model.go`: `Combination` 구조체 추가
- [ ] `model.go`: `JobInputData`에 `Combinations` 필드 추가
- [ ] `worker.go`: `basePrompt` 추출
- [ ] `worker.go`: `combinations` 배열 파싱
- [ ] `worker.go`: 조합별 프롬프트 생성 로직
- [ ] `worker.go`: 조합별 이미지 생성 루프
- [ ] `worker.go`: 로그 개선 (조합 정보 출력)

### 테스트
- [ ] 단일 조합 (1×1) 테스트
- [ ] 다중 조합 (2×2) 테스트
- [ ] 전체 조합 (4×2) 테스트
- [ ] 로그 확인
- [ ] 생성된 이미지 확인 (프롬프트 반영 여부)

---

**작성일**: 2025-10-23
**작성자**: Claude (Frontend)
**대상**: Go Server 개발자
