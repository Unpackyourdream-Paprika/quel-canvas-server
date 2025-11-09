# Production Path-based Workflow Architecture

Go Server용 카테고리별 워크플로우 처리 가이드

---

## 📋 Overview

`quel_production_path` 컬럼을 기반으로 카테고리별 모듈화된 워크플로우 처리 시스템

---

## 🗂️ Database Schema Changes

### quel_production_photo
```sql
ALTER TABLE public.quel_production_photo
ADD COLUMN quel_production_path VARCHAR(50);
```

### quel_production_jobs
```sql
ALTER TABLE public.quel_production_jobs
ADD COLUMN quel_production_path VARCHAR(50);
```

**가능한 값:**
- `fashion` (기존 로직)
- `beauty` (신규 모듈)
- `eats` (신규 모듈)
- `cinema` (신규 모듈)
- `cartoon` (신규 모듈)

---

## 🔄 Worker Flow (Go Server)

### 1. Redis Queue에서 Job 가져오기 (BRPOP)

```go
// Redis에서 job 가져오기
jobData, err := redisClient.BRPop(ctx, 0, "quel_jobs_queue").Result()

// job_id 파싱
jobID := parseJobID(jobData)
```

### 2. Database에서 Job 정보 조회

```go
// quel_production_jobs 테이블에서 조회
type ProductionJob struct {
    JobID               string  `db:"job_id"`
    ProductionID        string  `db:"production_id"`
    QuelMemberID        string  `db:"quel_member_id"`
    JobType             string  `db:"job_type"`
    JobStatus           string  `db:"job_status"`
    JobInputData        json.RawMessage `db:"job_input_data"`
    QuelProductionPath  string  `db:"quel_production_path"`  // ⭐ 새로 추가된 컬럼
    // ... other fields
}

var job ProductionJob
err := db.Get(&job, `
    SELECT
        job_id,
        production_id,
        quel_member_id,
        job_type,
        job_status,
        job_input_data,
        quel_production_path
    FROM quel_production_jobs
    WHERE job_id = $1
`, jobID)
```

### 3. Production Path 기반 라우팅

```go
// Path에 따라 다른 모듈로 라우팅
switch job.QuelProductionPath {
case "fashion":
    // 기존 로직 유지
    processFashionWorkflow(job)

case "beauty":
    // 신규 모듈: modules/beauty/workflow.go
    processBeautyWorkflow(job)

case "eats":
    // 신규 모듈: modules/eats/workflow.go
    processEatsWorkflow(job)

case "cinema":
    // 신규 모듈: modules/cinema/workflow.go
    processCinemaWorkflow(job)

case "cartoon":
    // 신규 모듈: modules/cartoon/workflow.go
    processCartoonWorkflow(job)

default:
    // Fallback: fashion 로직 사용 (하위 호환성)
    log.Warn("Unknown production_path, using fashion workflow",
             "path", job.QuelProductionPath)
    processFashionWorkflow(job)
}
```

---

## 📁 Go Server 폴더 구조 (제안)

```
goserver/
├── main.go
├── worker/
│   ├── worker.go              # Redis BRPOP 및 job dispatch
│   └── router.go              # Path 기반 라우팅 로직
│
├── modules/
│   ├── fashion/               # 기존 로직
│   │   ├── workflow.go        # processFashionWorkflow()
│   │   ├── nodes.go           # Fashion-specific node handlers
│   │   └── utils.go
│   │
│   ├── beauty/                # 신규 모듈
│   │   ├── workflow.go        # processBeautyWorkflow()
│   │   ├── nodes.go           # Beauty-specific node handlers
│   │   └── utils.go
│   │
│   ├── eats/                  # 신규 모듈
│   │   ├── workflow.go        # processEatsWorkflow()
│   │   ├── nodes.go           # Eats-specific node handlers
│   │   └── utils.go
│   │
│   ├── cinema/                # 신규 모듈
│   │   ├── workflow.go        # processCinemaWorkflow()
│   │   ├── nodes.go           # Cinema-specific node handlers
│   │   └── utils.go
│   │
│   └── cartoon/               # 신규 모듈
│       ├── workflow.go        # processCartoonWorkflow()
│       ├── nodes.go           # Cartoon-specific node handlers
│       └── utils.go
│
├── shared/                    # 공통 유틸리티
│   ├── comfy/                 # ComfyUI API wrapper
│   ├── storage/               # S3/Storage handling
│   ├── database/              # DB queries
│   └── credit/                # Credit deduction
│
└── config/
    └── config.go
```

---

## 🎯 구현 전략

### Phase 1: 기존 로직 유지 (Fashion)
```go
// 기존 fashion 워크플로우를 그대로 유지
// modules/fashion/workflow.go로 이동
func processFashionWorkflow(job ProductionJob) error {
    // 기존 로직 그대로 사용
    // ... (현재 worker 코드)
}
```

### Phase 2: 신규 모듈 구조화 (Beauty, Eats, Cinema, Cartoon)
```go
// modules/beauty/workflow.go
func processBeautyWorkflow(job ProductionJob) error {
    // Beauty 카테고리 전용 워크플로우
    // 1. job_input_data 파싱
    // 2. Beauty-specific node 처리
    // 3. ComfyUI 호출
    // 4. 결과 저장
    // 5. Credit 차감
    return nil
}

// modules/eats/workflow.go
func processEatsWorkflow(job ProductionJob) error {
    // Eats 카테고리 전용 워크플로우
    // ...
    return nil
}

// ... cinema, cartoon도 동일 패턴
```

