# 🎯 Go Server 모듈 리팩토링 Plan

## Phase 1: 현재 구조 분석 ✅

### 📂 현재 modules/generate-image 파일 구성
- **config.go** (3.3KB) - Config 타입, 환경변수 로드
- **handler.go** (879B) - HTTP 핸들러 (거의 사용 안함)
- **model.go** (3.0KB) - ProductionJob, Attach, Combination 등
- **service.go** (43KB) - Supabase, Storage, Gemini 로직
- **worker.go** (43KB) - Redis BRPOP, Job 처리 로직

### 🔍 주요 함수 분류

#### **공통 로직 (common으로 이동 가능)**

**config.go:**
- `LoadConfig()`, `GetConfig()`, `validate()`

**model.go:**
- `ProductionJob`, `Attach`, `Combination` (모든 모듈 공통)

**service.go (공통):**
- `FetchJobFromSupabase()` - Job 데이터 조회
- `UpdateJobStatus()` - Job 상태 업데이트
- `FetchAttachInfo()` - Attachment 정보 조회
- `DownloadImageFromStorage()` - Storage에서 이미지 다운로드
- `ConvertImageToBase64()` - Base64 변환
- `ConvertPNGToWebP()` - WebP 변환
- `UpdateProductionPhotoStatus()` - Production 상태 업데이트
- `UploadImageToStorage()` - Storage에 업로드
- `CreateAttachRecord()` - Attach 레코드 생성
- `UpdateJobProgress()` - Job 진행상황 업데이트
- `UpdateProductionAttachIds()` - Production attach_ids 업데이트
- `DeductCredits()` - Credit 차감

**worker.go (공통):**
- `StartWorker()` - Redis BRPOP 시작
- `connectRedis()` - Redis 연결
- `base64DecodeString()`, `minInt()` - 유틸리티

#### **카테고리별 로직 (각 모듈에 복사)**

**service.go (Fashion 전용):**
- `GenerateImageWithGemini()` - Gemini API 호출 (단일)
- `GenerateImageWithGeminiMultiple()` - Gemini API 호출 (다중)
- `generateDynamicPrompt()` - Fashion 프롬프트 생성
- `mergeImages()` - 이미지 병합 (Grid)
- `resizeImage()` - 이미지 리사이즈

**worker.go (카테고리별):**
- `processSingleBatch()` - Fashion 단일 배치
- `processPipelineStage()` - Fashion 파이프라인
- `processSimpleGeneral()` - General 탭
- `processSimplePortrait()` - Portrait 탭

---

## Phase 2: 폴더 구조 설계

```
modules/
├── common/                          # 공통 로직
│   ├── config/
│   │   └── config.go               # Config 로드 (LoadConfig, GetConfig, validate)
│   ├── model/
│   │   └── model.go                # ProductionJob, Attach, Combination 등
│   ├── database/
│   │   └── supabase.go             # DB 공통 함수 (Fetch, Update)
│   ├── storage/
│   │   └── storage.go              # Storage 업로드/다운로드
│   ├── redis/
│   │   └── redis.go                # Redis 연결 (connectRedis)
│   ├── credit/
│   │   └── credit.go               # Credit 차감 (DeductCredits)
│   └── utils/
│       └── image.go                # 이미지 변환 (Base64, WebP)
│
├── worker/                          # Worker 진입점
│   ├── worker.go                   # Redis BRPOP, Job dispatch
│   └── router.go                   # quel_production_path 기반 라우팅
│
├── fashion/                         # Fashion 모듈 (기존 generate-image)
│   ├── processor.go                # Fashion 워크플로우 처리 (ProcessJob)
│   ├── prompt.go                   # Fashion 프롬프트 생성 (generateDynamicPrompt)
│   ├── gemini.go                   # Gemini API 호출
│   └── image.go                    # 이미지 병합/리사이즈 (mergeImages, resizeImage)
│
├── beauty/                          # Beauty 모듈 (신규)
│   ├── processor.go
│   ├── prompt.go
│   ├── gemini.go
│   └── image.go
│
├── eats/                            # Eats 모듈 (신규)
│   ├── processor.go
│   ├── prompt.go
│   ├── gemini.go
│   └── image.go
│
├── cinema/                          # Cinema 모듈 (신규)
│   ├── processor.go
│   ├── prompt.go
│   ├── gemini.go
│   └── image.go
│
└── cartoon/                         # Cartoon 모듈 (신규)
    ├── processor.go
    ├── prompt.go
    ├── gemini.go
    └── image.go
```

