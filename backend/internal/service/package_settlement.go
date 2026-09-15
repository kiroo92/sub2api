package service

import (
	"context"
	"log/slog"
	"time"
)

// StartSettlement runs independently of payment availability. Only delivered
// memberships and the group's snapshotted deadline/tier are relevant here.
func (s *PackageService) StartSettlement() {
	s.settlementOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.settlementCancel = cancel
		s.settlementWG.Add(1)
		go func() {
			defer s.settlementWG.Done()
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
				err := s.SettleExpiredGroups(checkCtx)
				checkCancel()
				if err != nil && ctx.Err() == nil {
					slog.Warn("package group settlement failed", "error", err)
				}
				recoveryCtx, recoveryCancel := context.WithTimeout(ctx, 30*time.Second)
				recoveryErr := s.RecoverJobs(recoveryCtx)
				recoveryCancel()
				if recoveryErr != nil && ctx.Err() == nil {
					slog.Warn("package task attribution recovery failed", "error", recoveryErr)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}

func (s *PackageService) Stop() {
	if s.settlementCancel != nil {
		s.settlementCancel()
	}
	s.settlementWG.Wait()
}

func (s *PackageService) SettleExpiredGroups(ctx context.Context) error {
	rows, err := s.client.QueryContext(ctx, `SELECT id FROM package_group_buys WHERE status='open' AND ends_at<=NOW() ORDER BY ends_at,id LIMIT 100`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.settleExpiredGroup(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *PackageService) settleExpiredGroup(ctx context.Context, id int64) error {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	group, err := packageLockedGroup(ctx, tx.Client(), id)
	if err != nil {
		return err
	}
	if group.Status == "open" && group.EndsAt != nil && !time.Now().Before(*group.EndsAt) {
		if err := settlePackageGroup(ctx, tx.Client(), group); err != nil {
			return err
		}
	}
	return tx.Commit()
}
