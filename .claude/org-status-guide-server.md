# Organization 상태 관리 가이드 - 서비스 서버 백엔드 (Go)

## 개요

이미지 생산 등 크레딧을 사용하는 Go 백엔드 서버에서는 **반드시 조직(Organization) 상태를 확인**한 후 크레딧을 차감해야 합니다.

---

## org_status 상태값

| 상태 | 설명 | 크레딧 차감 |
|------|------|------------|
| `active` | 활성 상태 | **허용** |
| `pending` | 대기 상태 (결제 대기 등) | **거부** |
| `inactive` | 비활성 상태 | **거부** |
| `suspended` | 정지 상태 | **거부** |
| `deleted` | 삭제됨 | **거부** |

**중요: `active` 상태인 조직만 크레딧 사용이 가능합니다.**

---

## 구현 가이드

### 1. Organization 구조체 정의

```go
type Organization struct {
    OrgID              string  `json:"org_id" db:"org_id"`
    OrgName            string  `json:"org_name" db:"org_name"`
    OrgStatus          string  `json:"org_status" db:"org_status"`
    OrgCredit          int64   `json:"org_credit" db:"org_credit"`
    IsEnterprise       bool    `json:"is_enterprise" db:"is_enterprise"`
    EnterpriseTier     *string `json:"enterprise_tier" db:"enterprise_tier"`
    MaxMembers         *int    `json:"max_members" db:"max_members"`
    CreatedAt          string  `json:"created_at" db:"created_at"`
    UpdatedAt          string  `json:"updated_at" db:"updated_at"`
}

// 상태 상수 정의
const (
    OrgStatusActive    = "active"
    OrgStatusPending   = "pending"
    OrgStatusInactive  = "inactive"
    OrgStatusSuspended = "suspended"
    OrgStatusDeleted   = "deleted"
)
```

### 2. 조직 상태 확인 함수

```go
package organization

import (
    "database/sql"
    "errors"
    "fmt"
)

var (
    ErrOrgNotFound     = errors.New("organization not found")
    ErrOrgNotActive    = errors.New("organization is not active")
    ErrInsufficientCredit = errors.New("insufficient credit")
)

// GetOrganization 조직 정보 조회
func GetOrganization(db *sql.DB, orgID string) (*Organization, error) {
    query := `
        SELECT org_id, org_name, org_status, org_credit,
               is_enterprise, enterprise_tier, max_members,
               created_at, updated_at
        FROM quel_organization
        WHERE org_id = $1
    `

    var org Organization
    err := db.QueryRow(query, orgID).Scan(
        &org.OrgID, &org.OrgName, &org.OrgStatus, &org.OrgCredit,
        &org.IsEnterprise, &org.EnterpriseTier, &org.MaxMembers,
        &org.CreatedAt, &org.UpdatedAt,
    )

    if err == sql.ErrNoRows {
        return nil, ErrOrgNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get organization: %w", err)
    }

    return &org, nil
}

// IsOrgActive 조직이 활성 상태인지 확인
func IsOrgActive(org *Organization) bool {
    return org.OrgStatus == OrgStatusActive
}

// ValidateOrgForCreditUsage 크레딧 사용 전 조직 유효성 검증
func ValidateOrgForCreditUsage(db *sql.DB, orgID string, requiredCredit int64) (*Organization, error) {
    // 1. 조직 조회
    org, err := GetOrganization(db, orgID)
    if err != nil {
        return nil, err
    }

    // 2. 상태 확인 - active가 아니면 거부
    if !IsOrgActive(org) {
        return nil, fmt.Errorf("%w: current status is %s", ErrOrgNotActive, org.OrgStatus)
    }

    // 3. 크레딧 잔액 확인
    if org.OrgCredit < requiredCredit {
        return nil, fmt.Errorf("%w: has %d, needs %d",
            ErrInsufficientCredit, org.OrgCredit, requiredCredit)
    }

    return org, nil
}
```

### 3. 크레딧 차감 로직 (이미지 생산 시)