---

## Phase 3: 공통 인터페이스 설계

```go
// modules/common/processor/interface.go
package processor

import (
    "context"
    "github.com/yourorg/modules/common/model"
)

// Processor - 각 카테고리 모듈이 구현해야 할 인터페이스
type Processor interface {
    // Job 처리
    ProcessJob(ctx context.Context, job *model.ProductionJob) error

    // 입력 검증
    ValidateInput(inputData map[string]interface{}) error

    // 프롬프트 생성
    GeneratePrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string

    // 이미지 생성 (Gemini API)
    GenerateImage(ctx context.Context, categories *ImageCategories, prompt string, aspectRatio string) (string, error)
}

// ImageCategories - 이미지 카테고리 분류
type ImageCategories struct {
    Model       []byte
    Clothing    [][]byte
    Accessories [][]byte
    Background  []byte
}
```

---

## Phase 4: 라우팅 로직

### modules/worker/router.go

```go
package worker

import (
    "context"
    "fmt"
    "log"

    "github.com/yourorg/modules/common/model"
    "github.com/yourorg/modules/fashion"
    "github.com/yourorg/modules/beauty"
    "github.com/yourorg/modules/eats"
    "github.com/yourorg/modules/cinema"
    "github.com/yourorg/modules/cartoon"
)

// RouteJob - quel_production_path에 따라 적절한 모듈로 라우팅
func RouteJob(ctx context.Context, job *model.ProductionJob) error {
    // 1. quel_production_path 확인
    path := job.QuelProductionPath
    if path == "" {
        log.Printf("⚠️  Missing quel_production_path, defaulting to 'fashion'")
        path = "fashion" // Fallback
    }

    log.Printf("🔀 Routing job %s to %s module", job.JobID, path)

    // 2. Path별 라우팅
    switch path {
    case "fashion":
        processor := fashion.NewProcessor()
        return processor.ProcessJob(ctx, job)

    case "beauty":
        processor := beauty.NewProcessor()
        return processor.ProcessJob(ctx, job)

    case "eats":
        processor := eats.NewProcessor()
        return processor.ProcessJob(ctx, job)

    case "cinema":
        processor := cinema.NewProcessor()
        return processor.ProcessJob(ctx, job)

    case "cartoon":
        processor := cartoon.NewProcessor()
        return processor.ProcessJob(ctx, job)

    default:
        return fmt.Errorf("unknown production_path: %s", path)
    }
}
```

### modules/worker/worker.go

```go
package worker

import (
    "context"
    "log"
    "time"

    "github.com/yourorg/modules/common/config"
    "github.com/yourorg/modules/common/database"
    "github.com/yourorg/modules/common/redis"
)

// StartWorker - Redis Queue Worker 시작
func StartWorker() {
    log.Println("🔄 Redis Queue Worker starting...")

    cfg := config.GetConfig()

    // Redis 연결
    rdb := redis.Connect(cfg)
    if rdb == nil {
        log.Fatal("❌ Failed to connect to Redis")
        return
    }
    log.Println("✅ Redis connected successfully")

    // Queue 감시 시작
    log.Println("👀 Watching queue: jobs:queue")

    ctx := context.Background()

    // 무한 루프로 Queue 감시
    for {
        // BRPOP으로 Job 가져오기
        result, err := rdb.BRPop(ctx, 0, "jobs:queue").Result()
        if err != nil {
            log.Printf("❌ Redis BRPOP error: %v", err)
            time.Sleep(5 * time.Second)
            continue
        }

        jobID := result[1]
        log.Printf("🎯 Received new job: %s", jobID)

        // 비동기로 Job 처리
        go processJob(ctx, jobID)
    }
}

// processJob - Job 처리 (라우팅)
func processJob(ctx context.Context, jobID string) {
    log.Printf("🚀 Processing job: %s", jobID)

    // 1. Supabase에서 Job 데이터 조회
    db := database.NewClient()
    job, err := db.FetchJobFromSupabase(jobID)
    if err != nil {
        log.Printf("❌ Failed to fetch job %s: %v", jobID, err)
        return
    }

    // 2. Job 정보 로그
    log.Printf("📦 Job Data:")
    log.Printf("   JobID: %s", job.JobID)
    log.Printf("   JobType: %s", job.JobType)
    log.Printf("   QuelProductionPath: %s", job.QuelProductionPath)
    log.Printf("   TotalImages: %d", job.TotalImages)

    // 3. Path별 라우팅
    if err := RouteJob(ctx, job); err != nil {
        log.Printf("❌ Failed to process job %s: %v", jobID, err)
        db.UpdateJobStatus(ctx, jobID, "failed")
    }
}
```

