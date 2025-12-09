# quel_partner_customers

파트너-고객 관계 테이블 (서비스 코드 등록 시 생성)

## 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | uuid | NO | gen_random_uuid() | 관계 ID (PK) |
| customer_id | uuid | NO | - | 고객 ID (FK → quel_member) |
| tier1_partner_id | uuid | YES | - | Tier 1 파트너 ID (FK → quel_partners) |
| tier2_partner_id | uuid | YES | - | Tier 2 파트너 ID (FK → quel_partners) |
| service_code_id | uuid | YES | - | 서비스 코드 ID (FK → quel_service_referral_code) |
| credit_code_used | varchar(50) | YES | - | 사용한 서비스 코드 |
| description | text | YES | - | 설명 |
| registered_at | timestamp | YES | now() | 등록 시간 |
| status | varchar(50) | YES | 'active' | 상태 (active/inactive) |

## 🔗 Relationships

**Foreign Keys:**
- `customer_id` → `quel_member.quel_member_id`
- `tier1_partner_id` → `quel_partners.partner_id` (nullable)
- `tier2_partner_id` → `quel_partners.partner_id`
- `service_code_id` → `quel_service_referral_code.service_code_id`

## 🎯 Purpose

**이 테이블의 역할:**
1. 고객이 어떤 파트너의 서비스 코드를 사용했는지 기록
2. Tier 1 (상위 파트너)과 Tier 2 (직접 파트너) 관계 추적
3. 파트너 대시보드에서 "내 고객 목록" 조회 가능
4. 정산 시 누구에게 얼마를 줘야 하는지 확인

## 📝 Usage

### API Endpoints

#### Write Operations:
- `POST /api/register-service-code` - 서비스 코드 등록 시 INSERT

**File:** [src/app/api/register-service-code/route.ts](../../src/app/api/register-service-code/route.ts)

```typescript
// Line 102-113
await supabaseAdmin()
  .from("quel_partner_customers")
  .insert({
    customer_id: memberId,
    tier1_partner_id: tier1PartnerId,  // referral_partner_id에서 가져옴
    tier2_partner_id: serviceCodeData.tier2_partner_id,
    service_code_id: serviceCodeData.service_code_id,
    credit_code_used: normalizedCode,
    description: `Customer registered with service code: ${normalizedCode}`,
    status: "active",
  });
```

### Read Operations (예상 - 파트너 대시보드용)

#### Tier 2 파트너의 고객 목록
```typescript
const { data: customers } = await supabaseAdmin()
  .from("quel_partner_customers")
  .select(`
    *,
    quel_member:customer_id (
      quel_member_email,
      quel_member_name,
      quel_member_credit
    )
  `)
  .eq("tier2_partner_id", partnerId)
  .eq("status", "active")
  .order("registered_at", { ascending: false });
```

#### Tier 1 파트너의 전체 고객 (하위 Tier 2 고객 포함)
```typescript
const { data: customers } = await supabaseAdmin()
  .from("quel_partner_customers")
  .select(`
    *,
    quel_member:customer_id (
      quel_member_email,
      quel_member_name
    ),
    tier2_partner:tier2_partner_id (
      partner_name
    )
  `)
  .eq("tier1_partner_id", partnerId)
  .eq("status", "active")
  .order("registered_at", { ascending: false });
```

## 🔍 Common Queries

### 특정 고객의 파트너 정보 조회
```sql
SELECT
  pc.*,
  t1.partner_name as tier1_name,
  t1.partner_email as tier1_email,
  t2.partner_name as tier2_name,
  t2.partner_email as tier2_email,
  src.service_code
FROM quel_partner_customers pc
LEFT JOIN quel_partners t1 ON pc.tier1_partner_id = t1.partner_id
LEFT JOIN quel_partners t2 ON pc.tier2_partner_id = t2.partner_id
LEFT JOIN quel_service_referral_code src ON pc.service_code_id = src.service_code_id
WHERE pc.customer_id = 'xxx';
```

### Tier 2 파트너의 고객 수 집계
```sql
SELECT
  t2.partner_name,
  COUNT(pc.customer_id) as total_customers
FROM quel_partner_customers pc
JOIN quel_partners t2 ON pc.tier2_partner_id = t2.partner_id
WHERE pc.status = 'active'
GROUP BY t2.partner_id, t2.partner_name
ORDER BY total_customers DESC;
```

### Tier 1 파트너의 전체 네트워크 고객 수
```sql
SELECT
  t1.partner_name,
  COUNT(pc.customer_id) as total_network_customers
FROM quel_partner_customers pc
JOIN quel_partners t1 ON pc.tier1_partner_id = t1.partner_id
WHERE pc.status = 'active'
GROUP BY t1.partner_id, t1.partner_name
ORDER BY total_network_customers DESC;
```

## 🔄 Data Flow

```
1. User enters service code in UI
   ↓
2. POST /api/verify-service-code (validate)
   ↓
3. POST /api/register-service-code
   ↓
4. Query quel_service_referral_code (get tier2_partner_id)
   ↓
5. Query quel_partners (get referral_partner_id = tier1_partner_id)
   ↓
6. Update quel_member (save referral info)
   ↓
7. INSERT quel_partner_customers ← YOU ARE HERE
   ↓
8. Increment quel_service_referral_code.total_customers
```

## ⚠️ Important Notes

1. **Tier 1 is nullable**: Tier 2 파트너가 직접 가입했으면 Tier 1이 없을 수 있음
2. **One-to-one relationship**: 한 고객은 하나의 서비스 코드만 등록 가능 (현재 로직)
3. **Status field**: 향후 고객이 파트너 관계를 끊을 경우 'inactive'로 변경 가능
4. **RLS Policies**: Tier 1과 Tier 2 파트너 각각 자기 고객만 조회 가능하도록 설정됨

## 📊 RLS (Row Level Security)

```sql
-- Tier 1 파트너가 자기 고객 볼 수 있게
CREATE POLICY "Tier1 partners can view own customers"
ON quel_partner_customers
FOR SELECT
USING (
  tier1_partner_id IN (
    SELECT partner_id FROM quel_partners
    WHERE quel_member_id = auth.uid()
  )
);

-- Tier 2 파트너가 자기 고객 볼 수 있게
CREATE POLICY "Tier2 partners can view own customers"
ON quel_partner_customers
FOR SELECT
USING (
  tier2_partner_id IN (
    SELECT partner_id FROM quel_partners
    WHERE quel_member_id = auth.uid()
  )
);
```

## 📈 Statistics

### 전체 관계 수
```sql
SELECT COUNT(*) as total_relationships
FROM quel_partner_customers
WHERE status = 'active';
```

### 서비스 코드별 고객 수
```sql
SELECT
  credit_code_used,
  COUNT(*) as customer_count
FROM quel_partner_customers
WHERE status = 'active'
GROUP BY credit_code_used
ORDER BY customer_count DESC;
```

---

Last Updated: 2025-11-05