### Phase 3: 공통 로직 추출
```go
// shared/workflow/base.go
type WorkflowProcessor interface {
    ValidateInput(inputData json.RawMessage) error
    ProcessNodes(nodes []Node) ([]Image, error)
    SaveResults(images []Image, productionID string) error
    DeductCredits(memberID string, amount int) error
}

// 각 모듈이 이 인터페이스를 구현
type FashionProcessor struct { ... }
type BeautyProcessor struct { ... }
type EatsProcessor struct { ... }
// ...
```

---

## 🔍 검사 로직 (Validation)

### BRPOP 후 검사 항목

```go
func validateJob(job ProductionJob) error {
    // 1. Production Path 존재 여부
    if job.QuelProductionPath == "" {
        log.Warn("Missing production_path, defaulting to fashion")
        job.QuelProductionPath = "fashion"
    }

    // 2. 지원하는 Path인지 확인
    validPaths := []string{"fashion", "beauty", "eats", "cinema", "cartoon"}
    if !contains(validPaths, job.QuelProductionPath) {
        return fmt.Errorf("unsupported production_path: %s", job.QuelProductionPath)
    }

    // 3. Job Input Data 유효성
    if len(job.JobInputData) == 0 {
        return fmt.Errorf("empty job_input_data")
    }

    // 4. Job Status 확인
    if job.JobStatus != "pending" {
        return fmt.Errorf("job already processed: %s", job.JobStatus)
    }

    return nil
}
```

---

## 📊 DB Query 패턴

### Job 조회 with Production Path

```go
// 특정 Path의 Pending Jobs 조회
func GetPendingJobsByPath(path string) ([]ProductionJob, error) {
    var jobs []ProductionJob
    err := db.Select(&jobs, `
        SELECT * FROM quel_production_jobs
        WHERE job_status = 'pending'
          AND quel_production_path = $1
        ORDER BY created_at ASC
        LIMIT 100
    `, path)
    return jobs, err
}

// Production 정보 조회 with Path
func GetProductionWithPath(productionID string) (*Production, error) {
    var prod Production
    err := db.Get(&prod, `
        SELECT
            production_id,
            quel_member_id,
            workflow_data,
            quel_production_path
        FROM quel_production_photo
        WHERE production_id = $1
    `, productionID)
    return &prod, err
}
```

---

## ⚙️ 설정 예시 (config.yaml)

```yaml
worker:
  redis:
    queue_name: "quel_jobs_queue"
    timeout: 30s

  modules:
    fashion:
      enabled: true
      max_workers: 5
    beauty:
      enabled: true
      max_workers: 3
    eats:
      enabled: true
      max_workers: 3
    cinema:
      enabled: true
      max_workers: 3
    cartoon:
      enabled: true
      max_workers: 3

  fallback:
    default_path: "fashion"
    unknown_path_behavior: "use_default"  # or "reject"
```

---

## 🚀 마이그레이션 계획

### Step 1: 컬럼 추가 (완료)
```sql
ALTER TABLE public.quel_production_photo ADD COLUMN quel_production_path VARCHAR(50);
ALTER TABLE public.quel_production_jobs ADD COLUMN quel_production_path VARCHAR(50);
```

### Step 2: 기존 데이터 업데이트
```sql
-- 기존 레코드는 모두 fashion으로 설정
UPDATE quel_production_photo
SET quel_production_path = 'fashion'
WHERE quel_production_path IS NULL;

UPDATE quel_production_jobs
SET quel_production_path = 'fashion'
WHERE quel_production_path IS NULL;
```

### Step 3: Frontend에서 Path 전달
```typescript
// src/app/api/jobs/create/route.ts
const category = req.body.category || 'fashion'; // fashion/beauty/eats/cinema/cartoon

await supabase.from('quel_production_photo').insert({
  ...productionData,
  quel_production_path: category
});

await supabase.from('quel_production_jobs').insert({
  ...jobData,
  quel_production_path: category
});
```

### Step 4: Go Server 배포
1. 기존 fashion 로직을 `modules/fashion/`로 이동
2. Router 로직 추가
3. 신규 모듈은 fashion 로직 복사 후 점진적 수정
4. 배포 후 모니터링

---

## 📝 체크리스트

### Backend (Go Server)
- [ ] `worker/router.go` 구현 - Path 기반 라우팅
- [ ] `modules/fashion/` 폴더 생성 및 기존 로직 이동
- [ ] `modules/beauty/` 보일러플레이트 생성
- [ ] `modules/eats/` 보일러플레이트 생성
- [ ] `modules/cinema/` 보일러플레이트 생성
- [ ] `modules/cartoon/` 보일러플레이트 생성
- [ ] `shared/workflow/` 공통 인터페이스 정의
- [ ] DB 쿼리에 `quel_production_path` 컬럼 추가
- [ ] Validation 로직 구현
- [ ] Fallback 로직 구현 (unknown path → fashion)

### Database
- [x] `quel_production_photo` 테이블에 컬럼 추가
- [x] `quel_production_jobs` 테이블에 컬럼 추가
- [ ] 기존 데이터 마이그레이션 (NULL → 'fashion')
- [ ] 인덱스 추가 (성능 최적화)

### Frontend
- [ ] `/api/jobs/create` - category 파라미터 전달
- [ ] CategorySelector에서 선택된 category 전달
- [ ] Workflow data에 category 정보 포함

### Monitoring
- [ ] Path별 Job 처리 통계 수집
- [ ] 에러 로그에 Path 정보 포함
- [ ] Performance 모니터링 (Path별 처리 시간)

---

## 🎯 기대 효과

1. **모듈화**: 카테고리별 독립적인 워크플로우 관리
2. **확장성**: 새 카테고리 추가 시 기존 코드 영향 최소화
3. **디버깅**: Path별 로그 분리로 문제 추적 용이
4. **성능**: Path별 Worker 수 조정 가능
5. **유지보수**: 보일러플레이트 기반으로 일관된 구조

---

Last Updated: 2025-01-09
