# Database Tables Reference

Supabase 테이블 전체 구조 및 관계 정리

---

## 📊 All Tables Overview

### Member & Auth
```
quel_member
├─ quel_member_id (PK, uuid)
├─ quel_member_email (varchar)
├─ quel_member_name (varchar)
├─ quel_member_credit (integer, default: 0)
├─ referral_service_code (varchar)
├─ service_code_id (FK → quel_service_referral_code)
├─ tier2_partner_id (FK → quel_partners)
├─ referral_code_registered_at (timestamp)
├─ created_at (timestamp)
└─ updated_at (timestamp)
```

### Partner System
```
quel_partners
├─ partner_id (PK, uuid)
├─ partner_email (varchar(255), NOT NULL)
├─ partner_name (varchar(255), NOT NULL)
├─ partner_company (varchar(255))
├─ partner_phone (varchar(50))
├─ partner_status (varchar(50), default: 'pending')
├─ credit_code (varchar(50), NOT NULL)
├─ partner_level (integer, default: 1) - 1 or 2
├─ referrer_partner_id (FK → quel_partners, Tier 1 ID)
├─ commission_rate (numeric, default: 0.00)
├─ stripe_account_id (varchar(255))
├─ stripe_onboarding_completed (boolean, default: false)
├─ stripe_final_onboarding_completed (boolean, default: false)
├─ stripe_dashboard_url (text)
├─ created_at (timestamp)
└─ updated_at (timestamp)
```

```
quel_service_referral_code
├─ service_code_id (PK, uuid)
├─ service_code (varchar, UNIQUE)
├─ tier2_partner_id (FK → quel_partners)
├─ is_active (boolean, default: true)
├─ total_customers (integer, default: 0)
├─ created_at (timestamp)
└─ updated_at (timestamp)
```

```
quel_partner_customers
├─ id (PK, uuid)
├─ customer_id (FK → quel_member)
├─ tier1_partner_id (FK → quel_partners, nullable)
├─ tier2_partner_id (FK → quel_partners)
├─ service_code_id (FK → quel_service_referral_code)
├─ credit_code_used (varchar(50))
├─ description (text)
├─ registered_at (timestamp, default: now())
└─ status (varchar(50), default: 'active')
```

```
partner_settlements
├─ settlement_id (PK, uuid)
├─ payment_id (text) - Stripe Payment Intent ID
├─ partner_id (FK → quel_partners)
├─ partner_level (integer) - 1 or 2
├─ partner_name (text) - 스냅샷
├─ subtotal (integer) - 결제 총액 (세금 제외)
├─ partner_share (integer) - 파트너 받을 금액
├─ currency (text) - JPY, KRW 등
├─ stripe_transfer_id (text) - Destination Charges는 NULL
├─ stripe_account_id (text) - Connected Account ID
├─ transfer_status (text) - success/manual_required
├─ customer_id (FK → quel_member)
├─ service_code (text) - 스냅샷
└─ created_at (timestamp)
```

**정산 방식 (2025-01-07):**
- **Tier 1**: Destination Charges로 20% 자동 이체 (status: success)
- **Tier 2**: DB 기록만, Tier 1이 수동 분배 (status: manual_required)

### Commission
```
quel_commission_rates
├─ rate_id (PK, uuid)
├─ partner_id (uuid, nullable - NULL for global)
├─ company_rate (numeric, default: 80.00)
├─ partner_rate (numeric, default: 20.00)
├─ effective_date (timestamp, NOT NULL)
├─ created_by (uuid)
├─ created_at (timestamp)
└─ notes (text)
```

### Payment
```
payments
├─ id (PK, uuid)
├─ user_id (uuid, NOT NULL)
├─ buy_credit (bigint)
├─ price (bigint)
├─ currency (text)
├─ status (text)
├─ stripe_account_id (text)
├─ created_at (timestamp)
├─ original_credits (integer)
├─ bonus_credits (integer)
├─ payment_time (timestamp)
├─ subtotal (bigint)
├─ tax_rate (numeric)
├─ tax_amount (bigint)
└─ total_amount (bigint)
```

