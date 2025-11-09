# quel_payment

결제 정보 테이블

## 📋 Key Columns

| Column | Type | Description |
|--------|------|-------------|
| id | uuid | 결제 ID (PK) |
| quel_member_id | uuid | 회원 ID (FK → quel_member) |
| stripe_session_id | text | Stripe Checkout Session ID |
| stripe_payment_intent_id | text | Stripe Payment Intent ID |
| amount | integer | 결제 금액 (원화 단위) |
| currency | varchar | 통화 (KRW/JPY) |
| payment_status | varchar | 상태 (pending/completed/failed) |
| credits_purchased | integer | 구매한 크레딧 수 |
| created_at | timestamp | 생성 시간 |

## 📝 Usage

### API Endpoints

**File:** [src/app/api/stripe/checkout/route.ts](../../src/app/api/stripe/checkout/route.ts)

```typescript
// Checkout 세션 생성
const session = await stripe.checkout.sessions.create({
  mode: 'payment',
  // ...
});

// 결제 기록 생성
await supabase.from('quel_payment').insert({
  quel_member_id: userId,
  stripe_session_id: session.id,
  amount: totalAmount,
  currency: 'krw',
  payment_status: 'pending',
  credits_purchased: creditsAmount
});
```

**File:** [src/app/api/stripe/webhook/route.ts](../../src/app/api/stripe/webhook/route.ts)

```typescript
// checkout.session.completed 이벤트
const { data: payment } = await supabase
  .from('quel_payment')
  .select('*')
  .eq('stripe_session_id', session.id)
  .single();

// 결제 완료 업데이트
await supabase
  .from('quel_payment')
  .update({
    payment_status: 'completed',
    stripe_payment_intent_id: session.payment_intent
  })
  .eq('id', payment.id);
```

## 🔗 Relationships

**Referenced By:**
- `quel_credits_transactions.payment_id`
- `partner_settlements.payment_id`

## 🔄 Data Flow

```
1. User selects credit plan
   ↓
2. POST /api/stripe/checkout
   ↓
3. INSERT quel_payment (status: pending)
   ↓
4. Redirect to Stripe Checkout
   ↓
5. User completes payment
   ↓
6. Webhook: checkout.session.completed
   ↓
7. UPDATE quel_payment (status: completed)
   ↓
8. INSERT quel_credits_transactions
   ↓
9. UPDATE quel_member.quel_member_credit
   ↓
10. INSERT partner_settlements (if has service code)
```

---

Last Updated: 2025-11-05
