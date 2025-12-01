# organization_plan

조직 멤버 구독 플랜 테이블

## 📋 Key Columns

| Column | Type | Description |
|--------|------|-------------|
| id | uuid | 플랜 ID (PK) |
| name | text | 플랜명 (Stripe 상품명과 동일) |
| price | bigint | 가격 (통화 단위) |
| price_id | text | Stripe Price ID |
| currency | text | 통화 (KRW/JPY) |
| country | text | 국가 코드 (KR/JP) |
| billing_period | text | 결제 주기 (monthly/yearly) |
| description | text | 플랜 설명 |
| active | boolean | 활성화 여부 |
| created_at | timestamptz | 생성 시간 |

## 🔗 Related Tables

- 조직 멤버 구독 시 이 테이블에서 `price_id` 조회 → Stripe Checkout 생성

## 💡 Usage
```sql
-- 국가별 활성 플랜 조회
SELECT * FROM organization_plan 
WHERE country = 'KR' AND active = true;
```

## 📝 Sample Data

| name | price | currency | country | billing_period |
|------|-------|----------|---------|----------------|
| quelsuite-organization-month-one-people | 49000 | KRW | KR | monthly |
| quelsuite-organization-month-one-people | 5000 | JPY | JP | monthly |