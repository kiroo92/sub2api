package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PackageJob is durable creation-time attribution, not a credential snapshot.
type PackageJob struct {
	ID           string                   `json:"id"`
	UserID       int64                    `json:"user_id"`
	APIKeyID     int64                    `json:"api_key_id"`
	Kind         string                   `json:"kind"`
	ResourceID   string                   `json:"resource_id,omitempty"`
	AccountID    int64                    `json:"account_id"`
	Group        *Group                   `json:"group"`
	Selection    *PackageSelection        `json:"selection"`
	VideoPending *GrokVideoPendingBilling `json:"video_pending,omitempty"`
}

func (s *APIKeyService) PreparePackageJob(ctx context.Context, key *APIKey, kind string, accountID int64) (*PackageJob, error) {
	if !key.UsesPackages() || key.User == nil || key.Group == nil || key.PackageSelection == nil || s.packageService == nil {
		return nil, ErrPackageSelectionRequired
	}
	group := *key.Group
	group.AccountGroups = nil // Never journal account credentials through hydrated edges.
	job := &PackageJob{ID: uuid.NewString(), UserID: key.UserID, APIKeyID: key.ID, Kind: kind, AccountID: accountID, Group: &group, Selection: key.PackageSelection}
	if err := validatePackageJob(job); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	// Check the local recovery volume before starting any upstream work.
	probe, err := newPackageJobRecoveryFile()
	if err != nil {
		return nil, err
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())
	result, err := s.packageService.client.ExecContext(ctx, `INSERT INTO package_jobs(id,user_id,api_key_id,kind,attribution)
SELECT $1,$2,$3,$4,$5 WHERE EXISTS(SELECT 1 FROM package_periods pp JOIN user_packages p ON p.id=pp.package_id
WHERE pp.id=$6 AND p.id=$7 AND p.user_id=$2 AND p.group_id=$8)`, job.ID, key.UserID, key.ID, kind, string(raw), job.Selection.PeriodID, job.Selection.PackageID, job.Group.ID)
	if err == nil {
		var n int64
		n, err = result.RowsAffected()
		if err == nil && n != 1 {
			err = ErrPackageSelectionRequired
		}
	}
	if err == nil {
		// Detach the snapshot from request-local mutable group/selection pointers.
		var snapshot PackageJob
		err = json.Unmarshal(raw, &snapshot)
		job = &snapshot
	}
	return job, err
}

func (s *APIKeyService) CompletePackageJob(ctx context.Context, key *APIKey, job *PackageJob, resourceID string) error {
	if !key.UsesPackages() || job == nil || resourceID == "" || job.UserID != key.UserID || job.APIKeyID != key.ID || s.packageService == nil {
		return errors.New("package job response identity is missing")
	}
	if job.ResourceID != "" && job.ResourceID != resourceID {
		return errors.New("package job response identity changed")
	}
	job.ResourceID = resourceID
	if err := validatePackageJob(job); err != nil {
		return err
	}
	raw, err := json.Marshal(job)
	if err != nil {
		return err
	}
	// Journal the returned ID first. A database outage must never lead to replaying
	// an already-created upstream video/call. No request body or credential is stored.
	path, journalErr := writePackageJobRecovery(job, raw)
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.packageService.persistPackageJob(saveCtx, job, raw); err != nil {
		if journalErr != nil {
			return errors.Join(err, journalErr)
		}
		slog.Warn("package job completion retained for recovery", "job_id", job.ID, "error", err)
		return nil // The local durable record is retried by polling and the maintenance sweep.
	}
	if journalErr == nil {
		_ = os.Remove(path)
	}
	return nil
}

func (s *APIKeyService) RestorePackageJob(ctx context.Context, key *APIKey, kind, resourceID string) (*APIKey, error) {
	if !key.UsesPackages() || key.User == nil || s.packageService == nil || resourceID == "" {
		return nil, ErrPackageSelectionRequired
	}
	var raw []byte
	readErr := packageScan(ctx, s.packageService.client, `SELECT attribution FROM package_jobs WHERE user_id=$1 AND api_key_id=$2 AND kind=$3 AND resource_id=$4`, []any{key.UserID, key.ID, kind, resourceID}, &raw)
	if readErr != nil {
		path := packageJobRecoveryPath(key.UserID, key.ID, kind, resourceID)
		var err error
		raw, err = os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			if errors.Is(readErr, sql.ErrNoRows) {
				// A different replica may still be recovering the returned ID.
				// A pending owner draft is inconclusive, not proof of a missing task.
				var pending bool
				if err := packageScan(ctx, s.packageService.client, `SELECT EXISTS(SELECT 1 FROM package_jobs WHERE user_id=$1 AND api_key_id=$2 AND kind=$3 AND resource_id IS NULL)`, []any{key.UserID, key.ID, kind}, &pending); err != nil {
					return nil, err
				}
				if pending {
					return nil, ErrPackageSelectionRequired
				}
			}
			return nil, readErr
		}
	}
	var job PackageJob
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, err
	}
	if validatePackageJob(&job) != nil || job.UserID != key.UserID || job.APIKeyID != key.ID || job.Kind != kind || job.ResourceID != resourceID {
		return nil, ErrPackageSelectionRequired
	}
	if job.Kind == "grok_video" && job.VideoPending == nil {
		return nil, ErrPackageSelectionRequired
	}
	if readErr != nil {
		if err := s.packageService.persistPackageJob(ctx, &job, raw); err != nil {
			return nil, err // Retain the journal; never invent attribution while storage is unavailable.
		}
		_ = os.Remove(packageJobRecoveryPath(key.UserID, key.ID, kind, resourceID))
	}
	copyKey, copyUser := *key, *key.User
	copyUser.UserGroupRPMOverride = nil
	copyKey.User, copyKey.Group, copyKey.GroupID = &copyUser, job.Group, &job.Group.ID
	copyKey.PackageSelection, copyKey.PackageJob = job.Selection, &job
	return &copyKey, nil
}

