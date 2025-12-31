# QUEL 데이터베이스 테이블 구조

## 테이블 목록 (총 19개)

### 1. 회원 관련
| 테이블명 | 용도 |
|---------|------|
| `quel_member` | 회원 정보 |
| `quel_member_coupons` | 회원 쿠폰 |

### 2. 조직(Organization) 관련
| 테이블명 | 용도 |
|---------|------|
| `quel_organization` | 조직 정보 |
| `quel_organization_member` | 조직 멤버 관계 |
| `quel_organization_workspace` | 조직 워크스페이스 (무제한) |
| `organization_invite_token` | 조직 초대 토큰 |
| `organization_plan` | 조직 요금제 정보 |
| `organization_subscription` | 조직 구독 정보 |
| `organization_subscription_history` | 조직 구독 변경 이력 |

### 3. 크레딧/결제 관련
| 테이블명 | 용도 |
|---------|------|
| `payments` | 크레딧 충전 결제 내역 |
| `quel_credits` | 크레딧 사용 내역 |
| `quel_commission_rates` | 수수료율 |

### 4. 이미지/생성 관련
| 테이블명 | 용도 |
|---------|------|
| `quel_attach` | 이미지 첨부파일 |
| `quel_production_photo` | 생성 결과 이미지 |
| `quel_production_jobs` | 생성 작업(Job) |

### 5. 워크스페이스 관련
| 테이블명 | 용도 |
|---------|------|
| `quel_personal_workspace` | 개인 워크스페이스 (최대 3개) |

### 6. 파트너 관련
| 테이블명 | 용도 |
|---------|------|
| `quel_partners` | 파트너 정보 |
| `quel_partner_customers` | 파트너 고객 관계 |
| `quel_service_referral_code` | 서비스 추천 코드 |
| `partner_settlements` | 파트너 정산 내역 |

### 7. 기타
| 테이블명 | 용도 |
|---------|------|
| `quel_notifications` | 알림 |

---

## 테이블 관계도

```
quel_member (회원)
    ├── quel_member_coupons (1:N)
    ├── quel_credits (1:N) - 개인 크레딧 내역
    ├── quel_attach (1:N)
    ├── quel_production_photo (1:N)
    ├── quel_production_jobs (1:N)
    ├── quel_personal_workspace (1:N) - 최대 3개 제한
    ├── quel_organization_member (1:N) - 조직 멤버십
    ├── quel_partner_customers (1:1)
    └── quel_notifications (1:N)

quel_organization (조직)
    ├── quel_organization_member (1:N) - max_members로 제한
    ├── quel_organization_workspace (1:N) - 무제한
    ├── quel_credits (1:N) - 조직 크레딧 내역
    ├── payments (1:N) - 크레딧 충전 결제
    ├── quel_attach (1:N)
    ├── quel_production_photo (1:N)
    ├── quel_production_jobs (1:N)
    ├── organization_subscription (1:N) - 조직 구독
    ├── organization_subscription_history (1:N) - 구독 이력
    └── organization_invite_token (1:N) - 초대 토큰

organization_plan (요금제)
    └── organization_subscription (1:N)

quel_partners (파트너)
    ├── quel_partner_customers (1:N)
    ├── quel_service_referral_code (1:N)
    └── partner_settlements (1:N) - 정산 내역
```

---

## 조직 결제 흐름

```
[조직 생성/구독]
organization_plan (요금제 선택)
    ↓
organization_subscription (구독 생성, status: pending)
    ↓
Stripe Checkout Session 생성
    ↓
결제 완료 (webhook)
    ↓
organization_subscription (status: active)
    ↓
organization_subscription_history (이력 기록)

[멤버 초대] - 무료 (max_members 내에서)
quel_organization_member (멤버 추가, status: pending)
    ↓
organization_invite_token (초대 토큰 생성)
    ↓
이메일 발송
    ↓
초대 수락 시 멤버 status: active

[워크스페이스 추가] - 무제한
quel_organization_workspace (워크스페이스 생성)

[크레딧 충전]
Stripe Checkout Session 생성
    ↓
결제 완료 (webhook)
    ↓
payments (결제 레코드 생성)
    ↓
quel_organization.org_credit 증가

[크레딧 차감] - Go 서버 (이미지 생성 완료 시)
userID (memberId)로 시작
    ↓
quel_organization_member 테이블에서 해당 userID로 org_id 조회
    ↓
┌─────────────────────────────────────────────────┐
│ org_id가 있는가?                                 │
├─────────────────────────────────────────────────┤
│ YES → quel_organization에서 org_status 조회      │
│       ↓                                          │
│       ┌─────────────────────────────────────────┐│
│       │ org_status가 active인가?                ││
│       ├─────────────────────────────────────────┤│
│       │ YES → 조직 크레딧 차감                   ││
│       │       quel_organization.org_credit 차감 ││
│       │       quel_credits INSERT (org_id 포함) ││
│       │                                         ││
│       │ NO  → 개인 크레딧 차감                   ││
│       │       quel_member.quel_member_credit 차감││
│       │       quel_credits INSERT (user_id만)   ││
│       └─────────────────────────────────────────┘│
│                                                  │
│ NO  → 개인 크레딧 차감                            │
│       quel_member.quel_member_credit 차감        │
│       quel_credits INSERT (user_id만)            │
└─────────────────────────────────────────────────┘
    ↓
완료
```

