# quel_member

회원 정보 테이블

## 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| quel_member_id | uuid | NO | gen_random_uuid() | 회원 고유 ID (PK) |
| quel_email | text | NO | - | 이메일 |
| quel_name | text | YES | - | 이름 |
| provider | text | NO | - | OAuth 제공자 (google 등) |
| provider_sub | text | NO | - | OAuth 제공자 고유 ID |
| quel_member_credit | bigint | YES | 0 | 보유 크레딧 |
| referral_service_code | text | YES | - | 등록한 서비스 코드 |
| service_code_id | uuid | YES | - | 서비스 코드 ID (FK) |
| tier2_partner_id | uuid | YES | - | Tier 2 파트너 ID (FK) |
| referral_code_registered_at | timestamp | YES | - | 코드 등록 시간 |
| quel_member_status | USER-DEFINED | NO | 'active' | 회원 상태 (active/inactive/deleted 등) |
| quel_created_at | timestamp with time zone | NO | now() | 가입 시간 |
| quel_updated_at | timestamp with time zone | NO | now() | 업데이트 시간 |
| quel_deleted_at | timestamp with time zone | YES | - | 삭제 시간 (soft delete) |
| quel_avatar_url | text | YES | - | 프로필 이미지 URL |
| ui_language | text | YES | 'ko' | UI 언어 설정 |
| ui_scale | text | YES | 'M' | UI 스케일 설정 |

## 🔗 Relationships

**Foreign Keys:**
- `service_code_id` → `quel_service_referral_code.service_code_id`
- `tier2_partner_id` → `quel_partners.partner_id`

**Referenced By:**
- `quel_partner_customers.customer_id`
- `quel_production_photo.quel_member_id`
- `quel_production_jobs.quel_member_id`
- `quel_credits_transactions.quel_member_id`
- `quel_payment.quel_member_id`
- `partner_settlements.customer_id`

## 📝 Usage

### API Endpoints

#### Read Operations:
- `GET /api/user/me` - 현재 사용자 정보 조회
- `GET /api/auth/me` - 인증 상태 확인
- `GET /api/credits/available` - 사용 가능한 크레딧 확인

#### Write Operations:
- `POST /api/register-service-code` - 서비스 코드 등록 (referral 정보 업데이트)
- `POST /api/stripe/webhook` - 결제 완료 시 크레딧 증가
- `POST /api/credits/deduct` - 크레딧 차감
- `POST /api/oauth/google/callback` - Google OAuth 회원가입/로그인

### Code Examples

#### 회원 정보 조회
```typescript
const { data: member } = await supabaseAdmin()
  .from("quel_member")
  .select("*")
  .eq("quel_member_id", memberId)
  .single();
```

#### 서비스 코드 등록
```typescript
await supabaseAdmin()
  .from("quel_member")
  .update({
    referral_service_code: normalizedCode,
    service_code_id: serviceCodeData.service_code_id,
    tier2_partner_id: serviceCodeData.tier2_partner_id,
    referral_code_registered_at: new Date().toISOString(),
  })
  .eq("quel_member_id", memberId);
```

#### 크레딧 업데이트
```typescript
await supabaseAdmin()
  .from("quel_member")
  .update({
    quel_member_credit: member.quel_member_credit + creditAmount
  })
  .eq("quel_member_id", memberId);
```

## 🔍 Common Queries

### 특정 파트너의 모든 고객 조회
```sql
SELECT
  m.*,
  pc.registered_at as code_registered_at
FROM quel_member m
JOIN quel_partner_customers pc ON m.quel_member_id = pc.customer_id
WHERE pc.tier2_partner_id = 'xxx'
  OR pc.tier1_partner_id = 'xxx';
```

### 크레딧이 부족한 회원 조회
```sql
SELECT * FROM quel_member
WHERE quel_member_credit < 100;
```

### 서비스 코드 미등록 회원 조회
```sql
SELECT * FROM quel_member
WHERE referral_service_code IS NULL;
```

## ⚠️ Important Notes

1. **크레딧 잔액**: `quel_member_credit`은 실시간 잔액이며, 모든 증감은 `quel_credits_transactions`에 기록됨
2. **서비스 코드**: 한 번 등록하면 변경 불가 (현재 로직)
3. **파트너 관계**: `tier2_partner_id`는 직접 연결된 파트너, Tier 1은 `quel_partners.referral_partner_id`로 확인
4. **OAuth 통합**: Google OAuth 로그인 시 자동으로 회원 생성
5. **UI 설정**: `ui_language`(ko/en 등), `ui_scale`(S/M/L 등) 사용자별 UI 환경설정

## 📊 Statistics

### 총 회원 수
```sql
SELECT COUNT(*) as total_members FROM quel_member;
```

### 서비스 코드 등록 회원 수
```sql
SELECT COUNT(*) as registered_members
FROM quel_member
WHERE referral_service_code IS NOT NULL;
```

### 평균 보유 크레딧
```sql
SELECT AVG(quel_member_credit) as avg_credits FROM quel_member;
```

---

Last Updated: 2025-11-26