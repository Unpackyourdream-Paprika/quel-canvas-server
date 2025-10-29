# Image Generation Retry Logic Specification

## Overview
Pipeline stage 작업에서 각 Stage별로 요청한 quantity만큼 이미지가 생성되지 않는 경우, 부족한 갯수만큼 자동으로 재시도하여 정확히 요청한 갯수를 맞추는 로직입니다.

---

## Problem Statement

### 현재 상황
- 총 150장 요청 (Stage 0: 50장, Stage 1: 50장, Stage 2: 50장)
- Gemini API 호출 중 일부 실패 가능 (`continue`로 넘어감)
- 최종 결과: 147장만 생성되는 경우 발생

### 문제점
- Stage 0: 48/50 생성 (2장 부족)
- Stage 1: 50/50 생성 (정상)
- Stage 2: 49/50 생성 (1장 부족)
- **총 3장 부족한 상태로 Complete 처리됨**

---

## Solution Design

### 핵심 아이디어
1. **요청 갯수는 이미 있음**: `stages[idx]["quantity"]`
2. **실제 생성 갯수도 추적됨**: `results[idx].AttachIDs` 배열 길이
3. **차이 계산 = 부족분**: `quantity - len(results[idx].AttachIDs)`
4. **wg.Wait() 후 체크**: 모든 Stage의 첫 번째 for문 완료 후

### 체크 시점
```go
// 모든 Stage 병렬 처리 (라인 382-496)
for stageIdx, stageData := range stages {
    wg.Add(1)
    go func(idx int, data interface{}) {
        defer wg.Done()

        // 첫 번째 for문: 요청한 quantity만큼 생성 시도
        for i := 0; i < quantity; i++ {
            // Gemini API 호출
            // 실패 시 continue (stageGeneratedIds에 추가 안 됨)
        }

        // Stage 결과 저장
        results[stageIndex] = StageResult{
            AttachIDs: stageGeneratedIds,
            Success: len(stageGeneratedIds)
        }
    }(stageIdx, stageData)
}

// ===== 여기가 체크 시점 =====
wg.Wait()  // 라인 500
log.Printf("✅ All stages completed in parallel")

// ===== 재시도 로직 추가 위치 (라인 501) =====
```

---

## Implementation Logic

### Step 1: 각 Stage별 부족 갯수 계산

```go
// wg.Wait() 직후 (라인 501)
log.Printf("🔍 Checking missing images for each stage...")

for stageIdx, stageData := range stages {
    stage := stageData.(map[string]interface{})
    expectedQuantity := int(stage["quantity"].(float64))  // 요청 갯수
    actualQuantity := len(results[stageIdx].AttachIDs)    // 실제 생성 갯수
    missing := expectedQuantity - actualQuantity          // 부족분

    if missing > 0 {
        log.Printf("⚠️  Stage %d: Missing %d images (expected: %d, got: %d)",
            stageIdx, missing, expectedQuantity, actualQuantity)
    } else {
        log.Printf("✅ Stage %d: Complete (expected: %d, got: %d)",
            stageIdx, expectedQuantity, actualQuantity)
    }
}
```

### Step 2: 부족한 Stage만 재시도

