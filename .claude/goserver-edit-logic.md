# Go Server 모듈별 수정 가이드

이 문서는 `quel_production_path` 값에 따라 각 카테고리별로 다른 동작을 구현하기 위해 어떤 파일을 수정해야 하는지 설명합니다.

## 📁 프로젝트 구조

```
modules/
├── common/           # 공통 로직
│   ├── config/
│   ├── model/
│   ├── redis/
│   ├── database/
│   ├── storage/
│   ├── credit/
│   └── utils/
├── worker/           # Job 라우팅 담당
│   └── worker.go     # quel_production_path 기반 모듈 분기
├── fashion/          # Fashion 카테고리
├── beauty/           # Beauty 카테고리
├── eats/             # Eats 카테고리
├── cinema/           # Cinema 카테고리
└── cartoon/          # Cartoon 카테고리
```

## 🎯 카테고리별 수정 대상 파일

### 1. 프롬프트를 다르게 하려면

각 모듈의 **`prompt.go`** 파일 수정:

- `modules/fashion/prompt.go` - 패션 전용 프롬프트
- `modules/beauty/prompt.go` - 뷰티 전용 프롬프트
- `modules/eats/prompt.go` - 음식 전용 프롬프트
- `modules/cinema/prompt.go` - 시네마 전용 프롬프트
- `modules/cartoon/prompt.go` - 카툰 전용 프롬프트

**주요 함수**: `GenerateDynamicPrompt()`

```go
// 예시: Beauty 모듈의 프롬프트 커스터마이징
func GenerateDynamicPrompt(categories *ImageCategories, userPrompt string, aspectRatio string) string {
    // Beauty 전용 메인 지시사항
    mainInstruction := "[BEAUTY PHOTOGRAPHER'S APPROACH]\n" +
        "You are a world-class beauty photographer...\n"

    // ... 나머지 로직
}
```

### 2. 이미지 생성 로직을 다르게 하려면

각 모듈의 **`service.go`** 파일 수정:

- `modules/fashion/service.go`
- `modules/beauty/service.go`
- `modules/eats/service.go`
- `modules/cinema/service.go`
- `modules/cartoon/service.go`

**주요 함수들**:
- `GenerateImageWithGemini()` - 단일 이미지 생성
- `GenerateImageWithGeminiMultiple()` - 카테고리별 다중 이미지 생성
- `mergeImages()` - 이미지 병합 로직
- `UploadImageToStorage()` - 스토리지 업로드

```go
// 예시: Eats 모듈에서 다른 Gemini 모델 사용
func (s *Service) GenerateImageWithGemini(ctx context.Context, base64Image string, prompt string, aspectRatio string) (string, error) {
    // Eats 전용 설정
    modelName := "gemini-2.5-flash-food" // 커스텀 모델

    // ... 나머지 로직
}
```

### 3. Job 처리 흐름을 다르게 하려면

각 모듈의 **`processor.go`** 파일 수정:

- `modules/fashion/processor.go`
- `modules/beauty/processor.go`
- `modules/eats/processor.go`
- `modules/cinema/processor.go`
- `modules/cartoon/processor.go`

**주요 함수들**:
- `ProcessJob()` - 모듈 진입점
- `processSingleBatch()` - 단일 배치 처리
- `processPipelineStage()` - 파이프라인 단계 처리
- `processSimpleGeneral()` - 일반 간단 처리
- `processSimplePortrait()` - 인물 간단 처리

```go
// 예시: Cinema 모듈에서 다른 Job Type 추가
func ProcessJob(ctx context.Context, job *model.ProductionJob) {
    log.Printf("🎬 [CINEMA MODULE] Job %s started", job.JobID)

    service := NewService()

    switch job.JobType {
    case "single_batch":
        processSingleBatch(ctx, service, job)
    case "cinematic_wide": // Cinema 전용 타입
        processCinematicWide(ctx, service, job)
    default:
        processSingleBatch(ctx, service, job)
    }
}
```

## 🔄 라우팅 로직 (수정 불필요)

**`modules/worker/worker.go`** - 이미 구현됨

```go
func processJob(ctx context.Context, dbClient *database.Client, jobID string) {
    job, err := dbClient.FetchJobFromSupabase(jobID)

    path := job.QuelProductionPath
    if path == "" {
        path = "fashion" // 기본값
    }

    // path 값에 따라 모듈 라우팅
    switch path {
    case "fashion":
        fashion.ProcessJob(ctx, job)
    case "beauty":
        beauty.ProcessJob(ctx, job)
    case "eats":
        eats.ProcessJob(ctx, job)
    case "cinema":
        cinema.ProcessJob(ctx, job)
    case "cartoon":
        cartoon.ProcessJob(ctx, job)
    default:
        fashion.ProcessJob(ctx, job)
    }
}
```

## 📊 데이터베이스 (수정 불필요)

`quel_production_jobs` 테이블의 `quel_production_path` 컬럼 값:
- `"fashion"` 또는 `NULL` → Fashion 모듈
- `"beauty"` → Beauty 모듈
- `"eats"` → Eats 모듈
- `"cinema"` → Cinema 모듈
- `"cartoon"` → Cartoon 모듈

## 🚀 빌드 및 실행

```bash
# 빌드
go build -o quel-canvas-server.exe

# 실행
./quel-canvas-server.exe
```

## 📝 로그 확인

각 모듈이 호출될 때 다음과 같은 로그가 출력됩니다:

```
👗 [FASHION MODULE] Job abc123 started (quel_production_path: fashion)
💄 [BEAUTY MODULE] Job def456 started (quel_production_path: beauty)
🍔 [EATS MODULE] Job ghi789 started (quel_production_path: eats)
🎬 [CINEMA MODULE] Job jkl012 started (quel_production_path: cinema)
🎨 [CARTOON MODULE] Job mno345 started (quel_production_path: cartoon)
```

## ⚠️ 주의사항

1. **현재 상태**: 모든 모듈이 Fashion 모듈의 복사본으로 동일한 로직 사용
2. **수정 시작점**: `prompt.go` 파일부터 시작하여 점진적으로 커스터마이징 권장
3. **공통 로직**: `modules/common/` 내 파일들은 모든 모듈이 공유하므로 신중히 수정
4. **빌드 필수**: 파일 수정 후 반드시 `go build` 실행

## 🔍 디버깅

특정 모듈의 동작을 확인하려면:

1. 해당 모듈의 `processor.go`에 로그 추가
2. `service.go`의 각 함수에 상세 로그 추가
3. 빌드 후 실행하여 로그 확인

---

생성일: 2025-11-09