```
quel_credits (Credit Transaction History)
├─ id (PK, uuid)
├─ user_id (uuid, NOT NULL)
├─ transaction_type (varchar(20), NOT NULL) - purchase/deduction/refund
├─ amount (integer, NOT NULL) - + or -
├─ balance_after (integer, NOT NULL)
├─ description (text)
├─ attach_idx (bigint)
├─ created_at (timestamp, NOT NULL)
└─ production_idx (uuid)
```

```
plans
├─ id (PK, uuid)
├─ name (text)
├─ price (bigint)
├─ price_id (text) - Stripe Price ID
├─ rank (bigint)
├─ credits (bigint)
├─ created_at (timestamp)
├─ location (text)
├─ popular (boolean)
├─ discount (text)
├─ type (text)
├─ features (json[])
├─ subtitle (varchar)
├─ quel_member_idx (varchar)
└─ country (varchar) - KR/JP
```

### Production & Jobs
```
quel_production_photo
├─ production_id (PK, uuid)
├─ created_at (timestamp)
├─ quel_member_id (FK → quel_member)
├─ production_name (varchar(255))
├─ production_description (text)
├─ production_status (enum: pending/processing/completed)
├─ pipeline_type (varchar(50))
├─ stage_count (integer, default: 1)
├─ total_quantity (integer)
├─ camera_angle (varchar(50))
├─ shot_type (varchar(50))
├─ prompt_text (text)
├─ generated_image_count (integer, default: 0)
├─ attach_ids (jsonb)
├─ processing_duration_seconds (integer)
├─ input_images_count (integer)
├─ workflow_data (jsonb)
└─ quel_production_path (varchar(50)) - fashion/beauty/eats/cinema/cartoon
```

```
quel_production_jobs
├─ job_id (PK, uuid)
├─ production_id (FK → quel_production_photo)
├─ quel_member_id (FK → quel_member)
├─ job_type (varchar) - single_batch/pipeline_stage
├─ stage_index (integer, nullable)
├─ stage_name (varchar, nullable)
├─ batch_index (integer, nullable)
├─ job_status (varchar) - pending/processing/completed/failed
├─ total_images (integer)
├─ completed_images (integer, default: 0)
├─ failed_images (integer, default: 0)
├─ job_input_data (jsonb)
├─ retry_count (integer, default: 0)
├─ estimated_credits (integer)
├─ remaining_credits (integer)
├─ created_at (timestamp)
├─ updated_at (timestamp)
└─ quel_production_path (varchar(50)) - fashion/beauty/eats/cinema/cartoon
```

---

## 🔗 Key Relationships

### Partner Hierarchy
```
Tier 1 Partner (독립)
└─ referrer_partner_id: NULL

Tier 2 Partner (하위)
└─ referrer_partner_id: Tier 1 ID
   └─ Creates service codes
      └─ Customers register with code
```

### Service Code Registration Flow
```
Customer enters code
↓
quel_service_referral_code (verify)
↓
quel_partners (get tier1/tier2)
↓
quel_member (update referral info)
↓
quel_partner_customers (insert relationship)
```

### Payment & Settlement Flow (Updated 2025-01-07)
```
Customer purchases credits
↓
Stripe Checkout Session
↓
Destination Charges 설정:
  - Company: 80% → Platform Balance
  - Tier 1: 20% → Connected Account (자동 이체)
↓
Webhook: checkout.session.completed
↓
quel_member (update credits)
↓
quel_commission_rates (get rates)
↓
Calculate shares:
  Company: 80%
  Tier 1: 20% (전체)
    - Tier 1 keeps: 40% of 20% = 8%
    - Tier 2 receives: 60% of 20% = 12% (수동 분배)
↓
partner_settlements (insert × 2):
  - Tier 1: transfer_status = 'success' (Destination Charges 완료)
  - Tier 2: transfer_status = 'manual_required' (DB 기록만)
↓
Tier 1이 나중에 Tier 2에게 수동 송금 (Stripe 밖에서)
```

