# Organization Tables
조직 관리 관련 테이블 (Organization Management)
---
## 📊 Overview
```
quel_member (기존)
    │
    ├──< quel_organization_member >──┤
    │                                │
    │                                ▼
    │                         quel_organization
    │                                │
    └──< quel_organization_invitation >──┘
```
**관계 요약:**
- 멤버 1명 → 여러 조직 가입 가능
- 조직 1개 → 여러 멤버 보유
- 초대는 멤버 ↔ 조직 연결 + 초대자 추적
---
## 1. quel_organization
조직 본체 정보 테이블
### 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| org_id | uuid | NO | gen_random_uuid() | 조직 고유 ID (PK) |
| org_name | text | NO | - | 조직 이름 |
| org_description | text | YES | - | 조직 설명 |
| org_logo_attach_id | bigint | YES | - | 로고 이미지 (FK → quel_attach) |
| org_credit | bigint | YES | 0 | 조직 공용 크레딧 |
| owner_id | uuid | NO | - | 생성자/소유자 (FK → quel_member) |
| org_status | text | NO | 'active' | 상태 (active/inactive/deleted) |
| max_members | int | YES | - | 최대 멤버 수 제한 |
| created_at | timestamptz | NO | now() | 생성 시간 |
| updated_at | timestamptz | NO | now() | 수정 시간 |
| deleted_at | timestamptz | YES | - | 삭제 시간 (soft delete) |

### 🔗 Relationships

**Foreign Keys:**
- `owner_id` → `quel_member.quel_member_id`
- `org_logo_attach_id` → `quel_attach.attach_id` (ON DELETE SET NULL)

**Referenced By:**
- `quel_organization_member.org_id`
- `quel_organization_invitation.org_id`
- `quel_credits.org_id`

### 🎯 Purpose

- 팀/회사/그룹 단위 리소스 공유
- 조직 단위 크레딧 관리 (공동 결제, 공동 사용)
- 멤버들을 하나의 그룹으로 묶음

### 📝 Code Examples

#### 조직 생성
```typescript
const { data: org } = await supabaseAdmin()
  .from("quel_organization")
  .insert({
    org_name: "우리팀",
    owner_id: memberId,
  })
  .select()
  .single();
```

#### 조직 정보 조회
```typescript
const { data: org } = await supabaseAdmin()
  .from("quel_organization")
  .select("*, owner:quel_member!owner_id(*)")
  .eq("org_id", orgId)
  .single();
```

---

## 2. quel_organization_member

조직-멤버 관계 테이블 (다대다 중간 테이블)

### 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | uuid | NO | gen_random_uuid() | 관계 고유 ID (PK) |
| org_id | uuid | NO | - | 조직 ID (FK → quel_organization) |
| member_id | uuid | NO | - | 멤버 ID (FK → quel_member) |
| role | text | NO | 'member' | 역할 (owner/admin/member) |
| status | text | NO | 'active' | 상태 (active/left/banned) |
| invited_by | uuid | YES | - | 초대한 멤버 (FK → quel_member) |
| joined_at | timestamptz | YES | - | 가입 승인 시간 |
| created_at | timestamptz | NO | now() | 레코드 생성 시간 |
| updated_at | timestamptz | NO | now() | 수정 시간 |

### 🔗 Relationships

**Foreign Keys:**
- `org_id` → `quel_organization.org_id`
- `member_id` → `quel_member.quel_member_id`
- `invited_by` → `quel_member.quel_member_id`

**Constraints:**
- `UNIQUE(org_id, member_id)` - 한 조직에 같은 멤버 중복 불가

### 🎭 Roles

| Role | 권한 |
|------|------|
| owner | 모든 권한 (삭제, 양도, 멤버 관리, 크레딧 관리) |
| admin | 멤버 초대/추방, 크레딧 사용 |
| member | 조직 크레딧 사용만 가능 |

### 📝 Code Examples

#### 조직 생성 시 owner 추가
```typescript
await supabaseAdmin()
  .from("quel_organization_member")
  .insert({
    org_id: org.org_id,
    member_id: memberId,
    role: "owner",
    joined_at: new Date().toISOString(),
  });
```

#### 멤버가 속한 조직 목록 조회
```typescript
const { data: orgs } = await supabaseAdmin()
  .from("quel_organization_member")
  .select("*, organization:quel_organization(*)")
  .eq("member_id", memberId)
  .eq("status", "active");
```