func validatePackageJob(job *PackageJob) error {
	if job == nil || job.UserID <= 0 || job.APIKeyID <= 0 || job.Group == nil || job.Selection == nil || job.Selection.UserID != job.UserID || job.Selection.GroupID != job.Group.ID || job.Selection.PackageID <= 0 || job.Selection.PeriodID <= 0 {
		return ErrPackageSelectionRequired
	}
	if _, err := uuid.Parse(job.ID); err != nil {
		return err
	}
	if job.Kind != "image" && job.ResourceID != "" && job.AccountID <= 0 {
		return ErrPackageSelectionRequired
	}
	switch job.Kind {
	case "image", "grok_video", "live":
		return nil
	default:
		return errors.New("unknown package job kind")
	}
}

func (s *PackageService) persistPackageJob(ctx context.Context, job *PackageJob, raw []byte) error {
	result, err := s.client.ExecContext(ctx, `UPDATE package_jobs SET resource_id=$4,attribution=$5
WHERE id=$1 AND user_id=$2 AND api_key_id=$3 AND kind=$6 AND (resource_id IS NULL OR resource_id=$4)
AND attribution->'selection'=$5::jsonb->'selection' AND attribution->'group'=$5::jsonb->'group'`, job.ID, job.UserID, job.APIKeyID, job.ResourceID, string(raw), job.Kind)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return sql.ErrNoRows
	}
	return err
}

func packageJobRecoveryDir() string {
	dir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dir == "" {
		dir = "/app/data"
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			dir = "data"
		}
	}
	return filepath.Join(dir, "package-job-recovery")
}

func packageJobRecoveryPath(userID, keyID int64, kind, resourceID string) string {
	digest := sha256.Sum256([]byte(kind + "\x00" + resourceID))
	return filepath.Join(packageJobRecoveryDir(), fmt.Sprintf("%d-%d-%x.json", userID, keyID, digest))
}

func newPackageJobRecoveryFile() (*os.File, error) {
	dir := packageJobRecoveryDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.CreateTemp(dir, ".pending-")
}

func writePackageJobRecovery(job *PackageJob, raw []byte) (string, error) {
	f, err := newPackageJobRecoveryFile()
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if _, err := f.Write(raw); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	path := packageJobRecoveryPath(job.UserID, job.APIKeyID, job.Kind, job.ResourceID)
	if err := os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	// Unix needs the directory entry synced as well; Windows cannot sync a
	// directory handle through os.File. The file itself was synced above.
	if runtime.GOOS != "windows" {
		dir, err := os.Open(filepath.Dir(path))
		if err != nil {
			return path, err
		}
		defer func() { _ = dir.Close() }()
		if err := dir.Sync(); err != nil {
			return path, err
		}
	}
	return path, nil
}

// RecoverJobs retries only metadata writes, never upstream generation. The data
// volume must survive restart. Each replica sweeps its own records; polling also
// retries immediately when it reaches the replica holding the recovery record.
// ponytail: local recovery depends on the retained data volume; use shared durable
// storage if instances are routinely replaced without preserving that volume.
func (s *PackageService) RecoverJobs(ctx context.Context) error {
	dir, err := os.Open(packageJobRecoveryDir())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	entries, err := dir.ReadDir(100)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(packageJobRecoveryDir(), entry.Name())
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var job PackageJob
		if err := json.Unmarshal(raw, &job); err != nil {
			return err
		}
		if err := validatePackageJob(&job); err != nil {
			return err
		}
		if job.ResourceID == "" || path != packageJobRecoveryPath(job.UserID, job.APIKeyID, job.Kind, job.ResourceID) {
			return errors.New("invalid package recovery identity")
		}
		if err := s.persistPackageJob(ctx, &job, raw); err != nil {
			return err
		}
		_ = os.Remove(path)
	}
	return nil
}
