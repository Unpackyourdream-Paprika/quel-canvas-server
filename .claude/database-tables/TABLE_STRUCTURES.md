# Database Table Structures

QUELSUITE 데이터베이스 테이블 구조 참조 문서

---

## Member & Auth

### quel_member
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

---

## Partner System

### quel_partners
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

### quel_service_referral_code
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

### quel_partner_customers
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

### partner_settlements
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

---

## Commission

### quel_commission_rates
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

---

## Payment & Credits

### payments
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

### quel_credits
```
quel_credits
├─ id (PK, uuid)
├─ user_id (uuid, NOT NULL)
├─ transaction_type (varchar(20), NOT NULL) - purchase/deduction/refund
├─ amount (integer, NOT NULL)
├─ balance_after (integer, NOT NULL)
├─ description (text)
├─ attach_idx (bigint)
├─ created_at (timestamp, NOT NULL)
└─ production_idx (uuid)
```

### plans
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

---

## Production & Jobs

### quel_production_photo
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

### quel_production_jobs
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

## Table Relationships

```
quel_member
├─ service_code_id → quel_service_referral_code.service_code_id
├─ tier2_partner_id → quel_partners.partner_id
└─ quel_member_id ← quel_partner_customers.customer_id
                  ← quel_production_photo.quel_member_id
                  ← quel_production_jobs.quel_member_id
                  ← payments.user_id
                  ← quel_credits.user_id

quel_partners
├─ referrer_partner_id → quel_partners.partner_id (self-reference)
└─ partner_id ← quel_service_referral_code.tier2_partner_id
              ← quel_partner_customers.tier1_partner_id
              ← quel_partner_customers.tier2_partner_id
              ← partner_settlements.partner_id

quel_service_referral_code
└─ service_code_id ← quel_member.service_code_id
                   ← quel_partner_customers.service_code_id

payments
└─ id ← partner_settlements.payment_id

quel_production_photo
└─ production_id ← quel_production_jobs.production_id
```

---

## Key Notes

### Partner Hierarchy
- **Tier 1**: `referrer_partner_id = NULL` (독립 파트너)
- **Tier 2**: `referrer_partner_id = Tier 1 ID` (하위 파트너)

### Commission Structure (Updated 2025-01-07)
- Company: 80% (default)
- Partners: 20% (default)
  - **Tier 1**: 20% 전체를 Stripe Destination Charges로 수령
  - **Tier 2**: Tier 1이 받은 20% 중 60% (= 12%)를 수동 분배
  - **Tier 1 최종**: 20% 중 40% (= 8%) 보유

**정산 방식:**
- Tier 1: Destination Charges로 자동 이체
- Tier 2: Stripe 밖에서 Tier 1이 수동 송금

### Credit Flow
1. Purchase → `payments` (insert)
2. Add credits → `quel_member` (update credit)
3. Log transaction → `quel_credits` (insert, type: purchase)
4. Use credits → `quel_credits` (insert, type: deduction)

### Settlement Flow (Updated 2025-01-07)
1. Checkout Session 생성 → Destination Charges 설정
2. Payment complete → Webhook 이벤트
3. Get rates → `quel_commission_rates` (query)
4. Calculate shares:
   - Tier 1: 20% (Destination Charges로 자동 이체)
   - Tier 2: Tier 1의 60% (DB 기록만)
5. Insert → `partner_settlements` (× 2):
   - Tier 1: status = 'success'
   - Tier 2: status = 'manual_required'
6. Tier 1이 나중에 Tier 2에게 수동 송금

---

## 🔄 Recent Updates

### 2025-01-07: Stripe Destination Charges 구현
- `partner_settlements` 테이블 스키마 변경
- `currency` 컬럼 추가 (JPY, KRW 지원)
- `payment_id` 타입 변경: uuid → text
- `transfer_status` 값 단순화: success/manual_required
- Transfer API → Destination Charges 방식 전환

**비즈니스 로직 변경:**
- Tier 1: 20% 전체를 Destination Charges로 자동 수령
- Tier 2: DB에 기록만, Tier 1이 수동 분배

---

Last Updated: 2025-01-07
