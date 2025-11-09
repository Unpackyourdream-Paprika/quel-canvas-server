# quel_service_referral_code

서비스 추천 코드 테이블 (파트너가 생성한 코드)

## 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| service_code_id | uuid | NO | gen_random_uuid() | 코드 ID (PK) |
| service_code | varchar | NO | - | 추천 코드 (UNIQUE) |
| tier2_partner_id | uuid | NO | - | Tier 2 파트너 ID (FK → quel_partners) |
| is_active | boolean | YES | true | 활성화 여부 |
| total_customers | integer | YES | 0 | 총 등록 고객 수 |
| created_at | timestamp | YES | now() | 생성 시간 |
| updated_at | timestamp | YES | now() | 업데이트 시간 |

## 🔗 Relationships

**Foreign Keys:**
- `tier2_partner_id` → `quel_partners.partner_id`

**Referenced By:**
- `quel_member.service_code_id`
- `quel_partner_customers.service_code_id`

## 🎯 Purpose

**이 테이블의 역할:**
1. 파트너가 발급한 추천 코드 관리
2. 고객이 입력한 코드 검증
3. 코드당 등록 고객 수 집계
4. 파트너 성과 추적

## 📝 Usage

### API Endpoints

#### Read Operations:
- `POST /api/verify-service-code` - 코드 검증

**File:** [src/app/api/verify-service-code/route.ts](../../src/app/api/verify-service-code/route.ts)

```typescript
// Line 32-37
const { data: serviceCodeData, error: codeError } = await supabaseAdmin()
  .from("quel_service_referral_code")
  .select("service_code_id, tier2_partner_id, is_active")
  .eq("service_code", normalizedCode)
  .eq("is_active", true)
  .single();

if (codeError || !serviceCodeData) {
  return NextResponse.json(
    { valid: false, error: "Invalid or inactive service code" },
    { status: 404 }
  );
}
```

#### Write Operations:
- `POST /api/register-service-code` - 코드 등록 후 total_customers +1

**File:** [src/app/api/register-service-code/route.ts](../../src/app/api/register-service-code/route.ts)

```typescript
// Line 39-54: 코드 조회
const { data: serviceCodeData, error: codeError } = await supabaseAdmin()
  .from("quel_service_referral_code")
  .select("service_code_id, tier2_partner_id, is_active, total_customers")
  .eq("service_code", normalizedCode)
  .eq("is_active", true)
  .single();

// Line 89-95: total_customers 증가
const { error: incrementError } = await supabaseAdmin()
  .from("quel_service_referral_code")
  .update({
    total_customers: serviceCodeData.total_customers + 1,
  })
  .eq("service_code_id", serviceCodeData.service_code_id);
```

## 🔍 Common Queries

### 활성 코드 목록
```sql
SELECT
  src.*,
  p.partner_name,
  p.partner_email
FROM quel_service_referral_code src
JOIN quel_partners p ON src.tier2_partner_id = p.partner_id
WHERE src.is_active = true
ORDER BY src.total_customers DESC;
```

### 파트너의 모든 코드
```sql
SELECT * FROM quel_service_referral_code
WHERE tier2_partner_id = 'xxx'
ORDER BY created_at DESC;
```

### 가장 많이 사용된 코드 TOP 10
```sql
SELECT
  src.service_code,
  src.total_customers,
  p.partner_name
FROM quel_service_referral_code src
JOIN quel_partners p ON src.tier2_partner_id = p.partner_id
WHERE src.is_active = true
ORDER BY src.total_customers DESC
LIMIT 10;
```

### 사용되지 않은 코드
```sql
SELECT * FROM quel_service_referral_code
WHERE total_customers = 0
  AND is_active = true
ORDER BY created_at ASC;
```

## 🔄 Data Flow

```
1. Partner creates service code (파트너 대시보드)
   ↓
   INSERT INTO quel_service_referral_code
   ↓
2. Customer enters code in UI
   ↓
   POST /api/verify-service-code (validate)
   ↓
3. Customer confirms registration
   ↓
   POST /api/register-service-code
   ↓
   UPDATE quel_service_referral_code.total_customers +1
```

## ⚠️ Important Notes

1. **Code Format:**
   - 입력된 코드는 자동으로 `trim().toUpperCase()` 처리됨
   - 예: "test code" → "TEST CODE"

2. **Uniqueness:**
   - `service_code` 컬럼에 UNIQUE 제약조건 있어야 함
   - 중복 코드 생성 방지

3. **Deactivation:**
   - `is_active = false`로 설정하면 검증 실패
   - 코드는 삭제하지 않고 비활성화만 함 (히스토리 유지)

4. **total_customers:**
   - 실시간 카운터
   - `quel_partner_customers` 테이블과 동기화 필요
   - 증가만 가능 (감소 안 함)

## 📊 Statistics

### 전체 통계
```sql
SELECT
  COUNT(*) as total_codes,
  COUNT(CASE WHEN is_active THEN 1 END) as active_codes,
  SUM(total_customers) as total_customers,
  AVG(total_customers) as avg_customers_per_code
FROM quel_service_referral_code;
```

### 파트너별 코드 성과
```sql
SELECT
  p.partner_name,
  COUNT(src.service_code_id) as code_count,
  SUM(src.total_customers) as total_customers
FROM quel_service_referral_code src
JOIN quel_partners p ON src.tier2_partner_id = p.partner_id
WHERE src.is_active = true
GROUP BY p.partner_id, p.partner_name
ORDER BY total_customers DESC;
```

## 🐛 Troubleshooting

### total_customers 불일치 확인
```sql
-- 실제 고객 수와 total_customers 비교
SELECT
  src.service_code,
  src.total_customers as recorded_count,
  COUNT(pc.customer_id) as actual_count
FROM quel_service_referral_code src
LEFT JOIN quel_partner_customers pc ON src.service_code_id = pc.service_code_id
GROUP BY src.service_code_id, src.service_code, src.total_customers
HAVING src.total_customers != COUNT(pc.customer_id);
```

### total_customers 수동 동기화
```sql
-- total_customers를 실제 고객 수로 업데이트
UPDATE quel_service_referral_code src
SET total_customers = (
  SELECT COUNT(*)
  FROM quel_partner_customers pc
  WHERE pc.service_code_id = src.service_code_id
    AND pc.status = 'active'
);
```

---

Last Updated: 2025-11-05
