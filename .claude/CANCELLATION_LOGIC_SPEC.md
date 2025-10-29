# Job Cancellation Logic Specification

## Overview
사용자가 이미지 생성 작업을 중간에 취소할 수 있는 기능입니다. 취소 시점까지 생성된 이미지는 보존되며, 크레딧도 생성된 만큼만 차감됩니다.

---

## Problem Statement

### 현재 상황
- 150개 이미지 생성 요청 시 무조건 끝까지 실행
- 사용자가 중간에 멈출 방법이 없음
- 10개만 확인하고 싶어도 150개 완료까지 기다려야 함
- 한 번 시작하면 되돌릴 수 없음

### 요구사항
- 사용자가 중간에 "취소" 가능
- 취소 시점까지 생성된 이미지는 보존
- 크레딧은 생성된 이미지만큼만 차감
- 실시간으로 생성 중인 결과 확인 가능

---

## Job Status Flow

### 기존 상태값
```
pending → processing → completed
                    ↓
                  failed
```

### 추가된 상태값
```
pending → processing → completed
                    ↓
                  failed
                    ↓
                cancelled  ← 새로 추가
```

**상태값 목록:**
```sql
-- quel_image_generation_job.status
- pending     : 대기 중
- processing  : 생성 중
- completed   : 완료
- failed      : 실패
- cancelled   : 취소됨 (중간에 중단)
```

---

## Cancellation Flow

### 프론트엔드 → 백엔드

**1. 사용자가 "취소" 버튼 클릭**

**2. API 호출:**
```http
PATCH /api/jobs/{job_id}/cancel
```

**3. 서버에서 Job 상태 업데이트:**
```go
UPDATE quel_image_generation_job
SET status = 'cancelled'
WHERE job_id = '{job_id}'
```

**4. Worker가 다음 이미지 생성 전에 상태 확인:**
```go
for i := 0; i < quantity; i++ {
    // Job 상태 확인
    if isCancelled(job.JobID) {
        break  // 여기까지 생성된 것만 저장
    }

    // 이미지 생성 계속
}
```

---

## Implementation Design

### 1. API Endpoint 추가

**파일:** `main.go` (또는 라우터 파일)

```go
// 취소 엔드포인트 추가
http.HandleFunc("/api/jobs/cancel", handleJobCancel)

func handleJobCancel(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req struct {
        JobID string `json:"job_id"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Job 상태를 cancelled로 업데이트
    ctx := context.Background()
    service := generateimage.NewService()

    err := service.CancelJob(ctx, req.JobID)
    if err != nil {
        log.Printf("❌ Failed to cancel job: %v", err)
        http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
        return
    }

    log.Printf("✅ Job %s cancelled by user", req.JobID)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Job cancelled successfully",
        "job_id":  req.JobID,
    })
}
```

### 2. Service에 CancelJob 함수 추가

**파일:** `modules/generate-image/service.go`

```go
// CancelJob - Job 취소 (상태를 cancelled로 변경)
func (s *Service) CancelJob(ctx context.Context, jobID string) error {
    log.Printf("🚫 Cancelling job: %s", jobID)

    updateData := map[string]interface{}{
        "status": "cancelled",
    }

    _, _, err := s.supabase.From("quel_image_generation_job").
        Update(updateData, "", "").
        Eq("job_id", jobID).
        Execute()

    if err != nil {
        return fmt.Errorf("failed to cancel job: %w", err)
    }

    log.Printf("✅ Job %s status updated to cancelled", jobID)
    return nil
}