```go
for stageIdx, stageData := range stages {
    stage := stageData.(map[string]interface{})
    expectedQuantity := int(stage["quantity"].(float64))
    actualQuantity := len(results[stageIdx].AttachIDs)
    missing := expectedQuantity - actualQuantity

    if missing <= 0 {
        continue  // 부족분 없으면 스킵
    }

    log.Printf("🔄 Stage %d: Starting retry for %d missing images...", stageIdx, missing)

    // Stage 데이터 재추출
    prompt := stage["prompt"].(string)
    aspectRatio := "16:9"
    if ar, ok := stage["aspect-ratio"].(string); ok && ar != "" {
        aspectRatio = ar
    }
    mergedImageAttachID := int(stage["mergedImageAttachId"].(float64))

    // 입력 이미지 다시 다운로드
    imageData, err := service.DownloadImageFromStorage(mergedImageAttachID)
    if err != nil {
        log.Printf("❌ Stage %d: Failed to download input image for retry: %v", stageIdx, err)
        continue
    }
    base64Image := service.ConvertImageToBase64(imageData)

    // 두 번째 for문: 부족한 갯수만큼 재생성
    retrySuccess := 0
    for i := 0; i < missing; i++ {
        log.Printf("🔄 Stage %d: Retry generating image %d/%d...", stageIdx, i+1, missing)

        // Gemini API 호출 (첫 번째 for문과 동일한 로직)
        generatedBase64, err := service.GenerateImageWithGemini(ctx, base64Image, prompt, aspectRatio)
        if err != nil {
            log.Printf("❌ Stage %d: Retry %d failed: %v", stageIdx, i+1, err)
            continue
        }

        // Base64 → []byte 변환
        generatedImageData, err := base64DecodeString(generatedBase64)
        if err != nil {
            log.Printf("❌ Stage %d: Failed to decode retry image %d: %v", stageIdx, i+1, err)
            continue
        }

        // Storage 업로드
        filePath, webpSize, err := service.UploadImageToStorage(ctx, generatedImageData, userID)
        if err != nil {
            log.Printf("❌ Stage %d: Failed to upload retry image %d: %v", stageIdx, i+1, err)
            continue
        }

        // Attach 레코드 생성
        attachID, err := service.CreateAttachRecord(ctx, filePath, webpSize)
        if err != nil {
            log.Printf("❌ Stage %d: Failed to create attach record for retry %d: %v", stageIdx, i+1, err)
            continue
        }

        // 크레딧 차감
        if job.ProductionID != nil && userID != "" {
            go func(attachID int, prodID string) {
                if err := service.DeductCredits(context.Background(), userID, prodID, []int{attachID}); err != nil {
                    log.Printf("⚠️  Stage %d: Failed to deduct credits for retry attach %d: %v", stageIdx, attachID, err)
                }
            }(attachID, *job.ProductionID)
        }

        // results에 추가
        results[stageIdx].AttachIDs = append(results[stageIdx].AttachIDs, attachID)
        retrySuccess++

        // 전체 진행 상황 업데이트
        progressMutex.Lock()
        totalCompleted++
        currentProgress := totalCompleted
        tempAttachIds = append(tempAttachIds, attachID)
        progressMutex.Unlock()

        log.Printf("✅ Stage %d: Retry image %d/%d completed: AttachID=%d", stageIdx, i+1, missing, attachID)
        log.Printf("📊 Overall progress: %d/%d images completed", currentProgress, job.TotalImages)

        // DB 업데이트
        currentTempIds := make([]int, len(tempAttachIds))
        copy(currentTempIds, tempAttachIds)
        if err := service.UpdateJobProgress(ctx, job.JobID, currentProgress, currentTempIds); err != nil {
            log.Printf("⚠️  Failed to update progress: %v", err)
        }
    }

    log.Printf("✅ Stage %d retry completed: %d/%d images recovered",
        stageIdx, retrySuccess, missing)
    log.Printf("📊 Stage %d final count: %d/%d images",
        stageIdx, len(results[stageIdx].AttachIDs), expectedQuantity)
}

log.Printf("🔍 All retry attempts completed")
```

### Step 3: 최종 검증 및 병합

```go
// 기존 로직 (라인 504-523) 그대로 진행
log.Printf("🔍 ===== Stage Results Before Merge =====")
for i := 0; i < len(results); i++ {
    if results[i].AttachIDs != nil {
        log.Printf("📦 Stage %d: %v (total: %d)", i, results[i].AttachIDs, len(results[i].AttachIDs))
    } else {
        log.Printf("📦 Stage %d: [] (empty)", i)
    }
}
log.Printf("🔍 ========================================")

// Stage 순서대로 AttachID 합치기
allGeneratedAttachIds := []int{}
for i := 0; i < len(results); i++ {
    if results[i].AttachIDs != nil {
        allGeneratedAttachIds = append(allGeneratedAttachIds, results[i].AttachIDs...)
        log.Printf("📎 Stage %d: Added %d attach IDs in order", i, len(results[i].AttachIDs))
    }
}

log.Printf("🎯 Final merged array: %v (total: %d)", allGeneratedAttachIds, len(allGeneratedAttachIds))

// 최종 Complete 처리 (라인 533-549)
```

