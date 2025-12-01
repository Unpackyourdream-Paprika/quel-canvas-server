# 크레딧 차감 흐름 (Credit Deduction Flow)

## 📊 전체 흐름도

```
1. 프론트엔드 (Render 버튼 클릭)
   ↓
2. Job 생성 요청 (/api/jobs/create)
   ├─ 조직 멤버 여부 자동 조회
   ├─ 크레딧 계산 (quantity × IMAGE_PER_PRICE)
   ├─ 조직 크레딧 우선 확인
   │  ├─ 충분 → useOrgCredit = true
   │  └─ 부족 → 개인 크레딧 확인
   ├─ 개인 크레딧 확인 (조직 없거나 부족한 경우)
   │  ├─ 충분 → useOrgCredit = false
   │  └─ 부족 → 402 에러 (크레딧 부족)
   └─ Job 레코드 생성 (org_id 기록)
   ↓
3. Go Worker (이미지 생성 완료 후)
   ├─ Job 완료 콜백 → Node.js API
   ├─ /api/jobs/[jobId] PATCH (job_status: completed)
   └─ **실제 크레딧 차감 필요** ⚠️
   ↓
4. 크레딧 차감 실행
   ├─ Job의 org_id 확인
   │  ├─ org_id 있음 → 조직 크레딧 차감
   │  └─ org_id 없음 → 개인 크레딧 차감
   └─ 거래 내역 기록 (quel_credits)
```

---

## 🔄 단계별 상세 설명

### 1단계: Job 생성 시 크레딧 체크 (Node.js)

**파일**: `src/app/api/jobs/create/route.ts`

```typescript
// 1️⃣ 환경변수에서 가격 가져오기
const creditPerImage = parseInt(process.env.IMAGE_PER_PRICE || '20');
const estimated_credits = total_images * creditPerImage;

// 2️⃣ 조직 멤버 자동 조회
const { data: membership } = await supabase
  .from('quel_organization_member')
  .select('org_id, role, status')
  .eq('member_id', quel_member_id)
  .eq('status', 'active')
  .single();

let validatedOrgId = membership?.org_id || null;

// 3️⃣ 조직 크레딧 우선 확인
let useOrgCredit = false;
if (validatedOrgId) {
  const { data: orgData } = await supabase
    .from('quel_organization')
    .select('org_credit')
    .eq('org_id', validatedOrgId)
    .single();

  if (orgData.org_credit >= estimated_credits) {
    useOrgCredit = true; // ✅ 조직 크레딧 사용
  }
}

// 4️⃣ 개인 크레딧 확인 (조직 크레딧 부족 시)
if (!useOrgCredit) {
  const { data: member } = await supabase
    .from('quel_member')
    .select('quel_member_credit')
    .eq('quel_member_id', quel_member_id)
    .single();

  if (member.quel_member_credit < estimated_credits) {
    return NextResponse.json({ error: 'Insufficient credits' }, { status: 402 });
  }
}

// 5️⃣ Job 생성 (org_id 기록)
await supabase.from('quel_production_jobs').insert({
  // ... other fields
  org_id: useOrgCredit ? validatedOrgId : null, // ✅ 어떤 크레딧 사용할지 기록
  estimated_credits,
});
```

**로그 예시**:
```
🏢 자동 조회된 조직: { org_id: '4deb5088-...', role: 'owner' }
💰 크레딧 계산: 4장 × 20 = 80 크레딧 필요
🏢 조직 크레딧: 1000
✅ 조직 크레딧 사용 (충분함: 1000 >= 80)
```

---

### 2단계: Go Worker에서 이미지 생성

**Go Worker**가 Redis 큐에서 Job을 가져와 이미지 생성:

```go
// 1. Job 상태 업데이트 (processing)
// 2. 이미지 생성 (Replicate API 호출)
// 3. Supabase에 이미지 저장
// 4. Job 완료 콜백 → Node.js API
```

**콜백 요청**:
```http
PATCH /api/jobs/{jobId}
{
  "job_status": "completed",
  "completed_images": 4,
  "generated_attach_ids": [28693, 28694, 28695, 28696]
}
```

---

### 3단계: 실제 크레딧 차감 (⚠️ 구현 필요)

**파일**: `src/app/api/jobs/[jobId]/route.ts`

#### 현재 상태:
- Job 상태만 업데이트하고 **크레딧 차감 안 함**

#### 필요한 로직:

