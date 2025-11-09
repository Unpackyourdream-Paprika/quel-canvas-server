# Complete Payment & Settlement Flow

QUELSUITE 전체 결제 및 정산 플로우 통합 문서

---

## 📊 비즈니스 모델 개요

### Commission Structure

**일본 시장 (our_company = false):**
```
Customer Payment: ¥100,000
├─ Company: 80% = ¥80,000
└─ Tier 1 (일본 파트너): 20% = ¥20,000 (Stripe 자동 입금)
   └─ Tier 1 → Tier 2 수동 분배
      ├─ Tier 1 keeps: 40% of ¥20,000 = ¥8,000
      └─ Tier 2 receives: 60% of ¥20,000 = ¥12,000 (수동 송금)
```

**한국 시장 (our_company = true):**
```
Customer Payment: ₩100,000
├─ Company: 0% (partner_rate = 100%)
└─ Tier 1 (QUELSUITE Korea Master): 100% = ₩100,000 (Platform 보유)
   └─ Platform → Tier 2 수동 분배
      ├─ Tier 1 (Platform): 0% (우리 회사)
      └─ Tier 2 receives: 100% = ₩100,000 (수동 송금)
```

---

## 🔄 전체 플로우

### 1. 고객 결제 시작

```
Customer clicks "CHARGE"
↓
Frontend: /api/stripe/checkout (POST)
├─ user_id
├─ plan_id
└─ (optional) service_code
```

### 2. Checkout Session 생성

**File:** `src/app/api/stripe/checkout/route.ts`

```typescript
export async function POST(req: NextRequest) {
  const { planId, userId } = await req.json();

  // 1. Plan 정보 조회
  const { data: plan } = await supabaseAdmin()
    .from('plans')
    .select('*')
    .eq('id', planId)
    .single();

  // 2. 사용자의 파트너 정보 조회
  const { data: member } = await supabaseAdmin()
    .from('quel_member')
    .select(`
      *,
      service_code:quel_service_referral_code!service_code_id(
        service_code_id,
        tier2_partner_id
      )
    `)
    .eq('quel_member_id', userId)
    .single();

  // 3. Tier 2 파트너 정보 조회
  let tier1AccountId = null;
  let tier2PartnerId = null;
  let tier1PartnerId = null;

  if (member?.service_code?.tier2_partner_id) {
    const { data: tier2Partner } = await supabaseAdmin()
      .from('quel_partners')
      .select('partner_id, stripe_account_id, referrer_partner_id')
      .eq('partner_id', member.service_code.tier2_partner_id)
      .single();

    if (tier2Partner) {
      tier2PartnerId = tier2Partner.partner_id;
      tier1PartnerId = tier2Partner.referrer_partner_id;

      // Tier 1 파트너 정보 조회
      if (tier1PartnerId) {
        const { data: tier1Partner } = await supabaseAdmin()
          .from('quel_partners')
          .select('stripe_account_id, our_company')
          .eq('partner_id', tier1PartnerId)
          .single();

        // our_company = false인 경우에만 Stripe 계정 사용
        if (tier1Partner?.our_company === false) {
          tier1AccountId = tier1Partner?.stripe_account_id;
        }
      }
    }
  }

  // 4. Checkout Session 파라미터 설정
  const sessionParams: any = {
    payment_method_types: ['card'],
    line_items: [
      {
        price: plan.price_id,
        quantity: 1,
      },
    ],
    mode: 'payment',
    success_url: `${process.env.NEXT_PUBLIC_URL}/success?session_id={CHECKOUT_SESSION_ID}`,
    cancel_url: `${process.env.NEXT_PUBLIC_URL}/cancel`,
    client_reference_id: userId,
    metadata: {
      plan_id: planId,
      user_id: userId,
      plan_credits: plan.credits.toString(),
      tier1_partner_id: tier1PartnerId || '',
      tier2_partner_id: tier2PartnerId || '',
      service_code: member?.service_code?.service_code || '',
    },
  };

  // 5. Commission rates 조회
  let commissionRate = null;
  if (tier1PartnerId) {
    const { data: rate } = await supabaseAdmin()
      .from('quel_commission_rates')
      .select('company_rate, partner_rate')
      .or(`partner_id.eq.${tier1PartnerId},partner_id.is.null`)
      .order('partner_id', { nullsLast: true })
      .limit(1)
      .single();

    commissionRate = rate;
  }

  // 6. Tier 1 파트너가 있고 our_company = false인 경우 Destination Charges 설정
  if (tier1AccountId && commissionRate) {
    const subtotal = plan.price; // 세금 제외 금액
    const partnerRate = commissionRate.partner_rate / 100; // 20% → 0.20
    const tier1Share = Math.round(subtotal * partnerRate); // 20% 전체

    sessionParams.payment_intent_data = {
      transfer_data: {
        amount: tier1Share,
        destination: tier1AccountId,
      },
      on_behalf_of: tier1AccountId, // 핵심: Cross-border settlement 해결
      metadata: {
        tier1_partner_id: tier1PartnerId || '',
        tier2_partner_id: tier2PartnerId || '',
        subtotal: subtotal.toString(),
      },
    };
  }

  // 7. Checkout Session 생성
  const session = await stripe.checkout.sessions.create(sessionParams);

  return NextResponse.json({ url: session.url });
}
```