### Image Generation Flow
```
User clicks GENERATE
↓
quel_production_photo (create)
↓
quel_production_jobs (create)
↓
Redis Queue (enqueue)
↓
Worker processes job
↓
quel_production_image (insert results)
↓
quel_member (deduct credits)
```

---

## 📁 Detailed Documentation

- [quel_member](./quel_member.md) - 회원 정보
- [quel_partners](./quel_partners.md) - 파트너 정보
- [quel_service_referral_code](./quel_service_referral_code.md) - 서비스 코드
- [quel_partner_customers](./quel_partner_customers.md) - 고객-파트너 관계
- [partner_settlements](./partner_settlements.md) - 정산 내역
- [quel_commission_rates](./quel_commission_rates.md) - 커미션 비율
- [quel_payment](./quel_payment.md) - 결제 정보 (payments 테이블)
- [quel_production_jobs](./quel_production_jobs.md) - 작업 정보

---

## 🎯 API Usage Summary

### Service Code APIs
- `POST /api/verify-service-code` → `quel_service_referral_code`
- `POST /api/register-service-code` → `quel_member`, `quel_partners`, `quel_partner_customers`, `quel_service_referral_code`

### Payment APIs
- `POST /api/stripe/checkout` → `plans`, `payments`
- `POST /api/stripe/webhook` → `payments`, `quel_member`, `quel_commission_rates`, `partner_settlements`, `quel_partners`

### Production APIs
- `POST /api/production` → `quel_production_photo`
- `POST /api/jobs/create` → `quel_production_jobs`
- `GET /api/get-production/[id]` → `quel_production_photo`, `quel_production_jobs`

### Member APIs
- `GET /api/user/me` → `quel_member`
- `GET /api/auth/me` → `quel_member`

---

---

## 🔄 Recent Updates

### 2025-01-09: Production Path 컬럼 추가
- `quel_production_photo` 테이블에 `quel_production_path` 컬럼 추가
- `quel_production_jobs` 테이블에 `quel_production_path` 컬럼 추가
- 카테고리별 워크플로우 분리: fashion/beauty/eats/cinema/cartoon
- goserver modules 폴더 구조화 준비

**마이그레이션 SQL:**
```sql
-- 1. quel_production_photo 테이블에 quel_production_path 컬럼 추가
ALTER TABLE public.quel_production_photo
ADD COLUMN quel_production_path VARCHAR(50);

-- 2. quel_production_jobs 테이블에 quel_production_path 컬럼 추가
ALTER TABLE public.quel_production_jobs
ADD COLUMN quel_production_path VARCHAR(50);

-- 3. (선택사항) 성능 최적화를 위한 인덱스 추가
CREATE INDEX IF NOT EXISTS idx_quel_production_photo_path
ON public.quel_production_photo(quel_production_path);

CREATE INDEX IF NOT EXISTS idx_quel_production_jobs_path
ON public.quel_production_jobs(quel_production_path);
```

### 2025-01-07: Stripe Destination Charges 구현
- `partner_settlements` 테이블에 `currency` 컬럼 추가
- `payment_id` 타입 변경: uuid → text (Stripe Payment Intent ID)
- `transfer_status` 값 변경: `failed` 제거, `success`/`manual_required`만 사용
- Tier 1 정산: Destination Charges로 자동 이체
- Tier 2 정산: DB 기록만, 수동 분배

**주요 변경사항:**
- Transfer API 방식 → Destination Charges 방식
- Tier 1이 20% 전체 수령 후 Tier 2에게 수동 분배
- Multi-currency 지원 (JPY, KRW)

---

Last Updated: 2025-01-09
