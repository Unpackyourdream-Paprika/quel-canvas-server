# partner_settlements

파트너 정산 내역 테이블 (결제 완료 시 생성)

## 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| settlement_id | uuid | NO | gen_random_uuid() | 정산 ID (PK) |
| payment_id | text | YES | - | Stripe Payment Intent ID |
| partner_id | uuid | YES | - | 파트너 ID (FK → quel_partners) |
| partner_level | integer | YES | - | 파트너 레벨 (1 or 2) |
| partner_name | text | YES | - | 파트너 이름 (스냅샷) |
| subtotal | integer | YES | - | 결제 총액 (세금 제외) |
| partner_share | integer | YES | - | 파트너 받을 금액 |
| currency | text | YES | - | 통화 코드 (JPY, KRW 등) |
| stripe_transfer_id | text | YES | - | Stripe Transfer ID (Destination Charges는 NULL) |
| stripe_account_id | text | YES | - | Stripe Connected Account ID |
| transfer_status | text | YES | - | 전송 상태 (success/pending/manual_required) |
| customer_id | uuid | YES | - | 고객 ID (FK → quel_member) |
| service_code | text | YES | - | 사용한 서비스 코드 |
| created_at | timestamp | YES | now() | 생성 시간 |

## 🔗 Relationships

**Foreign Keys:**
- `payment_id` → `quel_payment.id`
- `partner_id` → `quel_partners.partner_id`
- `customer_id` → `quel_member.quel_member_id`

## 🎯 Purpose

**이 테이블의 역할:**
1. 고객이 크레딧 충전 시 파트너에게 지급할 금액 기록
2. Stripe Destination Charges 실행 결과 추적 (Tier 1)
3. 수동 정산 대기 금액 기록 (Tier 2)
4. 정산 히스토리 및 감사(Audit) 기록
5. 파트너 대시보드에서 수익 확인 가능

**정산 방식 (2025-01-07 기준):**
- **Tier 1**: Destination Charges로 20% 자동 이체
- **Tier 2**: DB 기록만, Tier 1이 수동 분배 (Stripe 밖에서)

## 📝 Usage

### API Endpoints

#### Write Operations:
- `POST /api/stripe/webhook` - Stripe 결제 완료 시 INSERT (checkout.session.completed)

**File:** [src/app/api/stripe/webhook/route.ts](../../src/app/api/stripe/webhook/route.ts)

**실제 코드 (checkout.session.completed 이벤트):**