```go
package credit

import (
    "database/sql"
    "fmt"
    "time"
)

type CreditTransaction struct {
    TransactionID   string `json:"transaction_id"`
    OrgID           string `json:"org_id"`
    Amount          int64  `json:"amount"`
    TransactionType string `json:"transaction_type"`
    Description     string `json:"description"`
    CreatedAt       string `json:"created_at"`
}

// DeductCredit 크레딧 차감 (이미지 생산 등)
func DeductCredit(db *sql.DB, orgID string, amount int64, description string) error {
    // 1. 조직 상태 및 크레딧 검증
    org, err := ValidateOrgForCreditUsage(db, orgID, amount)
    if err != nil {
        return err
    }

    // 2. 트랜잭션 시작
    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    // 3. 크레딧 차감 (비관적 락 사용)
    updateQuery := `
        UPDATE quel_organization
        SET org_credit = org_credit - $1,
            updated_at = $2
        WHERE org_id = $3
          AND org_status = 'active'
          AND org_credit >= $1
        RETURNING org_credit
    `

    var newCredit int64
    err = tx.QueryRow(updateQuery, amount, time.Now().UTC(), orgID).Scan(&newCredit)
    if err == sql.ErrNoRows {
        // 동시성 문제로 조건이 맞지 않음 (상태 변경 또는 크레딧 부족)
        return errors.New("credit deduction failed: organization state changed or insufficient credit")
    }
    if err != nil {
        return fmt.Errorf("failed to deduct credit: %w", err)
    }

    // 4. 트랜잭션 기록 생성
    insertQuery := `
        INSERT INTO quel_credit_transaction
        (org_id, amount, transaction_type, description, balance_after, created_at)
        VALUES ($1, $2, 'usage', $3, $4, $5)
    `
    _, err = tx.Exec(insertQuery, orgID, -amount, description, newCredit, time.Now().UTC())
    if err != nil {
        return fmt.Errorf("failed to record transaction: %w", err)
    }

    // 5. 커밋
    if err = tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}
```

### 4. 이미지 생산 핸들러 예시

```go
package handler

import (
    "encoding/json"
    "net/http"
)

type ImageGenerationRequest struct {
    OrgID       string `json:"org_id"`
    Prompt      string `json:"prompt"`
    ImageCount  int    `json:"image_count"`
}

type ImageGenerationResponse struct {
    Success bool     `json:"success"`
    Images  []string `json:"images,omitempty"`
    Error   string   `json:"error,omitempty"`
}

// 이미지 1장당 크레딧 비용
const CreditPerImage = 10

func HandleImageGeneration(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req ImageGenerationRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            respondError(w, http.StatusBadRequest, "Invalid request body")
            return
        }

        // 필요 크레딧 계산
        requiredCredit := int64(req.ImageCount * CreditPerImage)

        // ⚠️ 중요: 크레딧 사용 전 조직 상태 검증
        org, err := ValidateOrgForCreditUsage(db, req.OrgID, requiredCredit)
        if err != nil {
            switch {
            case errors.Is(err, ErrOrgNotFound):
                respondError(w, http.StatusNotFound, "Organization not found")
            case errors.Is(err, ErrOrgNotActive):
                respondError(w, http.StatusForbidden, "Organization is not active. Please contact administrator.")
            case errors.Is(err, ErrInsufficientCredit):
                respondError(w, http.StatusPaymentRequired, "Insufficient credit")
            default:
                respondError(w, http.StatusInternalServerError, "Internal server error")
            }
            return
        }

        // 크레딧 차감
        description := fmt.Sprintf("Image generation: %d images", req.ImageCount)
        if err := DeductCredit(db, req.OrgID, requiredCredit, description); err != nil {
            respondError(w, http.StatusInternalServerError, "Failed to deduct credit")
            return
        }

        // 이미지 생산 로직 실행
        images, err := generateImages(req.Prompt, req.ImageCount)
        if err != nil {
            // TODO: 실패 시 크레딧 환불 로직 구현
            respondError(w, http.StatusInternalServerError, "Image generation failed")
            return
        }

        respondJSON(w, http.StatusOK, ImageGenerationResponse{
            Success: true,
            Images:  images,
        })
    }
}

func respondError(w http.ResponseWriter, status int, message string) {
    respondJSON(w, status, ImageGenerationResponse{
        Success: false,
        Error:   message,
    })
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}
```

