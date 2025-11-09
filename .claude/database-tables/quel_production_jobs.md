# quel_production_jobs

이미지 생성 작업 정보 테이블

## 📋 Key Columns

| Column | Type | Description |
|--------|------|-------------|
| job_id | uuid | 작업 ID (PK) |
| production_id | uuid | 프로덕션 ID (FK → quel_production_photo) |
| quel_member_id | uuid | 회원 ID (FK → quel_member) |
| job_type | varchar | 작업 타입 (single_batch/pipeline_stage) |
| job_status | varchar | 상태 (pending/processing/completed/failed) |
| total_images | integer | 생성할 이미지 수 |
| completed_images | integer | 완료된 이미지 수 |
| failed_images | integer | 실패한 이미지 수 |
| estimated_credits | integer | 예상 크레딧 |
| remaining_credits | integer | 남은 크레딧 |
| job_input_data | jsonb | 입력 데이터 (prompt, images 등) |
| quel_production_path | varchar | 프로덕션 경로 (fashion/beauty/eats/cinema/cartoon) |

## 📝 Usage

### API Endpoints

**File:** [src/app/api/jobs/create/route.ts](../../src/app/api/jobs/create/route.ts)

```typescript
// Job 생성
await supabase.from('quel_production_jobs').insert({
  production_id,
  quel_member_id,
  job_type,
  job_status: 'pending',
  total_images,
  completed_images: 0,
  failed_images: 0,
  job_input_data,
  estimated_credits,
  remaining_credits
});
```

**File:** [src/app/api/jobs/[jobId]/route.ts](../../src/app/api/jobs/[jobId]/route.ts)

```typescript
// Job 상태 업데이트
await supabase
  .from('quel_production_jobs')
  .update({ job_status: 'completed' })
  .eq('job_id', jobId);
```

## 🔄 Data Flow

```
1. User clicks GENERATE
   ↓
2. POST /api/jobs/create
   ↓
3. INSERT quel_production_jobs (status: pending)
   ↓
4. POST /api/jobs/enqueue (Redis queue)
   ↓
5. Worker processes job
   ↓
6. UPDATE job_status: processing
   ↓
7. Images generated
   ↓
8. UPDATE completed_images, job_status: completed
```

---

Last Updated: 2025-11-05