```typescript
case 'checkout.session.completed': {
  const session = event.data.object as Stripe.Checkout.Session;

  // 1. Payment Intent 조회
  const paymentIntent = await stripe.paymentIntents.retrieve(
    session.payment_intent as string
  );

  // 2. Tier 1 Destination Charges 정보 확인
  const tier1Share = paymentIntent.transfer_data?.amount || 0;
  const tier1AccountId = paymentIntent.transfer_data?.destination as string;
  const tier1PartnerId = session.metadata?.tier1_partner_id;
  const tier2PartnerId = session.metadata?.tier2_partner_id;
  const subtotal = parseInt(paymentIntent.metadata?.subtotal || '0');

  // 3. Tier 1 파트너 정보 조회 (our_company 확인)
  let tier1Partner = null;
  if (tier1PartnerId) {
    const { data } = await supabaseAdmin()
      .from('quel_partners')
      .select('partner_name, our_company, stripe_account_id')
      .eq('partner_id', tier1PartnerId)
      .single();

    tier1Partner = data;
  }

  // 4. 정산 기록 분기 처리
  if (tier1Partner?.our_company === true) {
    // 한국 시장 (our_company = true): Tier 2만 기록
    if (tier2PartnerId && tier1Share > 0) {
      const { data: tier2Partner } = await supabaseAdmin()
        .from('quel_partners')
        .select('partner_name')
        .eq('partner_id', tier2PartnerId)
        .single();

      // Tier 2는 partner_rate의 100%를 받음 (한국 마스터는 0%)
      await supabaseAdmin().from('partner_settlements').insert({
        payment_id: paymentIntent.id,
        partner_id: tier2PartnerId,
        partner_level: 2,
        partner_name: tier2Partner?.partner_name,
        subtotal: subtotal,
        partner_share: tier1Share, // 100% of partner_rate
        currency: paymentIntent.currency.toUpperCase(),
        stripe_transfer_id: null,
        stripe_account_id: null,
        transfer_status: 'manual_required',
        customer_id: session.metadata?.user_id,
        service_code: session.metadata?.service_code || null,
        created_at: new Date().toISOString(),
      });
    }
  } else {
    // 일본 시장 (our_company = false): Tier 1 + Tier 2 기록
    // Tier 1 정산 기록 (Destination Charges로 자동 이체됨)
    if (tier1Share > 0 && tier1AccountId && tier1PartnerId) {
      await supabaseAdmin().from('partner_settlements').insert({
        payment_id: paymentIntent.id,
        partner_id: tier1PartnerId,
        partner_level: 1,
        partner_name: tier1Partner?.partner_name,
        subtotal: subtotal,
        partner_share: tier1Share,
        currency: paymentIntent.currency.toUpperCase(),
        stripe_transfer_id: null, // Destination Charges는 별도 transfer_id 없음
        stripe_account_id: tier1AccountId,
        transfer_status: 'success',
        customer_id: session.metadata?.user_id,
        service_code: session.metadata?.service_code || null,
        created_at: new Date().toISOString(),
      });
    }

    // Tier 2 정산 기록 (수동 분배 대기)
    if (tier2PartnerId && tier1Share > 0) {
      const { data: tier2Partner } = await supabaseAdmin()
        .from('quel_partners')
        .select('partner_name')
        .eq('partner_id', tier2PartnerId)
        .single();

      // Tier 2는 Tier 1이 받은 20% 중 60%를 받음
      const tier2Share = Math.round(tier1Share * 0.6); // 20% * 60% = 12%

      await supabaseAdmin().from('partner_settlements').insert({
        payment_id: paymentIntent.id,
        partner_id: tier2PartnerId,
        partner_level: 2,
        partner_name: tier2Partner?.partner_name,
        subtotal: subtotal,
        partner_share: tier2Share,
        currency: paymentIntent.currency.toUpperCase(),
        stripe_transfer_id: null,
        stripe_account_id: null, // Tier 2는 Stripe 밖에서 수동 정산
        transfer_status: 'manual_required',
        customer_id: session.metadata?.user_id,
        service_code: session.metadata?.service_code || null,
        created_at: new Date().toISOString(),
      });
    }
  }

  // 5. 사용자 크레딧 추가
  const credits = parseInt(session.metadata?.plan_credits || '0');
  if (credits > 0) {
    await supabaseAdmin()
      .from('quel_member')
      .update({
        quel_member_credit: credits,
      })
      .eq('quel_member_id', session.metadata?.user_id);
  }

  break;
}
```

## 🔍 Common Queries

### 파트너의 총 정산 금액
```sql
SELECT
  partner_id,
  partner_name,
  SUM(partner_share) as total_earnings,
  COUNT(*) as settlement_count
FROM partner_settlements
WHERE transfer_status = 'success'
GROUP BY partner_id, partner_name
ORDER BY total_earnings DESC;
```

### 특정 결제의 정산 내역 (Tier 1 + Tier 2)
```sql
SELECT
  ps.*,
  p.partner_email,
  p.partner_country
FROM partner_settlements ps
JOIN quel_partners p ON ps.partner_id = p.partner_id
WHERE ps.payment_id = 'xxx'
ORDER BY ps.partner_level;
```

### 실패한 정산 목록 (수동 처리 필요)
```sql
SELECT
  ps.*,
  p.partner_email,
  p.partner_country,
  m.quel_member_email as customer_email
FROM partner_settlements ps
JOIN quel_partners p ON ps.partner_id = p.partner_id
JOIN quel_member m ON ps.customer_id = m.quel_member_id
WHERE ps.transfer_status IN ('failed', 'manual_required')
ORDER BY ps.created_at DESC;
```

### 서비스 코드별 정산 통계
```sql
SELECT
  service_code,
  COUNT(DISTINCT customer_id) as unique_customers,
  COUNT(*) as total_settlements,
  SUM(partner_share) as total_distributed,
  SUM(subtotal) as total_revenue
FROM partner_settlements
WHERE transfer_status = 'success'
GROUP BY service_code
ORDER BY total_revenue DESC;
```

## 🔄 Data Flow

```
1. Customer purchases credits
   ↓
2. Stripe Checkout completed
   ↓
3. Webhook: checkout.session.completed
   ↓
4. Insert quel_payment
   ↓
5. Update quel_member.quel_member_credit
   ↓
6. Insert quel_credits_transactions
   ↓
7. Check if customer has service_code
   ↓
8. Get commission rates from quel_commission_rates
   ↓
9. Calculate Tier 1 & Tier 2 shares
   ↓
10. Execute Stripe Transfers (if eligible)
    ↓
11. INSERT partner_settlements (Tier 2) ← YOU ARE HERE
    ↓
12. INSERT partner_settlements (Tier 1 if exists)
```

