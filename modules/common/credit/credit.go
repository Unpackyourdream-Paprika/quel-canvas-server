package credit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/supabase-community/supabase-go"
	"quel-canvas-server/modules/common/config"
)

type Client struct {
	supabase *supabase.Client
}

// NewClient - Credit 클라이언트 생성
func NewClient() *Client {
	cfg := config.GetConfig()

	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey, &supabase.ClientOptions{})
	if err != nil {
		log.Printf("❌ Failed to create Supabase client: %v", err)
		return nil
	}

	return &Client{
		supabase: supabaseClient,
	}
}

// DeductCredits - 크레딧 차감 및 트랜잭션 기록
func (c *Client) DeductCredits(ctx context.Context, userID string, productionID string, attachIds []int) error {
	cfg := config.GetConfig()
	creditsPerImage := cfg.ImagePerPrice
	totalCredits := len(attachIds) * creditsPerImage

	log.Printf("💰 Deducting credits: User=%s, Images=%d, Total=%d credits", userID, len(attachIds), totalCredits)

	// 1. 현재 크레딧 조회
	var members []struct {
		QuelMemberCredit int `json:"quel_member_credit"`
	}

	data, _, err := c.supabase.From("quel_member").
		Select("quel_member_credit", "", false).
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to fetch user credits: %w", err)
	}

	if err := json.Unmarshal(data, &members); err != nil {
		return fmt.Errorf("failed to parse member data: %w", err)
	}

	if len(members) == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	currentCredits := members[0].QuelMemberCredit
	newBalance := currentCredits - totalCredits

	log.Printf("💰 Credit balance: %d → %d (-%d)", currentCredits, newBalance, totalCredits)

	// 2. 크레딧 차감
	_, _, err = c.supabase.From("quel_member").
		Update(map[string]interface{}{
			"quel_member_credit": newBalance,
		}, "", "").
		Eq("quel_member_id", userID).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to deduct credits: %w", err)
	}

	// 3. 각 이미지에 대해 트랜잭션 기록
	for _, attachID := range attachIds {
		transactionData := map[string]interface{}{
			"user_id":          userID,
			"transaction_type": "DEDUCT",
			"amount":           -creditsPerImage,
			"balance_after":    newBalance,
			"description":      "Generated With Image",
			"attach_idx":       attachID,
			"production_idx":   productionID,
		}

		_, _, err := c.supabase.From("quel_credits").
			Insert(transactionData, false, "", "", "").
			Execute()

		if err != nil {
			log.Printf("⚠️  Failed to record transaction for attach_id %d: %v", attachID, err)
		}
	}

	log.Printf("✅ Credits deducted successfully: %d credits from user %s", totalCredits, userID)
	return nil
}