// IsJobCancelled - Job이 취소되었는지 확인
func (s *Service) IsJobCancelled(ctx context.Context, jobID string) bool {
    var result []struct {
        Status string `json:"status"`
    }

    _, err := s.supabase.From("quel_image_generation_job").
        Select("status", "", false).
        Eq("job_id", jobID).
        ExecuteTo(&result)

    if err != nil || len(result) == 0 {
        return false
    }

    isCancelled := result[0].Status == "cancelled"
    if isCancelled {
        log.Printf("🚫 Job %s is cancelled", jobID)
    }

    return isCancelled
}
```

### 3. Worker에서 취소 체크

**파일:** `modules/generate-image/worker.go`

**수정 위치 1: Pipeline Stage 처리 (라인 421-485)**

```go
// Stage별 이미지 생성 루프
for i := 0; i < quantity; i++ {
    // ========== 취소 체크 추가 ==========
    if service.IsJobCancelled(ctx, job.JobID) {
        log.Printf("🚫 Stage %d: Job cancelled by user, stopping at %d/%d", stageIndex, i, quantity)
        break
    }
    // ===================================

    log.Printf("🎨 Stage %d: Generating image %d/%d...", stageIndex, i+1, quantity)

    // Gemini API 호출
    generatedBase64, err := service.GenerateImageWithGemini(ctx, base64Image, prompt, aspectRatio)
    if err != nil {
        log.Printf("❌ Stage %d: Gemini API failed for image %d: %v", stageIndex, i+1, err)
        continue
    }

    // ... 나머지 로직
}
```

**수정 위치 2: 재시도 루프 (라인 553-613)**

```go
// 재시도 루프
for i := 0; i < missing; i++ {
    // ========== 취소 체크 추가 ==========
    if service.IsJobCancelled(ctx, job.JobID) {
        log.Printf("🚫 Stage %d: Job cancelled during retry, stopping at %d/%d", stageIdx, i, missing)
        break
    }
    // ===================================

    log.Printf("🔄 Stage %d: Retry generating image %d/%d...", stageIdx, i+1, missing)

    // Gemini API 호출
    // ...
}
```

**수정 위치 3: Simple General 처리 (라인 700-860)**

```go
for i := 0; i < quantity; i++ {
    // ========== 취소 체크 추가 ==========
    if service.IsJobCancelled(ctx, job.JobID) {
        log.Printf("🚫 Simple General: Job cancelled by user, stopping at %d/%d", i, quantity)
        break
    }
    // ===================================

    // 이미지 생성 로직
    // ...
}
```

**수정 위치 4: Simple Portrait 처리 (라인 870-1015)**

```go
for i := 0; i < len(mergedImages); i++ {
    // ========== 취소 체크 추가 ==========
    if service.IsJobCancelled(ctx, job.JobID) {
        log.Printf("🚫 Simple Portrait: Job cancelled by user, stopping at %d/%d", i, len(mergedImages))
        break
    }
    // ===================================

    // 이미지 생성 로직
    // ...
}
```

---

## Cancellation Behavior

### 취소 시 동작

**1. 즉시 중단:**
- 다음 이미지 생성 전에 중단
- 현재 생성 중인 이미지는 완료될 때까지 대기 (Gemini API 호출 중단 불가)

**2. 생성된 이미지 보존:**
- 취소 시점까지 생성된 이미지는 모두 저장
- `generated_attach_ids` 배열에 포함
- Storage에 업로드 완료

**3. 크레딧 처리:**
- 생성된 이미지만큼만 크레딧 차감
- 예: 150개 요청 → 50개 생성 후 취소 → 50개만 크레딧 차감

**4. 최종 상태:**
```go
// Job 상태: cancelled
// completed_images: 50
// total_images: 150
// generated_attach_ids: [9001, 9002, ..., 9050]
```

---

## Performance Considerations

### DB 조회 빈도

**문제:**
- 매 이미지 생성마다 DB 조회 시 성능 저하

**해결 방안:**

**옵션 1: 매번 체크 (가장 반응 빠름)**
```go
for i := 0; i < quantity; i++ {
    if service.IsJobCancelled(ctx, job.JobID) {
        break
    }
    // 생성...
}
```

**옵션 2: N개마다 체크 (성능 최적화)**
```go
for i := 0; i < quantity; i++ {
    // 5개마다 체크
    if i%5 == 0 && service.IsJobCancelled(ctx, job.JobID) {
        break
    }
    // 생성...
}
```

**옵션 3: Context 기반 (권장)**
```go
// Context에 timeout 설정
ctx, cancel := context.WithCancel(context.Background())

// 별도 goroutine에서 주기적으로 체크
go func() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            if service.IsJobCancelled(ctx, job.JobID) {
                cancel()  // Context 취소
                return
            }
        case <-ctx.Done():
            return
        }
    }
}()