### 5. 미들웨어로 조직 상태 체크 (권장)

```go
package middleware

import (
    "context"
    "net/http"
)

type contextKey string

const OrgContextKey contextKey = "organization"

// OrgStatusMiddleware 조직 상태 체크 미들웨어
func OrgStatusMiddleware(db *sql.DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 헤더나 JWT에서 org_id 추출
            orgID := r.Header.Get("X-Organization-ID")
            if orgID == "" {
                http.Error(w, "Organization ID required", http.StatusBadRequest)
                return
            }

            // 조직 조회 및 상태 확인
            org, err := GetOrganization(db, orgID)
            if err != nil {
                if errors.Is(err, ErrOrgNotFound) {
                    http.Error(w, "Organization not found", http.StatusNotFound)
                    return
                }
                http.Error(w, "Internal server error", http.StatusInternalServerError)
                return
            }

            // 상태 확인 - active가 아니면 요청 거부
            if !IsOrgActive(org) {
                http.Error(w,
                    fmt.Sprintf("Organization is %s. Service unavailable.", org.OrgStatus),
                    http.StatusForbidden)
                return
            }

            // 컨텍스트에 조직 정보 저장
            ctx := context.WithValue(r.Context(), OrgContextKey, org)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// 핸들러에서 조직 정보 가져오기
func GetOrgFromContext(ctx context.Context) *Organization {
    org, ok := ctx.Value(OrgContextKey).(*Organization)
    if !ok {
        return nil
    }
    return org
}
```

---

## 에러 응답 코드 가이드

| HTTP Status | 상황 | 메시지 예시 |
|-------------|------|------------|
| 404 | 조직을 찾을 수 없음 | "Organization not found" |
| 403 | 조직이 active가 아님 | "Organization is not active. Please contact administrator." |
| 402 | 크레딧 부족 | "Insufficient credit" |
| 500 | 서버 내부 오류 | "Internal server error" |

---

## 엔터프라이즈 조직 추가 체크 (선택사항)

엔터프라이즈 조직은 계약 기간이 있으므로, 추가로 계약 만료 여부를 체크할 수 있습니다.

```go
// CheckEnterpriseContract 엔터프라이즈 계약 유효성 확인
func CheckEnterpriseContract(db *sql.DB, orgID string) error {
    query := `
        SELECT contract_end_date
        FROM quel_enterprise_company_info
        WHERE org_id = $1
    `

    var contractEndDate *time.Time
    err := db.QueryRow(query, orgID).Scan(&contractEndDate)
    if err == sql.ErrNoRows {
        // 엔터프라이즈 정보 없음 (일반 조직)
        return nil
    }
    if err != nil {
        return err
    }

    if contractEndDate != nil && contractEndDate.Before(time.Now()) {
        return errors.New("enterprise contract has expired")
    }

    return nil
}
```

---

## 체크리스트

- [ ] 크레딧 사용 전 `org_status` 확인 로직 구현
- [ ] `active` 상태가 아닌 경우 적절한 에러 응답 반환
- [ ] 크레딧 차감 시 트랜잭션 사용 (동시성 처리)
- [ ] 트랜잭션 기록 저장
- [ ] 에러 발생 시 크레딧 환불 로직 구현 (필요시)
- [ ] 미들웨어로 조직 상태 체크 자동화 (권장)

---

## 주의사항

1. **동시성 처리**: 크레딧 차감 시 반드시 데이터베이스 트랜잭션 사용
2. **이중 체크**: UPDATE 쿼리에서도 `org_status = 'active'` 조건 추가
3. **에러 핸들링**: 상태별 적절한 HTTP 상태 코드 반환
4. **로깅**: 크레딧 사용 실패 시 로그 기록 (추후 디버깅용)

---

Last Updated: 2025-12-31