## ⚠️ Important Notes

1. **Transfer Status Values:**
   - `success`: Destination Charges로 자동 이체 완료 (Tier 1 only)
   - `manual_required`: 수동 정산 필요 (Tier 2 always)
   - `pending`: Settlement period 대기 중 (사용 안 함)

2. **정산 방식별 차이:**
   - **Tier 1 (Level 1)**: Destination Charges → `transfer_status = 'success'`
   - **Tier 2 (Level 2)**: 수동 분배 → `transfer_status = 'manual_required'`

3. **Currency Support:**
   - JPY: 일본 파트너 (Destination Charges 작동)
   - KRW: 한국 고객 (Tier 2는 Tier 1이 수동 분배)
   - Platform은 multi-currency 지원 (JPY, USD 별도 관리)

4. **Snapshot Data:**
   - `partner_name`, `service_code`는 스냅샷 (나중에 변경되어도 정산 기록은 유지)
   - `subtotal`, `partner_share`, `currency`는 정산 당시 금액 기록

5. **Audit Trail:**
   - 모든 정산은 실패해도 기록됨 (감사 추적 가능)
   - `payment_id`로 Stripe Dashboard에서 Payment Intent 확인 가능
   - Tier 1: `stripe_account_id`로 Connected Account 확인

6. **Idempotency:**
   - 같은 `payment_id` + `partner_id`로 중복 INSERT 방지 로직 필요 (webhook 재전송 대비)

## 📊 Statistics

### 월별 정산 통계
```sql
SELECT
  DATE_TRUNC('month', created_at) as month,
  COUNT(*) as settlement_count,
  SUM(partner_share) as total_paid,
  SUM(CASE WHEN transfer_status = 'success' THEN 1 ELSE 0 END) as successful_transfers,
  SUM(CASE WHEN transfer_status = 'manual_required' THEN 1 ELSE 0 END) as manual_pending
FROM partner_settlements
GROUP BY DATE_TRUNC('month', created_at)
ORDER BY month DESC;
```

### 파트너별 정산 성공률
```sql
SELECT
  partner_id,
  partner_name,
  COUNT(*) as total_settlements,
  SUM(CASE WHEN transfer_status = 'success' THEN 1 ELSE 0 END) as successful,
  ROUND(100.0 * SUM(CASE WHEN transfer_status = 'success' THEN 1 ELSE 0 END) / COUNT(*), 2) as success_rate
FROM partner_settlements
GROUP BY partner_id, partner_name
ORDER BY success_rate DESC;
```

## 🐛 Troubleshooting

### Transfer 실패 시 확인사항
```sql
-- 실패한 정산의 파트너 상태 확인
SELECT
  ps.settlement_id,
  ps.transfer_status,
  p.stripe_onboarding_completed,
  p.stripe_final_onboarding_completed,
  p.partner_country
FROM partner_settlements ps
JOIN quel_partners p ON ps.partner_id = p.partner_id
WHERE ps.transfer_status = 'failed';
```

---

## 💾 SQL Schema

```sql
CREATE TABLE partner_settlements (
  settlement_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  payment_id TEXT NOT NULL,
  partner_id UUID REFERENCES quel_partners(partner_id),
  partner_level INTEGER NOT NULL CHECK (partner_level IN (1, 2)),
  partner_name TEXT,
  subtotal INTEGER NOT NULL,
  partner_share INTEGER NOT NULL,
  currency TEXT NOT NULL DEFAULT 'JPY',
  stripe_transfer_id TEXT,
  stripe_account_id TEXT,
  transfer_status TEXT NOT NULL CHECK (transfer_status IN ('success', 'manual_required')),
  customer_id UUID REFERENCES quel_member(quel_member_id),
  service_code TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

  -- Prevent duplicate settlements
  UNIQUE (payment_id, partner_id)
);

-- Indexes for common queries
CREATE INDEX idx_partner_settlements_partner_id ON partner_settlements(partner_id);
CREATE INDEX idx_partner_settlements_payment_id ON partner_settlements(payment_id);
CREATE INDEX idx_partner_settlements_customer_id ON partner_settlements(customer_id);
CREATE INDEX idx_partner_settlements_transfer_status ON partner_settlements(transfer_status);
CREATE INDEX idx_partner_settlements_created_at ON partner_settlements(created_at DESC);
CREATE INDEX idx_partner_settlements_service_code ON partner_settlements(service_code);
```

---

Last Updated: 2025-01-07
