# quel_production_photo

이미지 프로덕션 정보 테이블

## 📋 Schema

```sql
create table public.quel_production_photo (
  production_id uuid not null default gen_random_uuid (),
  created_at timestamp with time zone not null default now(),
  quel_member_id uuid null,
  production_name character varying(255) null,
  production_description text null,
  production_status public.production_status_enum null default 'pending'::production_status_enum,
  pipeline_type character varying(50) null,
  stage_count integer null default 1,
  total_quantity integer null,
  camera_angle character varying(50) null,
  shot_type character varying(50) null,
  prompt_text text null,
  generated_image_count integer null default 0,
  attach_ids jsonb null,
  processing_duration_seconds integer null,
  input_images_count integer null,
  workflow_data jsonb null,
  quel_production_path character varying(50) null,
  constraint quel_production_photo_pkey primary key (production_id)
) TABLESPACE pg_default;

create index IF not exists idx_quel_production_photo_member_status on public.quel_production_photo using btree (
  quel_member_id,
  production_status,
  created_at desc
) TABLESPACE pg_default;
```

## 📋 Key Columns

| Column | Type | Description |
|--------|------|-------------|
| production_id | uuid | 프로덕션 ID (PK) |
| created_at | timestamp | 생성 시간 |
| quel_member_id | uuid | 회원 ID (FK → quel_member) |
| production_name | varchar(255) | 프로덕션 이름 |
| production_description | text | 프로덕션 설명 |
| production_status | enum | 상태 (pending/processing/completed/failed) |
| pipeline_type | varchar(50) | 파이프라인 타입 |
| stage_count | integer | 스테이지 수 (기본값: 1) |
| total_quantity | integer | 총 이미지 수량 |
| camera_angle | varchar(50) | 카메라 앵글 |
| shot_type | varchar(50) | 샷 타입 |
| prompt_text | text | 프롬프트 텍스트 |
| generated_image_count | integer | 생성된 이미지 수 (기본값: 0) |
| attach_ids | jsonb | 첨부 파일 ID 목록 |
| processing_duration_seconds | integer | 처리 시간 (초) |
| input_images_count | integer | 입력 이미지 수 |
| workflow_data | jsonb | 워크플로우 데이터 |
| quel_production_path | varchar(50) | 프로덕션 경로 (fashion/beauty/eats/cinema/cartoon) |

## 📝 Usage

### API Endpoints

**File:** [src/app/api/jobs/create/route.ts](../../src/app/api/jobs/create/route.ts)

```typescript
// Production 생성
await supabase.from('quel_production_photo').insert({
  quel_member_id,
  production_name,
  production_status: 'pending',
  workflow_data,
  quel_production_path: 'fashion' // or beauty, eats, cinema, cartoon
});
```

## 🔄 Data Flow

```
1. User clicks GENERATE on /visual/{category}
   ↓
2. POST /api/jobs/create
   ↓
3. INSERT quel_production_photo (status: pending, path: category)
   ↓
4. INSERT quel_production_jobs (FK: production_id, path: category)
   ↓
5. Worker processes based on quel_production_path
   ↓
6. UPDATE production_status: completed
```

---

Last Updated: 2025-11-09
