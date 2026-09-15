package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// applyPackageUsageBilling owns an independent durable journal. Monetary effects
// use the same transaction/primitives as ordinary billing, with no old entitlement.
func (r *usageBillingRepository) applyPackageUsageBilling(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	if cmd.PackageID <= 0 || cmd.PackagePeriodID <= 0 || cmd.SubscriptionID != nil || cmd.SubscriptionCost != 0 || cmd.BalanceCost != 0 || cmd.PackageCost < 0 || math.IsNaN(cmd.PackageCost) || math.IsInf(cmd.PackageCost, 0) {
		return nil, service.ErrPackageSelectionRequired
	}
	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	// Commit the recoverable command before attempting monetary effects. A failed
	// transaction leaves a pending journal row; admission blocks pending owners.
	_, err = r.db.ExecContext(ctx, `INSERT INTO package_usage_billing(request_id,api_key_id,user_id,package_id,period_id,request_fingerprint,command)
	VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(request_id,api_key_id) DO NOTHING`, cmd.RequestID, cmd.APIKeyID, cmd.UserID, cmd.PackageID, cmd.PackagePeriodID, cmd.RequestFingerprint, payload)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var fingerprint string
	var applied bool
	if err = tx.QueryRowContext(ctx, `SELECT request_fingerprint,applied FROM package_usage_billing WHERE request_id=$1 AND api_key_id=$2 FOR UPDATE`, cmd.RequestID, cmd.APIKeyID).Scan(&fingerprint, &applied); err != nil {
		return nil, err
	}
	if fingerprint != cmd.RequestFingerprint {
		return nil, service.ErrUsageBillingRequestConflict
	}
	if applied {
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE package_periods pp SET used_usd=pp.used_usd+$1 FROM user_packages p
	WHERE pp.id=$2 AND pp.package_id=$3 AND p.id=pp.package_id AND p.user_id=$4
	AND EXISTS(SELECT 1 FROM api_keys k WHERE k.id=$5 AND k.user_id=$4)`, cmd.PackageCost, cmd.PackagePeriodID, cmd.PackageID, cmd.UserID, cmd.APIKeyID)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, sql.ErrNoRows
	}
	appliedResult := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, appliedResult); err != nil {
		return nil, err
	}
	result, err = tx.ExecContext(ctx, `UPDATE package_usage_billing SET applied=TRUE,applied_at=NOW() WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, cmd.APIKeyID)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("missing package billing journal update")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return appliedResult, nil
}
