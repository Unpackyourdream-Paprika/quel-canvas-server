# Redis Worker - Gemini API 통합 구현 플랜

**작성일:** 2025-10-01
**목표:** Go Server에서 Redis Queue Job을 처리하여 Gemini API로 이미지 생성 및 Supabase에 저장

---

## 📊 현재 상태 (완료된 부분)

### ✅ Phase 1: 데이터 준비 (완료)
- [x] Redis Queue에서 job_id 수신
- [x] Supabase `quel_production_jobs` 테이블에서 Job 데이터 조회
- [x] `mergedImageAttachId`로 원본 이미지 다운로드
- [x] 이미지 Base64 변환 완료

**로그 확인:**
```
🎯 Received new job: 827fc3b1-...
✅ Job fetched successfully
🖼️  MergedImageAttachID: 2109
✅ Image downloaded: 1598358 bytes
✅ Base64 Image (length: 2131144 chars)
```

---

## 🎯 구현해야 할 부분

### Phase 2: 시작 전 상태 업데이트

#### 2.1 Job 상태 업데이트
```go
// service.go
func (s *Service) UpdateJobStatus(ctx context.Context, jobID string, status string) error

// 호출
UpdateJobStatus(ctx, jobID, "processing")
```

**DB 업데이트:**
```sql
UPDATE quel_production_jobs
SET job_status = 'processing',
    started_at = now(),
    updated_at = now()
WHERE job_id = ?
```

#### 2.2 Production Photo 상태 업데이트
```go
// service.go (새로 만들기)
func (s *Service) UpdateProductionPhotoStatus(ctx context.Context, productionID string, status string) error

// 호출
UpdateProductionPhotoStatus(ctx, productionID, "processing")
```

**DB 업데이트:**
```sql
UPDATE quel_production_photo
SET production_status = 'processing',
    updated_at = now()
WHERE production_id = ?
```

---

### Phase 3: 이미지 생성 루프 (핵심)

#### 3.1 Gemini API 호출
```go
// service.go (새로 만들기)
func (s *Service) GenerateImageWithGemini(base64Image string, prompt string) (string, error)
```

**구현 내용:**
- Google Generative AI SDK 사용
- Model: `gemini-2.5-flash-image-preview`
- Input:
  ```go
  contentParts := []any{
    map[string]string{"text": prompt + "\n\nPlease generate 1 different variation of this image."},
    map[string]any{
      "inlineData": map[string]string{
        "mimeType": "image/png",
        "data": base64Image,
      },
    },
  }
  ```
- Output: 생성된 이미지 (base64 문자열)

**Node.js 참고 코드:**
```typescript
const genAI = await getGoogleClient();
const model = genAI.getGenerativeModel({ model: "gemini-2.5-flash-image-preview" });

const contentParts = [
  { text: prompt },
  { inlineData: { mimeType: img.mimeType, data: img.data } }
];

const result = await model.generateContent(contentParts);
const response = result.response;
```

#### 3.2 Base64 → PNG Buffer 변환
```go
// service.go에 이미 있음
imageData := base64.StdEncoding.DecodeString(base64Image)
```

#### 3.3 Supabase Storage 업로드
```go
// service.go (새로 만들기)
func (s *Service) UploadImageToStorage(imageData []byte, userID string) (string, error)
```

**구현 내용:**
- Path 생성: `generated-images/user-{userId}/generated_{timestamp}_{random}.png`
- HTTP PUT으로 Supabase Storage에 업로드
  ```
  POST https://{project}.supabase.co/storage/v1/object/attachments/{filePath}
  Headers:
    Authorization: Bearer {SERVICE_ROLE_KEY}
    Content-Type: image/png
  Body: imageData (binary)
  ```
- Return: 업로드된 파일 경로

**Node.js 참고 코드:**
```typescript
const filePath = `generated-images/user-${userId}/${fileName}`;
const { data, error } = await supabase.storage
  .from('attachments')
  .upload(filePath, buffer, {
    contentType: mimeType,
    cacheControl: '3600'
  });
```

#### 3.4 quel_attach 테이블에 레코드 생성
```go
// service.go (새로 만들기)
func (s *Service) CreateAttachRecord(filePath string, fileSize int64, mimeType string) (int, error)
```

**DB INSERT:**
```sql
INSERT INTO quel_attach (
  attach_original_name,
  attach_file_name,
  attach_file_path,
  attach_file_size,
  attach_file_type,
  attach_directory,
  attach_storage_type
) VALUES (
  'generated_xxx.png',
  'generated_xxx.png',
  'generated-images/user-{userId}/generated_xxx.png',
  123456,
  'image/png',
  'generated-images/user-{userId}/generated_xxx.png',
  'supabase'
)
RETURNING attach_id
```

