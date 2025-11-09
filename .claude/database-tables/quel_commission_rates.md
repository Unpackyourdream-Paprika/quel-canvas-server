# quel_commission_rates

커미션 비율 설정 테이블 (동적 비율 관리)

## 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| rate_id | uuid | NO | gen_random_uuid() | 비율 ID (PK) |
| partner_id | uuid | YES | - | 파트너 ID (특정 파트너용, NULL이면 전체) |
| company_rate | numeric | YES | 80.00 | 회사 비율 (%) |
| partner_rate | numeric | YES | 20.00 | 파트너 전체 비율 (%) |
| effective_date | timestamp | NO | - | 적용 시작 날짜 |
| created_by | uuid | YES | - | 생성자 ID |
| created_at | timestamp | YES | now() | 생성 시간 |
| notes | text | YES | - | 메모 |

## 🎯 Purpose

**이 테이블의 역할:**
1. 결제 시 파트너 정산 비율을 동적으로 관리
2. 날짜별로 다른 비율 적용 가능 (버전 관리)
3. 특정 파트너에게 다른 비율 적용 가능
4. Company vs Partners 비율 설정

## 📝 Usage

### API Endpoints

#### Read Operations:
- `POST /api/stripe/webhook` - 정산 시 비율 조회

**File:** [src/app/api/stripe/webhook/route.ts](../../src/app/api/stripe/webhook/route.ts)

```typescript
// 최신 커미션 비율 조회 (effective_date 기준)
const { data: commissionRate } = await supabaseAdmin()
  .from("quel_commission_rates")
  .select("company_rate, partner_rate")
  .lte("effective_date", new Date().toISOString())
  .order("effective_date", { ascending: false })
  .limit(1)
  .single();

// company_rate: 80% (회사)
// partner_rate: 20% (파트너 전체)
```

## 💰 Commission Structure

### 기본 비율 (Default)
```
결제 금액: 5,000원
├─ Company: 80% → 4,000원
└─ Partners: 20% → 1,000원
   ├─ Tier 2: 60% of 1,000 = 600원
   └─ Tier 1: 40% of 1,000 = 400원
```

**Note:** Tier 1/Tier 2 비율은 하드코딩되어 있음 (webhook 코드 내)
- Tier 2: 60%
- Tier 1: 40%

향후 개선: `tier1_commission_rate`, `tier2_commission_rate` 컬럼 추가 가능

## 🔄 Data Flow

```
1. Customer completes payment
   ↓
2. Webhook: checkout.session.completed
   ↓
3. Query quel_commission_rates (latest by effective_date)
   ↓
4. Calculate:
   - Company share = subtotal * (company_rate / 100)
   - Partner total = subtotal * (partner_rate / 100)
   - Tier 2 share = partner_total * 0.6
   - Tier 1 share = partner_total * 0.4
   ↓
5. Execute Stripe Transfers
   ↓
6. Record in partner_settlements
```

## 🔍 Common Queries

### 현재 적용 중인 비율
```sql
SELECT *
FROM quel_commission_rates
WHERE effective_date <= NOW()
  AND partner_id IS NULL  -- 전체 적용 비율
ORDER BY effective_date DESC
LIMIT 1;
```

### 특정 날짜의 비율
```sql
SELECT *
FROM quel_commission_rates
WHERE effective_date <= '2025-01-01'
ORDER BY effective_date DESC
LIMIT 1;
```

### 비율 변경 히스토리
```sql
SELECT
  effective_date,
  company_rate,
  partner_rate,
  notes
FROM quel_commission_rates
WHERE partner_id IS NULL
ORDER BY effective_date DESC;
```

### 특정 파트너 전용 비율
```sql
SELECT *
FROM quel_commission_rates
WHERE partner_id = 'xxx'
  AND effective_date <= NOW()
ORDER BY effective_date DESC
LIMIT 1;
```

## ⚠️ Important Notes

1. **Effective Date 기반:**
   - 과거 결제에 대한 정산도 당시 비율 적용
   - 미래 날짜로 비율 예약 가능

2. **Partner ID:**
   - `NULL`: 모든 파트너에게 적용되는 기본 비율
   - 특정 ID: 해당 파트너만 다른 비율 적용

3. **Version Control:**
   - 비율 변경 시 기존 레코드 수정 금지
   - 새로운 레코드 INSERT (히스토리 유지)

4. **Tier 비율:**
   - 현재 Tier 1/Tier 2 비율은 코드에 하드코딩
   - 향후 이 테이블에 컬럼 추가 고려

## 🚀 Future Enhancements

### 테이블 확장 (제안)
```sql
ALTER TABLE quel_commission_rates
ADD COLUMN tier1_commission_rate numeric DEFAULT 40.00;

ALTER TABLE quel_commission_rates
ADD COLUMN tier2_commission_rate numeric DEFAULT 60.00;
```

그러면 webhook 코드도 수정:
```typescript
const { data: commissionRate } = await supabaseAdmin()
  .from("quel_commission_rates")
  .select("company_rate, partner_rate, tier1_commission_rate, tier2_commission_rate")
  .lte("effective_date", new Date().toISOString())
  .order("effective_date", { ascending: false })
  .limit(1)
  .single();

const tier2Share = Math.round(totalPartnerShare * (commissionRate.tier2_commission_rate / 100));
const tier1Share = Math.round(totalPartnerShare * (commissionRate.tier1_commission_rate / 100));
```

## 📊 Sample Data

```sql
INSERT INTO quel_commission_rates
(company_rate, partner_rate, effective_date, notes)
VALUES
(80.00, 20.00, '2025-01-01', 'Initial launch rate'),
(75.00, 25.00, '2025-06-01', 'Increased partner rate for H2 2025');
```

---

Last Updated: 2025-11-05