---

## Phase 5: 마이그레이션 전략

### Step 1: common 폴더 생성 및 공통 로직 이동

**우선순위:**
1. ✅ `modules/common/config/` 생성 → config.go 이동
2. ✅ `modules/common/model/` 생성 → model.go 이동
3. ✅ `modules/common/redis/` 생성 → connectRedis 이동
4. ✅ `modules/common/database/` 생성 → Supabase 관련 함수 이동
5. ✅ `modules/common/storage/` 생성 → Storage 업로드/다운로드 이동
6. ✅ `modules/common/credit/` 생성 → DeductCredits 이동
7. ✅ `modules/common/utils/` 생성 → 이미지 변환 유틸 이동

**작업:**
- generate-image에서 공통 함수 복사
- package 이름 변경
- import 경로 수정

### Step 2: worker 폴더 생성

1. ✅ `modules/worker/` 생성
2. ✅ `worker.go` 작성 (StartWorker, processJob)
3. ✅ `router.go` 작성 (RouteJob)

### Step 3: fashion 모듈 생성 (generate-image 리팩토링)

1. ✅ `modules/fashion/` 생성
2. ✅ `processor.go` 작성 (ProcessJob 구현)
3. ✅ `prompt.go` 작성 (generateDynamicPrompt 이동)
4. ✅ `gemini.go` 작성 (GenerateImageWithGemini* 이동)
5. ✅ `image.go` 작성 (mergeImages, resizeImage 이동)
6. ✅ common import로 변경

**변경점:**
- `processSingleBatch` → `fashion.ProcessJob`에 통합
- `processPipelineStage` → `fashion.ProcessJob`에 통합
- common 패키지 함수 사용

### Step 4: 신규 모듈 생성 (beauty, eats, cinema, cartoon)

**각 모듈 생성:**
1. ✅ fashion 폴더 전체 복사
2. ✅ package 이름 변경
3. ✅ 프롬프트만 카테고리별로 수정
4. ✅ 나머지 로직은 동일하게 유지

