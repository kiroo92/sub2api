package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *teamRepository) AdminOwner(ctx context.Context, id int64) (int64, error) {
	var owner int64
	err := r.db.QueryRowContext(ctx, `SELECT owner_id FROM teams WHERE id=$1`, id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrTeamForbidden
	}
	return owner, err
}
func (r *teamRepository) AdminList(ctx context.Context, search, status string, page, size int) ([]service.TeamAdminItem, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	result := []service.TeamAdminItem{}
	total := 0
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		const filter = ` FROM teams t JOIN users u ON u.id=t.owner_id WHERE ($1='' OR t.name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%') AND ($2='' OR t.status=$2)`
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*)`+filter, search, status).Scan(&total); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT t.id,t.name,t.owner_id,u.email,t.status,(SELECT COUNT(*) FROM team_members m WHERE m.team_id=t.id AND m.active),(SELECT COALESCE(SUM(total_used),0) FROM team_members m WHERE m.team_id=t.id)`+filter+` ORDER BY t.id DESC LIMIT $3 OFFSET $4`, search, status, size, (page-1)*size)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item service.TeamAdminItem
			if err = rows.Scan(&item.ID, &item.Name, &item.OwnerID, &item.OwnerEmail, &item.Status, &item.MemberCount, &item.TotalUsage); err != nil {
				return err
			}
			result = append(result, item)
		}
		return rows.Err()
	})
	return result, total, err
}
