# quel_partners

파트너 정보 테이블

## 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| partner_id | uuid | NO | gen_random_uuid() | 파트너 ID (PK) |
| quel_member_id | uuid | YES | - | 연결된 회원 ID (FK → quel_member) |
| partner_name | varchar | YES | - | 파트너 이름 |
| partner_email | varchar | YES | - | 파트너 이메일 |
| partner_country | varchar | YES | - | 파트너 국가 (KR/JP 등) |
| partner_level | integer | YES | - | 파트너 레벨 (1 or 2) |
| referral_partner_id | uuid | YES | - | 추천한 파트너 ID (Tier 1 ID) |
| our_company | boolean | YES | false | 우리 회사 마스터 계정 여부 |
| stripe_account_id | text | YES | - | Stripe Connected Account ID |
| stripe_onboarding_completed | boolean | YES | false | Stripe 기본 온보딩 완료 |
| stripe_final_onboarding_completed | boolean | YES | false | Stripe 최종 온보딩 완료 |
| created_at | timestamp | YES | now() | 생성 시간 |
| updated_at | timestamp | YES | now() | 업데이트 시간 |

## 🔗 Relationships

**Foreign Keys:**
- `quel_member_id` → `quel_member.quel_member_id`
- `referral_partner_id` → `quel_partners.partner_id` (self-reference)

**Referenced By:**
- `quel_service_referral_code.tier2_partner_id`
- `quel_member.tier2_partner_id`
- `quel_partner_customers.tier1_partner_id`
- `quel_partner_customers.tier2_partner_id`
- `partner_settlements.partner_id`

## 🎯 Purpose

**이 테이블의 역할:**
1. 파트너 기본 정보 관리
2. 2-Tier 구조 (Tier 1 ← Tier 2) 관계 저장
3. Stripe Connected Account 연동 정보
4. 정산 가능 여부 확인

## 📝 Usage

### API Endpoints

#### Read Operations:
- `POST /api/register-service-code` - Tier 1 파트너 ID 조회

**File:** [src/app/api/register-service-code/route.ts](../../src/app/api/register-service-code/route.ts)

```typescript
// Line 56-68: Tier 2 파트너 조회
const { data: tier2Partner, error: tier2Error } = await supabaseAdmin()
  .from("quel_partners")
  .select("referral_partner_id")
  .eq("partner_id", serviceCodeData.tier2_partner_id)
  .single();

const tier1PartnerId = tier2Partner?.referral_partner_id || null;
```

- `POST /api/stripe/webhook` - 정산 대상 파트너 조회

**File:** [src/app/api/stripe/webhook/route.ts](../../src/app/api/stripe/webhook/route.ts)

```typescript
// Tier 2 파트너 정보 조회
const { data: tier2Partner } = await supabaseAdmin()
  .from("quel_partners")
  .select(`
    partner_id,
    partner_name,
    partner_country,
    stripe_account_id,
    stripe_onboarding_completed,
    stripe_final_onboarding_completed,
    referral_partner_id
  `)
  .eq("partner_id", tier2PartnerId)
  .single();

// 자동 정산 가능 여부 확인
if (tier2Partner.stripe_onboarding_completed &&
    tier2Partner.stripe_final_onboarding_completed &&
    tier2Partner.partner_country === 'KR') {
  // Stripe Transfer 실행
}

// Tier 1 파트너 조회
if (tier2Partner.referral_partner_id) {
  const { data: tier1Partner } = await supabaseAdmin()
    .from("quel_partners")
    .select("*")
    .eq("partner_id", tier2Partner.referral_partner_id)
    .single();
}
```

## 🔍 Common Queries

### Tier 1 파트너 목록
```sql
SELECT * FROM quel_partners
WHERE partner_level = 1
ORDER BY created_at DESC;
```

### Tier 2 파트너와 상위 Tier 1
```sql
SELECT
  t2.*,
  t1.partner_name as tier1_name,
  t1.partner_email as tier1_email
FROM quel_partners t2
LEFT JOIN quel_partners t1 ON t2.referral_partner_id = t1.partner_id
WHERE t2.partner_level = 2
ORDER BY t2.created_at DESC;
```

### Stripe 온보딩 완료된 파트너
```sql
SELECT * FROM quel_partners
WHERE stripe_onboarding_completed = true
  AND stripe_final_onboarding_completed = true
ORDER BY updated_at DESC;
```