**예시: modules/beauty/**
```go
// processor.go
package beauty

import (
    "context"
    "github.com/yourorg/modules/common/database"
    "github.com/yourorg/modules/common/model"
)

type Processor struct {
    db *database.Client
}

func NewProcessor() *Processor {
    return &Processor{
        db: database.NewClient(),
    }
}

func (p *Processor) ProcessJob(ctx context.Context, job *model.ProductionJob) error {
    // Beauty 전용 워크플로우 처리
    // (fashion과 동일한 구조, 프롬프트만 다름)
    return nil
}
```

### Step 5: main.go 수정

```go
// main.go
package main

import (
    "log"
    "github.com/yourorg/modules/worker"
)

func main() {
    log.Println("🚀 Quel Canvas Collaboration Server starting...")

    // Worker 시작 (기존 코드)
    go worker.StartWorker()  // ← 변경된 부분

    // WebSocket 서버 시작 (기존 코드)
    // ...
}
```

### Step 6: 테스트 & 배포

**테스트 순서:**
1. ✅ 로컬에서 fashion 모듈 테스트
2. ✅ beauty 모듈 테스트
3. ✅ eats, cinema, cartoon 모듈 테스트
4. ✅ 통합 테스트

**배포 전략:**
1. 개발 환경 배포
2. Fashion 모듈 우선 테스트
3. 문제 없으면 나머지 모듈 배포
4. 모니터링 & 롤백 준비

---

## Phase 6: 체크리스트

### 🔥 Priority 1: 공통 로직 추출 (필수)

- [ ] `modules/common/config/config.go` 생성
- [ ] `modules/common/model/model.go` 생성
- [ ] `modules/common/redis/redis.go` 생성
- [ ] `modules/common/database/supabase.go` 생성
- [ ] `modules/common/storage/storage.go` 생성
- [ ] `modules/common/credit/credit.go` 생성
- [ ] `modules/common/utils/image.go` 생성

### 🔥 Priority 2: Worker 분리 (필수)

- [ ] `modules/worker/worker.go` 생성
- [ ] `modules/worker/router.go` 생성

### 🔥 Priority 3: Fashion 모듈 리팩토링 (필수)

- [ ] `modules/fashion/` 폴더 생성
- [ ] `modules/fashion/processor.go` 작성
- [ ] `modules/fashion/prompt.go` 작성
- [ ] `modules/fashion/gemini.go` 작성
- [ ] `modules/fashion/image.go` 작성
- [ ] common import로 변경
- [ ] 테스트

### 🔥 Priority 4: 신규 모듈 생성 (확장)

- [ ] `modules/beauty/` 생성 (fashion 복사)
- [ ] `modules/eats/` 생성 (fashion 복사)
- [ ] `modules/cinema/` 생성 (fashion 복사)
- [ ] `modules/cartoon/` 생성 (fashion 복사)
- [ ] 각 모듈 프롬프트 커스터마이징

### 🔥 Priority 5: DB 마이그레이션 (필수)

- [ ] `quel_production_photo.quel_production_path` 컬럼 확인
- [ ] `quel_production_jobs.quel_production_path` 컬럼 확인
- [ ] 기존 데이터 'fashion'으로 업데이트
- [ ] model.go에 QuelProductionPath 필드 추가

### 🔥 Priority 6: 통합 & 배포

- [ ] main.go import 수정
- [ ] 로컬 빌드 테스트
- [ ] 개발 환경 배포
- [ ] 프로덕션 배포
- [ ] 모니터링

---

## Phase 7: 기대 효과

### ✅ 모듈화
- 카테고리별 독립적인 코드 관리
- 코드 재사용성 향상
- 유지보수 용이

### ✅ 확장성
- 새 카테고리 추가 시 fashion 폴더 복사만 하면 됨
- 기존 코드 영향 최소화
- 프롬프트만 수정하면 새 기능 추가 가능

### ✅ 디버깅
- Path별 로그 분리
- 문제 추적 용이
- 각 모듈 독립적으로 테스트 가능

### ✅ 성능
- Path별 Worker 수 조정 가능
- 모듈별 최적화 가능
- 병목 지점 파악 용이

### ✅ 유지보수
- 일관된 폴더 구조
- 명확한 책임 분리
- 코드 가독성 향상

---

## 🚨 주의사항

### 1. 기존 로직 유지
- Fashion 모듈은 기존 generate-image 로직을 **100% 동일**하게 유지
- 프롬프트, 이미지 처리 방식 모두 동일
- 리팩토링 후 동작 검증 필수

### 2. 단계별 진행
- 한 번에 모든 모듈 생성 X
- common → worker → fashion → 신규 모듈 순서로 진행
- 각 단계마다 테스트

### 3. 롤백 준비
- 기존 generate-image 폴더 백업
- Git 브랜치 분리
- 문제 발생 시 즉시 롤백 가능하도록 준비

### 4. DB 호환성
- QuelProductionPath가 NULL인 경우 'fashion' fallback
- 기존 데이터 마이그레이션 필수
- 스키마 변경 후 앱 배포 순서 중요

---

Last Updated: 2025-01-09