**Return:** `attach_id` (예: 2110)

#### 3.5 진행 상황 실시간 업데이트
```go
// service.go (새로 만들기)
func (s *Service) UpdateJobProgress(ctx context.Context, jobID string, completedImages int, generatedAttachIds []int) error
```

**DB 업데이트:**
```sql
UPDATE quel_production_jobs
SET completed_images = ?,
    generated_attach_ids = ?, -- JSONB 배열
    updated_at = now()
WHERE job_id = ?
```

**예시:**
- 1장 완료: `completed_images=1, generated_attach_ids=[2110]`
- 2장 완료: `completed_images=2, generated_attach_ids=[2110, 2111]`

#### 3.6 에러 처리
```go
// worker.go의 루프 내
for i := 0; i < quantity; i++ {
  imageBase64, err := service.GenerateImageWithGemini(base64Image, prompt)
  if err != nil {
    log.Printf("❌ Image generation failed (%d/%d): %v", i+1, quantity, err)
    failedImages++
    continue // 실패해도 계속 진행
  }

  // 성공 시 업로드 및 저장
  // ...
}
```

---

### Phase 4: 완료 처리

#### 4.1 Job 완료 상태 업데이트
```go
// service.go (UpdateJobStatus 재사용)
UpdateJobStatus(ctx, jobID, "completed")
```

**DB 업데이트:**
```sql
UPDATE quel_production_jobs
SET job_status = 'completed',
    completed_at = now(),
    updated_at = now()
WHERE job_id = ?
```

#### 4.2 Production Photo 업데이트
```go
// service.go (새로 만들기)
func (s *Service) UpdateProductionPhotoComplete(
  ctx context.Context,
  productionID string,
  newAttachIds []int,
) error
```

**구현 내용:**
1. 현재 `attach_ids` 조회
2. 새로운 `newAttachIds` 추가 (누적)
3. `generated_image_count` 증가
4. `production_status` → "completed"

**DB 업데이트:**
```sql
-- 1. 현재 attach_ids 조회
SELECT attach_ids FROM quel_production_photo WHERE production_id = ?

-- 2. Go에서 배열 병합
currentAttachIds := [2110, 2111]
newAttachIds := [2112, 2113]
allAttachIds := append(currentAttachIds, newAttachIds...) // [2110, 2111, 2112, 2113]

-- 3. 업데이트
UPDATE quel_production_photo
SET attach_ids = ?, -- JSON 배열
    generated_image_count = generated_image_count + ?,
    production_status = 'completed',
    updated_at = now()
WHERE production_id = ?
```

---

## 🔧 필요한 Go 패키지

### Gemini API SDK
```bash
go get cloud.google.com/go/ai/generativelanguage/apiv1beta
go get google.golang.org/api/option
```

또는 공식 SDK가 있다면:
```bash
go get github.com/google/generative-ai-go
```

---

## 📝 worker.go 최종 구조

```go
func processJob(ctx context.Context, service *Service, jobID string) {
  // Phase 1: 데이터 준비 (✅ 이미 완료)
  job := service.FetchJobFromSupabase(jobID)
  base64Image := ... // 이미 완료

  // Phase 2: 상태 업데이트
  service.UpdateJobStatus(ctx, jobID, "processing")
  service.UpdateProductionPhotoStatus(ctx, job.ProductionID, "processing")

  // Phase 3: 이미지 생성 루프
  quantity := job.JobInputData["quantity"].(int)
  prompt := job.JobInputData["prompt"].(string)
  userId := job.JobInputData["userId"].(string)

  generatedAttachIds := []int{}
  failedImages := 0

  for i := 0; i < quantity; i++ {
    log.Printf("🎨 Generating image %d/%d", i+1, quantity)

    // 3.1 Gemini API 호출
    generatedBase64, err := service.GenerateImageWithGemini(base64Image, prompt)
    if err != nil {
      failedImages++
      continue
    }

    // 3.2 Base64 → Binary
    imageData, _ := base64.StdEncoding.DecodeString(generatedBase64)

    // 3.3 Storage 업로드
    filePath, err := service.UploadImageToStorage(imageData, userId)
    if err != nil {
      failedImages++
      continue
    }

    // 3.4 Attach 레코드 생성
    attachId, err := service.CreateAttachRecord(filePath, len(imageData), "image/png")
    if err != nil {
      failedImages++
      continue
    }

    generatedAttachIds = append(generatedAttachIds, attachId)

    // 3.5 진행 상황 업데이트
    service.UpdateJobProgress(ctx, jobID, i+1, generatedAttachIds)
  }

  // Phase 4: 완료 처리
  service.UpdateJobStatus(ctx, jobID, "completed")
  service.UpdateProductionPhotoComplete(ctx, job.ProductionID, generatedAttachIds)

  log.Printf("✅ Job completed: %d images generated, %d failed", len(generatedAttachIds), failedImages)
}
```