#### 조직의 멤버 목록 조회
```typescript
const { data: members } = await supabaseAdmin()
  .from("quel_organization_member")
  .select("*, member:quel_member(*)")
  .eq("org_id", orgId)
  .eq("status", "active");
```

---

## 3. quel_organization_invitation

조직 초대 관리 테이블

### 📋 Table Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| invitation_id | uuid | NO | gen_random_uuid() | 초대 ID (PK) |
| org_id | uuid | NO | - | 조직 ID (FK → quel_organization) |
| inviter_id | uuid | NO | - | 초대한 멤버 (FK → quel_member) |
| invitee_email | text | NO | - | 초대받는 이메일 |
| invitee_id | uuid | YES | - | 초대받는 멤버 ID (가입자인 경우) |
| role | text | NO | 'member' | 부여할 역할 (admin/member) |
| status | text | NO | 'pending' | 상태 (pending/accepted/rejected/expired) |
| token | text | YES | - | 초대 링크용 토큰 |
| expires_at | timestamptz | YES | - | 만료 시간 |
| responded_at | timestamptz | YES | - | 응답 시간 |
| created_at | timestamptz | NO | now() | 초대 발송 시간 |

### 🔗 Relationships

**Foreign Keys:**
- `org_id` → `quel_organization.org_id`
- `inviter_id` → `quel_member.quel_member_id`
- `invitee_id` → `quel_member.quel_member_id`

### 📊 Status Flow
```
pending → accepted (수락)
        → rejected (거절)
        → expired (기간 만료)
```

### 📝 Code Examples

#### 초대 생성
```typescript
const { data: invitation } = await supabaseAdmin()
  .from("quel_organization_invitation")
  .insert({
    org_id: orgId,
    inviter_id: inviterId,
    invitee_email: email,
    role: "member",
    token: crypto.randomUUID(),
    expires_at: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(), // 7일 후
  })
  .select()
  .single();
```

#### 초대 수락
```typescript
// 1. invitation 상태 업데이트
await supabaseAdmin()
  .from("quel_organization_invitation")
  .update({
    status: "accepted",
    responded_at: new Date().toISOString(),
  })
  .eq("invitation_id", invitationId);

// 2. organization_member에 추가
await supabaseAdmin()
  .from("quel_organization_member")
  .insert({
    org_id: invitation.org_id,
    member_id: memberId,
    role: invitation.role,
    invited_by: invitation.inviter_id,
    joined_at: new Date().toISOString(),
  });
```

#### 대기 중인 초대 조회
```typescript
const { data: pending } = await supabaseAdmin()
  .from("quel_organization_invitation")
  .select("*, organization:quel_organization(*)")
  .eq("invitee_email", userEmail)
  .eq("status", "pending")
  .gt("expires_at", new Date().toISOString());
```

---

## 4. quel_credits (업데이트)

### 추가된 컬럼

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| org_id | uuid | YES | 조직 ID (FK → quel_organization) |
| used_by_member_id | uuid | YES | 조직 크레딧 사용 시 실제 사용자 |

### 🎯 사용 패턴

| user_id | org_id | used_by_member_id | 의미 |
|---------|--------|-------------------|------|
| ✓ | NULL | NULL | 개인 크레딧 거래 |
| NULL | ✓ | ✓ | 조직 크레딧 거래 |

### 📝 Code Examples

#### 조직 크레딧 차감
```typescript
// 1. 조직 크레딧 차감
await supabaseAdmin()
  .from("quel_organization")
  .update({
    org_credit: org.org_credit - amount
  })
  .eq("org_id", orgId);

// 2. 거래 기록
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

---

## 🔄 전체 흐름 예시
```
1. A가 조직 "우리팀" 생성
   → quel_organization INSERT (owner_id = A)
   → quel_organization_member INSERT (member_id = A, role = owner)

2. A가 B를 초대
   → quel_organization_invitation INSERT (status = pending)

3. B가 수락
   → quel_organization_invitation UPDATE (status = accepted)
   → quel_organization_member INSERT (member_id = B, role = member)

4. B가 조직 크레딧 사용
   → quel_organization UPDATE (org_credit 차감)
   → quel_credits INSERT (org_id, used_by_member_id = B)
```

---

## ⚠️ Important Notes

1. **owner는 조직당 1명** - 양도 시 기존 owner를 admin으로 변경
2. **soft delete** - 탈퇴 시 `status = 'left'`로 이력 보존
3. **초대 만료** - `expires_at` 체크 필요, 만료된 초대는 무효
4. **크레딧 분리** - 개인/조직 크레딧은 완전히 별개로 관리

---

Last Updated: 2025-11-26
