package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Save separately from the debit transaction so a rolled-back debit remains retryable.
func (r *usageBillingRepository) saveTeamBill(ctx context.Context, cmd *service.UsageBillingCommand) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	var fingerprint string
	err = r.db.QueryRowContext(ctx, `INSERT INTO team_billing_pending(request_id,api_key_id,admission_id,fingerprint,command)
SELECT $1::text,$2::bigint,$3::varchar,$4::text,$5::jsonb FROM team_requests r JOIN team_members m ON m.id=r.member_id JOIN teams t ON t.id=m.team_id
WHERE r.id=$3 AND r.api_key_id=$2 AND m.id=$6 AND t.owner_id=$7 AND NOT r.closed
ON CONFLICT(request_id,api_key_id) DO UPDATE SET fingerprint=team_billing_pending.fingerprint
RETURNING fingerprint`, cmd.RequestID, cmd.APIKeyID, cmd.TeamRequestID, cmd.RequestFingerprint, string(data), cmd.TeamMemberID, cmd.UserID).Scan(&fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		// A response may be retried after its lease was safely released.
		err = r.db.QueryRowContext(ctx, `SELECT request_fingerprint FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2
UNION ALL SELECT request_fingerprint FROM usage_billing_dedup_archive WHERE request_id=$1 AND api_key_id=$2 LIMIT 1`, cmd.RequestID, cmd.APIKeyID).Scan(&fingerprint)
	}
	if err != nil {
		return err
	}
	if fingerprint != cmd.RequestFingerprint {
		return service.ErrUsageBillingRequestConflict
	}
	return nil
}

func deletePendingTeamBill(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	if cmd.TeamMemberID == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM team_billing_pending WHERE request_id=$1 AND api_key_id=$2 AND fingerprint=$3`, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
	return err
}

func (r *teamRepository) CloseRequest(ctx context.Context, id string, unresolved bool) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE team_requests SET closed=TRUE,unresolved=unresolved OR $2 WHERE id=$1`, id, unresolved); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM team_requests r WHERE id=$1 AND closed AND NOT unresolved AND NOT EXISTS(SELECT 1 FROM team_billing_pending p WHERE p.admission_id=r.id)`, id)
		return err
	})
}

func (r *teamRepository) RecoverBilling(ctx context.Context, userID int64) (int, error) {
	count := 0
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		team, _, err := teamActor(ctx, tx, userID, true)
		if err != nil {
			return err
		}
		// Only closed execution contexts can be retried; an idle WS may still produce more turns.
		rows, err := tx.QueryContext(ctx, `SELECT p.command FROM team_billing_pending p JOIN team_requests r ON r.id=p.admission_id JOIN team_members m ON m.id=r.member_id WHERE m.team_id=$1 AND r.closed ORDER BY p.created_at,p.request_id LIMIT 100 FOR UPDATE OF p,r`, team.ID)
		if err != nil {
			return err
		}
		var commands []service.UsageBillingCommand
		for rows.Next() {
			var data []byte
			var cmd service.UsageBillingCommand
			if err = rows.Scan(&data); err == nil {
				err = json.Unmarshal(data, &cmd)
			}
			if err != nil {
				rows.Close()
				return err
			}
			commands = append(commands, cmd)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		billing := &usageBillingRepository{db: r.db}
		for i := range commands {
			cmd := &commands[i]
			if cmd.UserID != userID {
				return service.ErrTeamForbidden
			}
			applied, err := billing.claimUsageBillingKey(ctx, tx, cmd)
			if err != nil {
				return err
			}
			if applied {
				if err = billing.applyUsageBillingEffects(ctx, tx, cmd, &service.UsageBillingApplyResult{}); err != nil {
					return err
				}
			}
			if cmd.TeamUsageLog != nil {
				log := *cmd.TeamUsageLog
				if log.UserID != cmd.UserID || log.APIKeyID != cmd.APIKeyID {
					return service.ErrTeamForbidden
				}
				log.ActualCost = service.QuantizeUsageBillingAmount(cmd.BalanceCost + cmd.SubscriptionCost)
				if _, err = (&usageLogRepository{}).createSingle(ctx, tx, &log); err != nil {
					return err
				}
				if _, err = tx.ExecContext(ctx, `UPDATE usage_logs SET actual_cost=$3 WHERE request_id=$1 AND api_key_id=$2`, log.RequestID, log.APIKeyID, log.ActualCost); err != nil {
					return err
				}
			}
			if err = deletePendingTeamBill(ctx, tx, cmd); err != nil {
				return err
			}
			count++
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM team_requests r USING team_members m WHERE r.member_id=m.id AND m.team_id=$1 AND r.closed AND NOT r.unresolved AND NOT EXISTS(SELECT 1 FROM team_billing_pending p WHERE p.admission_id=r.id)`, team.ID)
		return err
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