### 3. Stripe Checkout

```
Customer enters payment info
↓
Stripe processes payment
↓
checkout.session.completed event → Webhook
```

### 4. Webhook 처리

**File:** `src/app/api/stripe/webhook/route.ts`

```typescript
export async function POST(req: NextRequest) {
  const body = await req.text();
  const sig = req.headers.get('stripe-signature')!;

  let event: Stripe.Event;

  try {
    event = stripe.webhooks.constructEvent(
      body,
      sig,
      process.env.STRIPE_WEBHOOK_SECRET!
    );
  } catch (err: any) {
    return NextResponse.json({ error: err.message }, { status: 400 });
  }

  switch (event.type) {
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
  }

  return NextResponse.json({ received: true });
}
```

---

## 🌏 Market-Specific Flows

### 일본 시장 (our_company = false)

```
Customer pays ¥100,000
  ↓
Stripe Checkout Session
  ├─ Destination Charges:
  │  ├─ Company: ¥80,000 → Platform Balance
  │  └─ Tier 1: ¥20,000 → JP Connected Account (자동 이체)
  ↓
Webhook: checkout.session.completed
  ↓
Check: tier1Partner.our_company === false
  ↓
partner_settlements INSERT (×2):
  ├─ Tier 1: partner_share = ¥20,000, transfer_status = 'success'
  └─ Tier 2: partner_share = ¥12,000, transfer_status = 'manual_required'
  ↓
Tier 1 나중에 Tier 2에게 ¥12,000 수동 송금
```

### 한국 시장 (our_company = true)

```
Customer pays ₩100,000 → Stripe converts to USD
  ↓
Stripe Checkout Session
  ├─ No Destination Charges (tier1AccountId = null)
  ├─ Company: 0% (partner_rate = 100%)
  └─ Platform Balance: USD equivalent
  ↓
Webhook: checkout.session.completed
  ↓
Check: tier1Partner.our_company === true
  ↓
partner_settlements INSERT (×1):
  └─ Tier 2 only: partner_share = 100% of partner_rate, transfer_status = 'manual_required'
  (Tier 1 기록 안함 - 우리 회사니까)
  ↓
Admin에서 Tier 2에게 수동 송금
```

---

## 💾 Database Changes

### 1. partner_settlements 테이블

**Currency 컬럼 추가:**
```sql
ALTER TABLE partner_settlements
ADD COLUMN currency TEXT DEFAULT 'KRW';
```

**Payment_id 타입 변경 (optional - 현재 uuid, 문서는 text):**
```sql
-- 현재 테이블에 데이터가 없다면:
ALTER TABLE partner_settlements
ALTER COLUMN payment_id TYPE TEXT;

-- 데이터가 있다면 migration 필요
```

### 2. quel_partners 테이블

**our_company 컬럼 추가:**
```sql
ALTER TABLE quel_partners
ADD COLUMN our_company BOOLEAN DEFAULT false;
```

**한국 마스터 계정 생성 예시:**
```sql
INSERT INTO quel_partners (
  partner_name,
  partner_email,
  partner_country,
  partner_level,
  our_company,
  stripe_account_id,
  created_at
) VALUES (
  'QUELSUITE Korea Master',
  'korea@quelsuite.com',
  'KR',
  1,
  true,
  NULL,
  NOW()
);
```

### 3. quel_commission_rates

**한국 마스터 계정용 Rate 설정:**
```sql
INSERT INTO quel_commission_rates (
  partner_id,
  company_rate,
  partner_rate,
  effective_date,
  notes
) VALUES (
  '<korean_master_partner_id>',
  0.00,
  100.00,
  NOW(),
  '한국 마스터 계정 - Tier 2에게 100% 분배'
);
```

