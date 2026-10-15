package repository

import (
	"context"
	"database/sql"
	"encoding/json"
)

func (r *teamRepository) SetModels(ctx context.Context, owner, member int64, models []string) error {
	if models == nil {
		models = []string{}
	}
	return r.transaction(ctx, func(tx *sql.Tx) error {
		t, _, err := teamActor(ctx, tx, owner, true)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(models)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE team_members SET allowed_models=$3 WHERE team_id=$1 AND user_id=$2 AND active`, t.ID, member, string(encoded))
		return teamAffected(res, err)
	})
}