### 국가별 파트너 수
```sql
SELECT
  partner_country,
  COUNT(*) as partner_count
FROM quel_partners
GROUP BY partner_country
ORDER BY partner_count DESC;
```

## 🔄 Partner Hierarchy

```
Tier 1 Partner (독립 파트너)
├─ referral_partner_id: NULL
├─ partner_level: 1
└─ Can recruit Tier 2 partners

Tier 2 Partner (하위 파트너)
├─ referral_partner_id: Tier 1의 partner_id
├─ partner_level: 2
└─ Can create service codes
```

## ⚠️ Important Notes

1. **Partner Levels:**
   - Tier 1: 독립 파트너, `referral_partner_id = NULL`
   - Tier 2: Tier 1이 추천한 파트너, `referral_partner_id = Tier 1 ID`

2. **our_company (우리 회사 마스터 계정):**
   - `our_company = true`: Platform이 직접 관리하는 Tier 1 마스터 계정
     - Stripe 계정 없음 (`stripe_account_id = NULL`)
     - Tier 2 파트너들에게 100% 분배
     - `partner_settlements`에 Tier 1 기록 안함 (Tier 2만 기록)
     - 예: 한국 시장 마스터 계정
   - `our_company = false`: 일반 외부 파트너
     - Stripe Connected Account 필요
     - Destination Charges로 자동 정산
     - `partner_settlements`에 Tier 1 + Tier 2 모두 기록
     - 예: 일본 파트너들

3. **Stripe Onboarding:**
   - `stripe_onboarding_completed`: 기본 온보딩 완료
   - `stripe_final_onboarding_completed`: 최종 온보딩 완료
   - 둘 다 `true`여야 자동 정산 가능
   - `our_company = true`인 경우 Stripe 온보딩 불필요

4. **Country Code:**
   - KR: 한국 (our_company = true로 설정 권장)
   - JP: 일본 (Destination Charges 사용)

5. **Stripe Account ID:**
   - Stripe Connect Custom Account ID
   - 형식: `acct_xxxxxxxxxxxxx`
   - `our_company = true`인 경우 NULL
   - 정산 시 `destination`으로 사용

## 📊 Statistics

### 파트너 구조 통계
```sql
SELECT
  COUNT(CASE WHEN partner_level = 1 THEN 1 END) as tier1_count,
  COUNT(CASE WHEN partner_level = 2 THEN 1 END) as tier2_count,
  COUNT(*) as total_partners
FROM quel_partners;
```

### Tier 1별 하위 Tier 2 수
```sql
SELECT
  t1.partner_name as tier1_name,
  COUNT(t2.partner_id) as tier2_count
FROM quel_partners t1
LEFT JOIN quel_partners t2 ON t1.partner_id = t2.referral_partner_id
WHERE t1.partner_level = 1
GROUP BY t1.partner_id, t1.partner_name
ORDER BY tier2_count DESC;
```

### Stripe 온보딩 현황
```sql
SELECT
  COUNT(*) as total,
  SUM(CASE WHEN stripe_onboarding_completed THEN 1 ELSE 0 END) as basic_complete,
  SUM(CASE WHEN stripe_final_onboarding_completed THEN 1 ELSE 0 END) as final_complete,
  SUM(CASE WHEN stripe_onboarding_completed AND stripe_final_onboarding_completed THEN 1 ELSE 0 END) as fully_ready
FROM quel_partners;
```

## 🐛 Troubleshooting

### 정산 불가능한 파트너 찾기
```sql
SELECT
  p.*,
  COUNT(ps.settlement_id) as pending_settlements
FROM quel_partners p
LEFT JOIN partner_settlements ps ON p.partner_id = ps.partner_id
  AND ps.transfer_status = 'manual_required'
WHERE (p.stripe_onboarding_completed = false
   OR p.stripe_final_onboarding_completed = false
   OR p.partner_country = 'JP')
GROUP BY p.partner_id
ORDER BY pending_settlements DESC;
```

### 고아 Tier 2 파트너 (Tier 1 삭제됨)
```sql
SELECT t2.*
FROM quel_partners t2
LEFT JOIN quel_partners t1 ON t2.referral_partner_id = t1.partner_id
WHERE t2.partner_level = 2
  AND t2.referral_partner_id IS NOT NULL
  AND t1.partner_id IS NULL;
```

---

Last Updated: 2025-11-05