---

## Code Modification Points

### 파일: `modules/generate-image/worker.go`

**수정 위치**: 라인 501 (wg.Wait() 직후)

**기존 코드**:
```go
// 라인 498-503
log.Printf("⏳ Waiting for all stages to complete...")
wg.Wait()
log.Printf("✅ All stages completed in parallel")

// 배열 합치기 전 각 Stage 결과 출력
log.Printf("🔍 ===== Stage Results Before Merge =====")
```

**수정 후**:
```go
// 라인 498-503
log.Printf("⏳ Waiting for all stages to complete...")
wg.Wait()
log.Printf("✅ All stages completed in parallel")

// ========== 재시도 로직 추가 시작 ==========
log.Printf("🔍 Checking missing images for each stage...")

// Step 1: 부족 갯수 확인
for stageIdx, stageData := range stages {
    stage := stageData.(map[string]interface{})
    expectedQuantity := int(stage["quantity"].(float64))
    actualQuantity := len(results[stageIdx].AttachIDs)
    missing := expectedQuantity - actualQuantity

    if missing > 0 {
        log.Printf("⚠️  Stage %d: Missing %d images", stageIdx, missing)
    } else {
        log.Printf("✅ Stage %d: Complete", stageIdx)
    }
}

// Step 2: 재시도 루프
for stageIdx, stageData := range stages {
    // ... (위 Step 2 로직 전체)
}

log.Printf("🔍 All retry attempts completed")
// ========== 재시도 로직 추가 끝 ==========

// 배열 합치기 전 각 Stage 결과 출력
log.Printf("🔍 ===== Stage Results Before Merge =====")
```

---

## Expected Results

### Before (재시도 없음)
```
🎬 Stage 0 completed: 48/50 images generated
🎬 Stage 1 completed: 50/50 images generated
🎬 Stage 2 completed: 49/50 images generated
✅ All stages completed in parallel
🔍 ===== Stage Results Before Merge =====
📦 Stage 0: [9634, 9635, ...] (total: 48)
📦 Stage 1: [9684, 9685, ...] (total: 50)
📦 Stage 2: [9734, 9735, ...] (total: 49)
🎯 Final merged array: [...] (total: 147)
🏁 Pipeline Job finished: 147/150 images completed  ❌ 부족
```

### After (재시도 추가)
```
🎬 Stage 0 completed: 48/50 images generated
🎬 Stage 1 completed: 50/50 images generated
🎬 Stage 2 completed: 49/50 images generated
✅ All stages completed in parallel

🔍 Checking missing images for each stage...
⚠️  Stage 0: Missing 2 images (expected: 50, got: 48)
✅ Stage 1: Complete (expected: 50, got: 50)
⚠️  Stage 2: Missing 1 images (expected: 50, got: 49)

🔄 Stage 0: Starting retry for 2 missing images...
🔄 Stage 0: Retry generating image 1/2...
✅ Stage 0: Retry image 1/2 completed: AttachID=9800
🔄 Stage 0: Retry generating image 2/2...
✅ Stage 0: Retry image 2/2 completed: AttachID=9801
✅ Stage 0 retry completed: 2/2 images recovered
📊 Stage 0 final count: 50/50 images

🔄 Stage 2: Starting retry for 1 missing images...
🔄 Stage 2: Retry generating image 1/1...
✅ Stage 2: Retry image 1/1 completed: AttachID=9802
✅ Stage 2 retry completed: 1/1 images recovered
📊 Stage 2 final count: 50/50 images

🔍 All retry attempts completed

🔍 ===== Stage Results Before Merge =====
📦 Stage 0: [9634, 9635, ..., 9800, 9801] (total: 50)
📦 Stage 1: [9684, 9685, ...] (total: 50)
📦 Stage 2: [9734, 9735, ..., 9802] (total: 50)
🎯 Final merged array: [...] (total: 150)
🏁 Pipeline Job finished: 150/150 images completed  ✅ 완벽
```

---

## Benefits