// 메인 루프에서 Context 확인
for i := 0; i < quantity; i++ {
    select {
    case <-ctx.Done():
        log.Printf("🚫 Job cancelled")
        break
    default:
        // 생성 계속
    }
}
```

---

## Frontend Integration

### API 호출 예시

```typescript
// 취소 버튼 클릭 시
async function cancelJob(jobId: string) {
  try {
    const response = await fetch('/api/jobs/cancel', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ job_id: jobId }),
    });

    const data = await response.json();

    if (data.success) {
      console.log('Job cancelled successfully');
      // UI 업데이트: "취소됨" 상태 표시
    }
  } catch (error) {
    console.error('Failed to cancel job:', error);
  }
}
```

### 실시간 진행 상황 확인

```typescript
// 주기적으로 Job 상태 폴링
const pollJobStatus = async (jobId: string) => {
  const interval = setInterval(async () => {
    const job = await fetchJob(jobId);

    // 생성된 이미지 표시
    displayGeneratedImages(job.generated_attach_ids);

    // 상태 확인
    if (job.status === 'completed' ||
        job.status === 'cancelled' ||
        job.status === 'failed') {
      clearInterval(interval);

      if (job.status === 'cancelled') {
        showMessage('작업이 취소되었습니다. 생성된 이미지: ' + job.completed_images);
      }
    }
  }, 2000);  // 2초마다 확인
};
```

---

## Expected Log Output

### 정상 완료
```
🎨 Stage 0: Generating image 1/50...
✅ Stage 0: Image 1/50 completed: AttachID=9001
🎨 Stage 0: Generating image 2/50...
✅ Stage 0: Image 2/50 completed: AttachID=9002
...
🏁 Pipeline Job finished: 150/150 images completed
```

### 중간 취소
```
🎨 Stage 0: Generating image 1/50...
✅ Stage 0: Image 1/50 completed: AttachID=9001
🎨 Stage 0: Generating image 2/50...
✅ Stage 0: Image 2/50 completed: AttachID=9002
🎨 Stage 0: Generating image 3/50...
🚫 Stage 0: Job cancelled by user, stopping at 3/50

🎨 Stage 1: Generating image 1/50...
✅ Stage 1: Image 1/50 completed: AttachID=9051
🚫 Stage 1: Job cancelled by user, stopping at 1/50

🎬 Stage 0 completed: 2/50 images generated
🎬 Stage 1 completed: 1/50 images generated
🎬 Stage 2 completed: 0/50 images generated

🏁 Pipeline Job cancelled: 3/150 images completed
```

---

## Edge Cases

### Case 1: 취소 요청 중 이미지 생성 완료
- 현재 생성 중인 이미지는 완료될 때까지 대기
- 다음 이미지부터 중단

### Case 2: 재시도 중 취소
- 재시도 루프도 즉시 중단
- 재시도로 생성된 이미지도 보존

### Case 3: 병렬 Stage 중 취소
- 모든 Stage goroutine이 각각 취소 확인
- 각 Stage는 독립적으로 중단

### Case 4: 크레딧 차감 타이밍
- 이미 차감된 크레딧은 복구 안 함
- 취소 후 생성되지 않은 이미지는 차감 안 됨

---

## Benefits

✅ **사용자 제어권**: 언제든 작업 중단 가능
✅ **비용 절감**: 불필요한 이미지 생성 방지, 크레딧 절약
✅ **유연성**: 일부 결과만 보고 판단 가능
✅ **데이터 보존**: 취소해도 생성된 이미지는 유지
✅ **즉각 반응**: 취소 요청 후 빠르게 중단

---

## Implementation Priority

### Phase 1: 기본 취소 기능 (필수)
- [x] Job status에 'cancelled' 추가
- [ ] API 엔드포인트 추가 (`/api/jobs/cancel`)
- [ ] Service.CancelJob() 함수 추가
- [ ] Service.IsJobCancelled() 함수 추가
- [ ] Worker 각 루프에 취소 체크 추가

### Phase 2: 성능 최적화 (권장)
- [ ] Context 기반 취소 구현
- [ ] 주기적 체크로 DB 부하 감소

### Phase 3: 프론트엔드 연동
- [ ] 취소 버튼 UI 추가
- [ ] 취소 API 호출 구현
- [ ] 취소 상태 표시

---

ㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡㅡ

## 복기 (Retrospective)

### 문제 인식
- 사용자가 150개 요청 후 중간에 멈출 방법이 없음
- 한번 시작하면 무조건 끝까지 실행
- 비용과 시간 낭비 가능성

### 핵심 요구사항
1. **사용자가 중간에 취소 가능**
2. **취소 시점까지 생성된 이미지는 보존**
3. **크레딧은 생성된 만큼만 차감**
4. **실시간으로 생성 중인 결과 확인**

### 해결 방법
- Job status에 'cancelled' 상태 추가
- 각 이미지 생성 전 Job 상태 확인
- 취소 감지 시 즉시 루프 중단 (break)
- 생성된 결과는 모두 DB에 저장

### 설계 결정
1. **즉시 중단**: 다음 이미지 생성 전 체크
2. **데이터 보존**: 생성된 이미지는 모두 유지
3. **크레딧 정확성**: 생성된 것만 차감 (이미 구현됨)
4. **성능 고려**: Context 기반 또는 주기적 체크로 최적화

### 구현 복잡도
- **낮음**: 기존 구조에 취소 체크만 추가
- API 엔드포인트 1개
- Service 함수 2개
- Worker 체크 로직 4곳

### 추후 고려사항
- 취소 후 재시작 기능?
- 일시정지(pause) 기능?
- 취소된 Job 통계/모니터링?
