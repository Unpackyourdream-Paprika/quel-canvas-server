# quel_credits

크레딧 거래 내역 테이블 (Credit Transaction History)

## 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | uuid | NO | gen_random_uuid() | 거래 ID (PK) |
| user_id | uuid | YES | - | 사용자 ID (FK → quel_member) - 개인 크레딧용 |
| org_id | uuid | YES | - | 조직 ID (FK → quel_organization) - 조직 크레딧용 |
| used_by_member_id | uuid | YES | - | 실제 사용자 (FK → quel_member) - 조직 크레딧 사용 시 |
| transaction_type | varchar(20) | NO | - | 거래 유형 (purchase/DEDUCT/refund) |
| amount | integer | NO | - | 크레딧 변동량 (+ or -) |
| balance_after | integer | NO | - | 거래 후 잔액 |
| description | text | YES | - | 거래 설명 |
| attach_idx | bigint | YES | - | 첨부 파일 인덱스 (FK → quel_attach) |
| production_idx | uuid | YES | - | 프로덕션 ID (FK → quel_production_photo) |
| created_at | timestamp with time zone | NO | now() | 생성 시간 |

## 🔗 Relationships

**Foreign Keys:**
- `user_id` → `quel_member.quel_member_id`
- `org_id` → `quel_organization.org_id`
- `used_by_member_id` → `quel_member.quel_member_id`
- `production_idx` → `quel_production_photo.production_id`
- `attach_idx` → `quel_attach.attach_id`

## 🎯 Usage Patterns

| user_id | org_id | used_by_member_id | 의미 |
|---------|--------|-------------------|------|
| ✓ | NULL | NULL | 개인 크레딧 거래 |
| NULL | ✓ | ✓ | 조직 크레딧 거래 |

## 📝 Transaction Types

- **purchase**: 크레딧 구매 (결제 완료 시, amount > 0)
- **DEDUCT**: 크레딧 차감 (이미지 생성 등 사용 시, amount < 0)
- **refund**: 크레딧 환불 (amount > 0)

## 🔍 Common Queries

### 개인 크레딧 사용 내역
```sql
SELECT * FROM quel_credits
WHERE user_id = 'user_id_here'
ORDER BY created_at DESC;
```

### 조직 크레딧 사용 내역
```sql
SELECT 
  c.*,
  m.quel_name as used_by_name
FROM quel_credits c
LEFT JOIN quel_member m ON c.used_by_member_id = m.quel_member_id
WHERE c.org_id = 'org_id_here'
ORDER BY c.created_at DESC;
```

### 조직 내 멤버별 사용량
```sql
SELECT
  used_by_member_id,
  SUM(ABS(amount)) as total_used
FROM quel_credits
WHERE org_id = 'org_id_here'
  AND transaction_type = 'DEDUCT'
GROUP BY used_by_member_id;
```

## 📝 Code Examples

### 개인 크레딧 차감
```typescript
await supabaseAdmin()
  .from("quel_credits")
  .insert({
    user_id: memberId,
    transaction_type: "DEDUCT",
    amount: -amount,
    balance_after: member.quel_member_credit - amount,
    description: "이미지 생성",
  });
```

### 조직 크레딧 차감
```typescript
await supabaseAdmin()
  .from("quel_credits")
  .insert({
    org_id: orgId,
    used_by_member_id: memberId,
    transaction_type: "DEDUCT",
    amount: -amount,
    balance_after: org.org_credit - amount,
    description: "조직 크레딧 사용",
  });
```

## ⚠️ Important Notes

1. **user_id / org_id 배타적**: 둘 중 하나만 값이 있어야 함
2. **used_by_member_id**: org_id가 있을 때만 사용 (누가 조직 크레딧을 썼는지)
3. **balance_after**: 개인이면 개인 잔액, 조직이면 조직 잔액 기준
4. **amount**: 양수(구매/환불) 또는 음수(사용)

---

Last Updated: 2025-11-26