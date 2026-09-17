package service

import "context"

type TeamAdminItem struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	OwnerID     int64   `json:"owner_id"`
	OwnerEmail  string  `json:"owner_email"`
	Status      string  `json:"status"`
	MemberCount int     `json:"member_count"`
	TotalUsage  float64 `json:"total_usage"`
}
type teamAdminScopeKey struct{}

func WithTeamAdminScope(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, teamAdminScopeKey{}, id)
}
func TeamMatchesAdminScope(ctx context.Context, id int64) bool {
	expected, ok := ctx.Value(teamAdminScopeKey{}).(int64)
	return !ok || expected == id
}
func (s *TeamService) AdminOwner(ctx context.Context, id int64) (int64, error) {
	return s.repo.AdminOwner(ctx, id)
}
func (s *TeamService) AdminList(ctx context.Context, search, status string, page, size int) ([]TeamAdminItem, int, error) {
	return s.repo.AdminList(ctx, search, status, page, size)
}
