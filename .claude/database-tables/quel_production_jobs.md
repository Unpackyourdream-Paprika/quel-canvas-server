# quel_production_jobs

이미지 생성 작업(Job) 관리 테이블

## Schema

```sql
create table public.quel_production_jobs (
  job_id uuid not null default gen_random_uuid (),
  production_id uuid not null,
  job_type character varying not null,
  stage_index integer null,
  stage_name character varying null,
  batch_index integer null,
  job_status public.job_status_enum null default 'pending'::job_status_enum,
  total_images integer not null,
  completed_images integer null default 0,
  failed_images integer null default 0,
  job_input_data jsonb not null,
  generated_attach_ids jsonb null default '[]'::jsonb,
  error_message text null,
  retry_count integer null default 0,
  created_at timestamp with time zone null default now(),
  started_at timestamp with time zone null,
  completed_at timestamp with time zone null,
  updated_at timestamp with time zone null default now(),
  quel_member_id uuid null,
  org_id uuid null references quel_organization(org_id),
  estimated_credits integer null default 0,
  remaining_credits numeric null default 0,
  quel_production_path character varying null,
  constraint quel_production_jobs_pkey primary key (job_id),
  constraint quel_production_jobs_production_id_fkey foreign key (production_id)
    references quel_production_photo (production_id)
) TABLESPACE pg_default;

create index IF not exists idx_quel_production_jobs_org_id on public.quel_production_jobs using btree (
  org_id
) TABLESPACE pg_default;
```

## Key Columns

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| job_id | uuid | NO | gen_random_uuid() | Job ID (PK) |
| production_id | uuid | NO | - | 프로덕션 ID (FK → quel_production_photo) |
| job_type | varchar | NO | - | Job 타입 (single_batch/pipeline_stage/simple_general/simple_portrait) |
| stage_index | integer | YES | - | 파이프라인 스테이지 인덱스 (pipeline_stage용) |
| stage_name | varchar | YES | - | 스테이지 이름 (pipeline_stage용) |
| batch_index | integer | YES | - | 배치 인덱스 (single_batch용) |
| job_status | enum | YES | 'pending' | 상태 (pending/processing/completed/failed) |
| total_images | integer | NO | - | 총 생성할 이미지 수 |
| completed_images | integer | YES | 0 | 완료된 이미지 수 |
| failed_images | integer | YES | 0 | 실패한 이미지 수 |
| job_input_data | jsonb | NO | - | 입력 데이터 (prompt, uploadedImages, cameraAngle, shotType) |
| generated_attach_ids | jsonb | YES | '[]' | 생성된 이미지 attach ID 목록 |
| error_message | text | YES | - | 에러 메시지 |
| retry_count | integer | YES | 0 | 재시도 횟수 |
| created_at | timestamptz | YES | now() | 생성 시간 |
| started_at | timestamptz | YES | - | 처리 시작 시간 |
| completed_at | timestamptz | YES | - | 완료 시간 |
| updated_at | timestamptz | YES | now() | 수정 시간 |
| quel_member_id | uuid | YES | - | 멤버 ID (FK → quel_member) |
| org_id | uuid | YES | - | 조직 ID (FK → quel_organization) - nullable |
| estimated_credits | integer | YES | 0 | 예상 크레딧 (total_images * 20) |
| remaining_credits | numeric | YES | 0 | 남은 크레딧 |
| quel_production_path | varchar | YES | - | 프로덕션 경로 (fashion/beauty/eats/cinema/cartoon) |

## Job Types

| Type | Description |
|------|-------------|
| single_batch | 단일 배치 작업 |
| pipeline_stage | 멀티 스테이지 파이프라인 |
| simple_general | 심플 일반 모드 |
| simple_portrait | 심플 인물 모드 |

## Job Status Flow

```
pending → processing → completed
                   ↘ failed
```

## API Endpoints

### POST /api/jobs/create
Job 생성

```typescript
// Request
{
  production_id: string,
  quel_member_id: string,
  job_type: 'single_batch' | 'pipeline_stage' | 'simple_general' | 'simple_portrait',
  stage_index?: number,        // pipeline_stage용
  stage_name?: string,         // pipeline_stage용
  batch_index?: number,        // single_batch용
  total_images: number,
  job_input_data: {
    prompt: string,
    uploadedImages: string[],
    cameraAngle?: string,
    shotType?: string
  },
  quel_production_path?: string  // fashion/beauty/eats/cinema/cartoon
}

// Response
{
  success: true,
  job_id: string,
  job_status: 'pending',
  message: 'Job created successfully'
}
```

### POST /api/jobs/enqueue
Job을 Redis 큐에 추가

```typescript
// Request
{ job_id: string }

// Response
{
  success: true,
  message: 'Job enqueued successfully',
  job_id: string,
  queue: 'jobs:queue',
  queuePosition: number
}
```

### GET /api/jobs
멤버의 Job 목록 조회

```typescript
// Query params
?userId=<member_id>&status=pending,processing

// Response
{ jobs: Job[] }
```

## Data Flow

```
1. User clicks GENERATE on /visual/{category}
   ↓
2. POST /api/jobs/create
   - Check/Create quel_production_photo (if not exists)
   - Calculate estimated_credits (total_images * 20)
   - Check member credits
   - INSERT quel_production_jobs (status: pending)
   ↓
3. POST /api/jobs/enqueue
   - Add job_id to Redis queue
   ↓
4. Go Worker picks up job from Redis queue
   ↓
5. Worker processes images
   - UPDATE job_status: processing → completed/failed
   - UPDATE completed_images, generated_attach_ids
   ↓
6. UPDATE quel_production_photo.production_status: completed
```

---

Last Updated: 2025-11-26
