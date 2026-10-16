package service

import "time"

type LuckyDrawTotals struct {
	RechargeAmount float64 `json:"recharge_amount"`
	ChancesEarned  int64   `json:"chances_earned"`
	ManualChances  int64   `json:"manual_chances"`
	DrawsUsed      int64   `json:"draws_used"`
	Chances        int64   `json:"chances"`
	Wins           int64   `json:"wins"`
	PrizeAmount    float64 `json:"prize_amount"`
}
type LuckyDrawUserStats struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	LuckyDrawTotals
}
type LuckyDrawPrizeStats struct {
	Name         string  `json:"name"`
	Amount       float64 `json:"amount"`
	Draws        int64   `json:"draws"`
	TotalAwarded float64 `json:"total_awarded"`
}
type LuckyDrawAdminStats struct {
	RechargeStartsAt time.Time             `json:"recharge_starts_at"`
	Summary          LuckyDrawTotals       `json:"summary"`
	Users            []LuckyDrawUserStats  `json:"users"`
	Prizes           []LuckyDrawPrizeStats `json:"prizes"`
	Total            int64                 `json:"total"`
	Page             int                   `json:"page"`
	PageSize         int                   `json:"page_size"`
	Grants           []LuckyDrawGrant      `json:"grants"`
}
type LuckyDrawGrant struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	AdminID   int64     `json:"admin_id"`
	Chances   int       `json:"chances"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}