✅ **정확한 갯수 보장**: 요청한 quantity만큼 정확히 생성
✅ **Stage별 독립 재시도**: 부족한 Stage만 재생성
✅ **기존 로직 유지**: 첫 번째 생성 로직은 전혀 수정 안 함
✅ **크레딧 정확성**: 재시도된 이미지도 정확히 크레딧 차감
✅ **순서 보장**: `results[stageIndex]`로 Stage 순서 유지
✅ **로그 추적**: 어떤 Stage에서 몇 개 재시도했는지 명확히 로그 출력

---

## Edge Cases

### Case 1: 재시도도 실패하는 경우
- 재시도 for문에서도 `continue`로 넘어감
- 최종적으로 여전히 부족할 수 있음
- 로그에 `Stage X retry completed: 1/2 images recovered` 출력
- Complete 처리는 되지만 실제 갯수 부족

**해결 방안**: 재시도를 여러 번 반복? (추후 논의)

### Case 2: 모든 Stage가 정상 생성된 경우
- 재시도 로직은 아무것도 안 함 (`if missing <= 0 { continue }`)
- 로그만 `✅ Stage X: Complete` 출력
- 성능 영향 거의 없음

### Case 3: Stage 순서와 병합 순서
- `results[stageIndex]`로 Stage 순서 보장
- 재시도로 추가된 AttachID는 해당 Stage 배열 끝에 append
- 최종 병합 시 Stage 순서대로 합쳐짐 (정상)

---

## Notes

- 재시도 로직은 **동기적**으로 실행 (병렬 아님)
- 이유: 모든 Stage 완료 후 부족분만 처리하므로 병렬 필요 없음
- 세마포어는 GenerateImageWithGemini 함수 내부에서 이미 제어됨
- 재시도 중에도 실시간 progress 업데이트 포함

---

ㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡ

## 복기 (Retrospective)

### 문제 발견
- 사용자가 150장 요청 시 가끔 갯수가 부족한 상황 발생
- 예: 147장만 생성되고 Complete 처리됨
- 각 Stage별로 병렬 처리 중 일부 API 호출 실패로 인한 누락

### 핵심 요구사항
1. **현재 생성 로직은 유지**: 병렬 처리 그대로
2. **체크는 전체 완료 후**: 각 Stage의 첫 번째 for문 모두 완료 후
3. **Stage별 갯수 트래킹**: 요청 gal�수 vs 실제 생성 갯수
4. **부족분만 재생성**: 빠진 갯수만큼 추가 API 호출
5. **확실히 차면 Complete**: 최종 갯수 확인 후 완료 처리

### 해결 방법
- **체크 시점**: `wg.Wait()` 직후 (라인 500-501)
- **체크 방법**: `stages[idx]["quantity"]` vs `len(results[idx].AttachIDs)`
- **재시도**: 차이만큼 두 번째 for문 실행
- **완료 조건**: 최종 병합 후 갯수 검증

### 가능한 이유
- `stages[]` 배열에 모든 요청 정보 보관 (quantity, prompt, aspect-ratio, input image)
- `results[]` 배열에 실제 생성 결과 저장 (AttachIDs, Success count)
- `wg.Wait()` 후 모든 정보 확보 가능
- Stage별 독립적 재시도 가능 (입력 이미지, 프롬프트 재사용)

### 설계 결정
1. **재시도는 동기적**: 병렬 필요 없음 (이미 첫 번째에서 병렬 처리 완료)
2. **세마포어 재사용**: GenerateImageWithGemini 내부에서 자동 제어
3. **크레딧 정확성**: 재시도 이미지도 정상적으로 크레딧 차감
4. **로그 상세화**: 어떤 Stage에서 몇 개 재시도했는지 명확히 출력

### 추후 고려사항
- 재시도도 실패하는 경우 처리 방안 (재재시도? 최대 시도 횟수?)
- 재시도 로그를 별도 테이블에 저장? (통계/모니터링 목적)
- 재시도 성공률 추적?

### 코드 수정 최소화
- **수정 파일**: `modules/generate-image/worker.go` 단 1개
- **수정 위치**: 라인 501 (wg.Wait() 직후) 1곳만
- **기존 로직**: 전혀 수정 안 함 (첫 번째 for문, 병합 로직 그대로)
- **추가 코드**: 약 100줄 (재시도 로직)