---

## 주요 API와 테이블 매핑

| API 경로 | 사용 테이블 |
|---------|------------|
| `/api/auth/me` | quel_member, quel_organization_member |
| `/api/credits/deduct` | quel_organization_member, quel_organization, quel_credits, quel_member |
| `/api/credits/available` | quel_organization_member, quel_organization, quel_member, quel_production_jobs |
| `/api/jobs/create` | quel_organization_member, quel_organization, quel_member, quel_production_photo, quel_production_jobs |
| `/api/jobs/[jobId]` | quel_production_jobs, quel_attach |
| `/api/upload-image` | quel_attach |
| `/api/get-production` | quel_production_photo, quel_attach, quel_credits |
| `/api/organizations/[org_id]` | quel_organization, quel_organization_member, quel_organization_workspace |
| `/api/organizations/checkout-create` | quel_member, quel_organization, quel_organization_member, organization_plan, organization_subscription |
| `/api/organizations/[org_id]/invite` | quel_organization_member, quel_organization, quel_member, organization_invite_token |
| `/api/organizations/[org_id]/payment-history` | payments |
| `/api/organizations/[org_id]/workspace` | quel_organization_workspace, quel_organization_member |
| `/api/invite/accept` | organization_invite_token, quel_organization, quel_organization_member |
| `/api/stripe/checkout` | quel_member, quel_organization_member, quel_service_referral_code, quel_partners, quel_commission_rates |
| `/api/stripe/webhook` | organization_subscription, organization_subscription_history, payments, quel_organization, quel_member, partner_settlements |
| `/api/personal-workspace` | quel_personal_workspace |

---

## 조직 vs 개인 사용자 비교

| 항목 | 개인 사용자 | 조직 |
|-----|-----------|------|
| 워크스페이스 | 최대 3개 | 무제한 |
| 멤버 | 1명 (본인) | 플랜에 따라 max_members |
| 크레딧 | quel_member.quel_member_credit | quel_organization.org_credit |
| 결제 내역 | payments (user_id) | payments (org_id) |

---

## Go 서버 모듈별 크레딧 차감 로직 현황

### 조직 크레딧 지원 완료 (org_status 체크 필요)
| 모듈 | 파일 |
|------|------|
| beauty | `modules/beauty/service.go` |
| cartoon | `modules/cartoon/service.go` |
| cinema | `modules/cinema/service.go` |
| eats | `modules/eats/service.go` |
| fashion | `modules/fashion/service.go` |
| landing-demo | `modules/landing-demo/service.go` |
| flux-schnell | `modules/submodule/flux-schnell/service.go` |

### 조직 크레딧 미지원 (추후 적용 필요)
| 모듈 | 파일 | 비고 |
|------|------|------|
| modify | `modules/modify/service.go` | 개인 크레딧만 지원 |
| multiview | `modules/multiview/service.go` | 개인 크레딧만 지원 |
| studio | `modules/unified-prompt/studio/service.go` | 개인 크레딧만 지원 |
| generate-image | `modules/generate-image/service.go` | 개인 크레딧만 지원 |
| common/credit | `modules/common/credit/credit.go` | 공통 모듈 |

---

## 환경별 Supabase 설정

| 환경 | Supabase URL |
|-----|--------------|
| Production | `https://ftunegfzqsbhtucctaqq.supabase.co` |
| Development | `https://wgaylvfaicajrgcpibff.supabase.co` |