---

## 🎯 구현 순서

### Step 1: service.go 함수 추가
1. `GenerateImageWithGemini()` - Gemini API 호출
2. `UploadImageToStorage()` - Storage 업로드
3. `CreateAttachRecord()` - Attach 레코드 생성
4. `UpdateJobProgress()` - 진행 상황 업데이트
5. `UpdateProductionPhotoStatus()` - Production Photo 상태 업데이트
6. `UpdateProductionPhotoComplete()` - Production Photo 완료 처리

### Step 2: worker.go 수정
- `processJob()` 함수에 Phase 2~4 통합

### Step 3: 테스트
1. 서버 실행
2. Frontend에서 Job 전송
3. 로그 확인:
   ```
   🎨 Generating image 1/2
   📥 Uploading to storage...
   💾 Creating attach record...
   ✅ Progress: 1/2 (50%)
   🎨 Generating image 2/2
   📥 Uploading to storage...
   💾 Creating attach record...
   ✅ Progress: 2/2 (100%)
   ✅ Job completed: 2 images generated, 0 failed
   ```

---

## 📊 에러 처리 정책

### 시나리오 1: 전체 성공
```
completed_images = total_images
failed_images = 0
job_status = "completed"
production_status = "completed"
```

### 시나리오 2: 부분 실패
```
completed_images < total_images
failed_images > 0
job_status = "completed" ✅ (부분 성공도 완료 처리)
production_status = "completed"
```

### 시나리오 3: 전체 실패
```
completed_images = 0
failed_images = total_images
job_status = "failed"
production_status = "failed"
```

---

## 🔍 클라이언트 폴링 시나리오

### 진행 중 (2초마다 폴링)
```json
// 1장 완료
{
  "job_status": "processing",
  "total_images": 2,
  "completed_images": 1,
  "generated_attach_ids": [2110]
}

// 2장 완료
{
  "job_status": "completed",
  "total_images": 2,
  "completed_images": 2,
  "generated_attach_ids": [2110, 2111]
}
```

### 완료 후 클라이언트 처리
```typescript
if (job.job_status === "completed") {
  const resultImages = (job.generated_attach_ids || []).map(
    (attachId: number) => ({ attachId: attachId })
  );
  // [{ attachId: 2110 }, { attachId: 2111 }]
}
```

---

## ✅ 체크리스트

### Phase 2
- [ ] `UpdateJobStatus()` 함수 구현
- [ ] `UpdateProductionPhotoStatus()` 함수 구현
- [ ] worker.go에 Phase 2 통합

### Phase 3
- [ ] Gemini API SDK 설치
- [ ] `GenerateImageWithGemini()` 함수 구현
- [ ] `UploadImageToStorage()` 함수 구현
- [ ] `CreateAttachRecord()` 함수 구현
- [ ] `UpdateJobProgress()` 함수 구현
- [ ] worker.go에 루프 구현
- [ ] 에러 처리 구현

### Phase 4
- [ ] `UpdateProductionPhotoComplete()` 함수 구현
- [ ] worker.go에 Phase 4 통합

### 테스트
- [ ] 전체 성공 시나리오 테스트
- [ ] 부분 실패 시나리오 테스트
- [ ] 전체 실패 시나리오 테스트
- [ ] 클라이언트 폴링 동작 확인

---

## 📌 참고 사항

### Node.js 코드 참고 위치
- API Route: `/app/api/generate-images/route.ts`
- Gemini API 호출 부분
- Storage 업로드 부분
- Attach 레코드 생성 부분

### 환경변수
```env
GEMINI_API_KEY=AIzaSyBpa5PYipzap9DhseRQ1GWLBvc8DtW0Ev8
GEMINI_MODEL=gemini-2.5-flash-image
SUPABASE_STORAGE_BASE_URL=https://lmhyvrgijwckxthuskxx.supabase.co/storage/v1/object/public/attachments/
```

### DB 테이블
- `quel_production_jobs` - Job 진행 상황
- `quel_production_photo` - Production 정보 및 최종 결과
- `quel_attach` - 생성된 이미지 메타데이터