---

## 🔑 Key Implementation Points

### 1. Checkout API

**Destination Charges 설정 조건:**
```typescript
// our_company = false AND stripe_account_id 있을 때만
if (tier1AccountId && commissionRate && !tier1Partner?.our_company) {
  sessionParams.payment_intent_data = {
    transfer_data: {
      amount: tier1Share,
      destination: tier1AccountId,
    },
    on_behalf_of: tier1AccountId,
  };
}
```

### 2. Webhook API

**정산 기록 분기:**
```typescript
if (tier1Partner?.our_company === true) {
  // 한국: Tier 2만 기록 (100%)
  insertTier2Only(tier1Share); // 100% of partner_rate
} else {
  // 일본: Tier 1 + Tier 2 기록
  insertTier1(tier1Share); // 20% of subtotal
  insertTier2(tier1Share * 0.6); // 60% of tier1Share
}
```

### 3. Currency Handling

**Multi-currency support:**
- JPY → JPY (no conversion)
- KRW → USD (Stripe auto-converts)
- Platform holds separate balances (JPY Balance, USD Balance)
- `currency` 컬럼에 원본 통화 기록

---

## 📊 Admin Dashboard Queries

### 한국 Tier 2 파트너 정산 현황

```sql
SELECT
  ps.partner_name,
  ps.partner_share,
  ps.currency,
  ps.created_at,
  ps.transfer_status,
  m.quel_member_email as customer_email
FROM partner_settlements ps
JOIN quel_partners p ON ps.partner_id = p.partner_id
JOIN quel_partners t1 ON p.referrer_partner_id = t1.partner_id
JOIN quel_member m ON ps.customer_id = m.quel_member_id
WHERE t1.our_company = true
  AND ps.transfer_status = 'manual_required'
ORDER BY ps.created_at DESC;
```

### 일본 파트너 자동 정산 내역

```sql
SELECT
  ps.partner_name,
  ps.partner_level,
  ps.partner_share,
  ps.currency,
  ps.stripe_account_id,
  ps.created_at
FROM partner_settlements ps
JOIN quel_partners p ON ps.partner_id = p.partner_id
WHERE p.our_company = false
  AND ps.transfer_status = 'success'
ORDER BY ps.created_at DESC;
```

---

## 🧪 Testing Scenarios

### Scenario 1: 일본 고객 → 일본 파트너

```
1. Create JP Tier 1 partner with Stripe account
2. Create JP Tier 2 partner under Tier 1
3. Create service code for Tier 2
4. Customer registers with service code
5. Customer pays ¥100,000
6. Verify:
   - Platform Balance: ¥80,000
   - Tier 1 Balance: ¥20,000 (Pending → Available after 4 days)
   - partner_settlements: 2 rows (Tier 1: success, Tier 2: manual_required)
```

### Scenario 2: 한국 고객 → 한국 영업진

```
1. Create KR Master (our_company = true, no Stripe account)
2. Set commission: company_rate = 0%, partner_rate = 100%
3. Create KR Tier 2 partner under Master
4. Create service code for Tier 2
5. Customer registers with service code
6. Customer pays ₩100,000
7. Verify:
   - Platform Balance: USD equivalent
   - partner_settlements: 1 row (Tier 2 only, manual_required)
   - No Tier 1 settlement record
```

---

## 🚨 Error Handling

### Webhook Idempotency

```typescript
// UNIQUE constraint on (payment_id, partner_id) prevents duplicates
try {
  await supabaseAdmin().from('partner_settlements').insert({...});
} catch (error) {
  if (error.code === '23505') { // Duplicate key
    console.log('Settlement already recorded, skipping');
    return;
  }
  throw error;
}
```

### Missing Partner Data

```typescript
if (!tier1Partner) {
  console.error('Tier 1 partner not found:', tier1PartnerId);
  // Record to error log, but continue with credit addition
}

if (!tier2Partner) {
  console.error('Tier 2 partner not found:', tier2PartnerId);
  // Skip settlement, but add credits
}
```

---

## 📝 Summary

### 일본 시장 (Automated)

- ✅ Destination Charges로 Tier 1 자동 정산
- ✅ Multi-currency support (JPY)
- ⚠️ Tier 2는 수동 정산 (Stripe 제약)

### 한국 시장 (Manual)

- ✅ 커미션 100% → Tier 2 분배
- ✅ Platform이 직접 관리
- ✅ DB에서 매출 확인 후 수동 송금

---

Last Updated: 2025-01-07
