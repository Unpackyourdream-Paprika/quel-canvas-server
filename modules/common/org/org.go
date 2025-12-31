package org

import (
	"encoding/json"
	"log"

	"github.com/supabase-community/supabase-go"
)

// OrgStatus 상수
const (
	StatusActive    = "active"
	StatusPending   = "pending"
	StatusInactive  = "inactive"
	StatusSuspended = "suspended"
	StatusDeleted   = "deleted"
)

// IsOrgActive - 조직이 active 상태인지 확인
// orgID가 nil이거나 빈 문자열이면 false 반환
// 조직이 active 상태이면 true, 아니면 false 반환
func IsOrgActive(supabase *supabase.Client, orgID *string) bool {
	if orgID == nil || *orgID == "" {
		return false
	}

	var orgs []struct {
		OrgStatus string `json:"org_status"`
	}

	data, _, err := supabase.From("quel_organization").
		Select("org_status", "", false).
		Eq("org_id", *orgID).
		Execute()

	if err != nil {
		log.Printf("⚠️ [Org] Failed to check org_status for %s: %v", *orgID, err)
		return false
	}

	if err := json.Unmarshal(data, &orgs); err != nil {
		log.Printf("⚠️ [Org] Failed to parse org data for %s: %v", *orgID, err)
		return false
	}

	if len(orgs) == 0 {
		log.Printf("⚠️ [Org] Organization not found: %s", *orgID)
		return false
	}

	if orgs[0].OrgStatus == StatusActive {
		log.Printf("✅ [Org] Organization %s is active", *orgID)
		return true
	}

	log.Printf("⚠️ [Org] Organization %s status is '%s' (not active)", *orgID, orgs[0].OrgStatus)
	return false
}

// ShouldUseOrgCredit - 조직 크레딧을 사용해야 하는지 판단
// 조직이 존재하고 active 상태이면 true 반환
func ShouldUseOrgCredit(supabase *supabase.Client, orgID *string) bool {
	return IsOrgActive(supabase, orgID)
}