```typescript
// Job 완료 시 실행 (PATCH /api/jobs/{jobId})

// 1️⃣ Job 정보 조회
const { data: job } = await supabase
  .from('quel_production_jobs')
  .select('org_id, quel_member_id, estimated_credits, job_status')
  .eq('job_id', jobId)
  .single();

// 중복 차감 방지
if (job.job_status === 'completed') {
  return; // 이미 완료된 Job
}

// 2️⃣ 크레딧 차감 로직
if (job.org_id) {
  // 🏢 조직 크레딧 차감
  await supabase
    .from('quel_organization')
    .update({
      org_credit: supabase.raw(`org_credit - ${job.estimated_credits}`)
    })
    .eq('org_id', job.org_id);

  // 거래 내역 기록
  await supabase.from('quel_credits').insert({
    org_id: job.org_id,
    used_by_member_id: job.quel_member_id,
    transaction_type: 'DEDUCT',
    amount: -job.estimated_credits,
    description: `이미지 생성 (Job ${jobId})`,
  });

  console.log(`🏢 조직 크레딧 차감: ${job.estimated_credits}`);
} else {
  // 👤 개인 크레딧 차감
  await supabase
    .from('quel_member')
    .update({
      quel_member_credit: supabase.raw(`quel_member_credit - ${job.estimated_credits}`)
    })
    .eq('quel_member_id', job.quel_member_id);

  // 거래 내역 기록
  await supabase.from('quel_credits').insert({
    user_id: job.quel_member_id,
    transaction_type: 'DEDUCT',
    amount: -job.estimated_credits,
    description: `이미지 생성 (Job ${jobId})`,
  });

  console.log(`👤 개인 크레딧 차감: ${job.estimated_credits}`);
}

// 3️⃣ Job 상태 업데이트
await supabase
  .from('quel_production_jobs')
  .update({ job_status: 'completed' })
  .eq('job_id', jobId);
```

---

## 📋 데이터베이스 구조

### quel_production_jobs 테이블

| 컬럼 | 타입 | 설명 |
|------|------|------|
| job_id | uuid | Job 고유 ID |
| quel_member_id | uuid | 사용자 ID |
| org_id | uuid (nullable) | 조직 ID (조직 크레딧 사용 시) |
| estimated_credits | int | 필요 크레딧 (quantity × IMAGE_PER_PRICE) |
| job_status | text | 상태 (pending/processing/completed/failed) |

**org_id 판단 기준**:
- `org_id IS NOT NULL` → 조직 크레딧 차감
- `org_id IS NULL` → 개인 크레딧 차감

---

### quel_credits 테이블 (거래 내역)

| 컬럼 | 타입 | 설명 |
|------|------|------|
| user_id | uuid (nullable) | 개인 크레딧 거래 시 |
| org_id | uuid (nullable) | 조직 크레딧 거래 시 |
| used_by_member_id | uuid (nullable) | 조직 크레딧을 사용한 실제 멤버 |
| transaction_type | text | DEDUCT/PURCHASE/REFUND |
| amount | int | 금액 (차감 시 음수) |
| description | text | 거래 사유 |

**거래 유형**:
```typescript
// 개인 크레딧 차감
{
  user_id: "404f00f0-...",
  org_id: null,
  used_by_member_id: null,
  transaction_type: "DEDUCT",
  amount: -80
}

// 조직 크레딧 차감
{
  user_id: null,
  org_id: "4deb5088-...",
  used_by_member_id: "404f00f0-...", // 실제 사용자
  transaction_type: "DEDUCT",
  amount: -80
}
```

---

## 🔧 환경 변수

**파일**: `.env` 또는 `.env.local`

```bash
# 이미지당 크레딧 가격
IMAGE_PER_PRICE=20
```

**사용 위치**:
- `src/app/api/jobs/create/route.ts:214`

```typescript
const creditPerImage = parseInt(process.env.IMAGE_PER_PRICE || '20');
```

---

## ⚠️ 중요 사항

### 1. 중복 차감 방지
```typescript
// Job 완료 시 상태 체크
if (job.job_status === 'completed') {
  console.warn('⚠️ Already completed, skipping credit deduction');
  return;
}
```

### 2. Atomic 업데이트 사용
```typescript
// ❌ 잘못된 방법 (Race Condition 가능)
const current = member.quel_member_credit;
await update({ quel_member_credit: current - 80 });

// ✅ 올바른 방법 (Atomic)
await update({
  quel_member_credit: supabase.raw('quel_member_credit - 80')
});
```

### 3. 트랜잭션 필요
```typescript
// 크레딧 차감 + 거래 내역 기록은 하나의 트랜잭션으로 처리
// 실패 시 rollback 필요
```

---

## 📊 크레딧 흐름 요약

| 단계 | 위치 | 동작 | 크레딧 변화 |
|------|------|------|------------|
| **1. Job 생성** | `/api/jobs/create` | 크레딧 체크 (사전 검증) | 변화 없음 |
| **2. 이미지 생성** | Go Worker | 이미지 생성 | 변화 없음 |
| **3. Job 완료** | `/api/jobs/[jobId]` | **실제 크레딧 차감** ⚠️ | **-80** |

---

## 🚀 다음 구현 필요 사항

### 1. `/api/jobs/[jobId]/route.ts` 수정
- [ ] Job 완료 시 크레딧 차감 로직 추가
- [ ] org_id 기반 조직/개인 크레딧 선택
- [ ] 거래 내역 기록 (quel_credits)
- [ ] 중복 차감 방지

### 2. Go Worker 수정 (선택)
- [ ] Job 완료 콜백에 크레딧 정보 포함
- [ ] 실패 시 크레딧 환불 로직

### 3. 프론트엔드 수정 (선택)
- [ ] 크레딧 부족 시 에러 메시지 표시
- [ ] 조직/개인 크레딧 잔액 표시

---

Last Updated: 2025-12-01
