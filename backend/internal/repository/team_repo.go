package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type teamRepository struct {
	db    *sql.DB
	redis *redis.Client
}

func NewTeamRepository(db *sql.DB, rdb *redis.Client) service.TeamRepository {
	return &teamRepository{db: db, redis: rdb}
}
func (r *teamRepository) transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func teamActor(ctx context.Context, tx *sql.Tx, userID int64, owner bool) (*service.Team, int64, error) {
	var t service.Team
	var member int64
	var groups []byte
	err := tx.QueryRowContext(ctx, `SELECT t.id,t.owner_id,t.name,t.status,t.group_ids,t.daily_limit,t.weekly_limit,t.monthly_limit,t.created_at,m.id FROM teams t JOIN team_members m ON m.team_id=t.id WHERE m.user_id=$1 AND m.active FOR UPDATE OF t,m`, userID).Scan(&t.ID, &t.OwnerID, &t.Name, &t.Status, &groups, &t.DefaultLimits.Daily, &t.DefaultLimits.Weekly, &t.DefaultLimits.Monthly, &t.CreatedAt, &member)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, service.ErrTeamForbidden
	}
	if err != nil {
		return nil, 0, err
	}
	if owner && t.OwnerID != userID {
		return nil, 0, service.ErrTeamForbidden
	}
	if !service.TeamMatchesAdminScope(ctx, t.ID) {
		return nil, 0, service.ErrTeamForbidden
	}
	if t.Status == "dissolving" {
		return nil, 0, service.ErrTeamUnavailable
	}
	if err = json.Unmarshal(groups, &t.GroupIDs); err != nil {
		return nil, 0, err
	}
	return &t, member, nil
}
func lockTeamUser(ctx context.Context, tx *sql.Tx, id int64) error {
	var got int64
	return tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&got)
}
func noActiveTeam(ctx context.Context, tx *sql.Tx, id int64) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM team_members WHERE user_id=$1 AND active)`, id).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return service.ErrTeamConflict
	}
	return nil
}
func (r *teamRepository) Create(ctx context.Context, id int64, name string) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		if err := lockTeamUser(ctx, tx, id); err != nil {
			return err
		}
		if err := noActiveTeam(ctx, tx, id); err != nil {
			return err
		}
		var team int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO teams(owner_id,name) VALUES($1,$2) RETURNING id`, id, name).Scan(&team); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO team_members(team_id,user_id) VALUES($1,$2)`, team, id)
		return err
	})
}
func (r *teamRepository) Snapshot(ctx context.Context, id int64) (*service.TeamSnapshot, error) {
	out := &service.TeamSnapshot{Members: []service.TeamMember{}, Invitations: []service.TeamInvitation{}, PendingInvitations: []service.TeamInvitation{}, AvailableGroups: []service.TeamGroup{}}
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		t, member, err := teamActor(ctx, tx, id, false)
		if err != nil && !service.TeamMatchesAdminScope(ctx, 0) {
			return err
		}
		if err != nil && !errors.Is(err, service.ErrTeamForbidden) && !errors.Is(err, service.ErrTeamUnavailable) {
			return err
		}
		if err == nil {
			out.Team = t
			if t.OwnerID == id {
				if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_requests r JOIN team_members m ON m.id=r.member_id WHERE m.team_id=$1`, t.ID).Scan(&out.PendingRequests); err != nil {
					return err
				}
				if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_billing_pending p JOIN team_requests r ON r.id=p.admission_id JOIN team_members m ON m.id=r.member_id WHERE m.team_id=$1 AND r.closed`, t.ID).Scan(&out.PendingBilling); err != nil {
					return err
				}
			}
			out.MyMemberID = member
			out.Role = "member"
			if t.OwnerID == id {
				out.Role = "owner"
			}
			rows, err := tx.QueryContext(ctx, `SELECT m.id,m.user_id,u.email,COALESCE(u.username,''),m.active,m.daily_limit,m.weekly_limit,m.monthly_limit,m.daily_used,m.weekly_used,m.monthly_used,m.total_used,m.daily_start,m.weekly_start,m.monthly_start,m.joined_at FROM team_members m JOIN users u ON u.id=m.user_id WHERE m.team_id=$1 ORDER BY m.id`, t.ID)
			if err != nil {
				return err
			}
			for rows.Next() {
				var m service.TeamMember
				var day, week, month *time.Time
				if err = rows.Scan(&m.ID, &m.UserID, &m.Email, &m.Username, &m.Active, &m.Limits.Daily, &m.Limits.Weekly, &m.Limits.Monthly, &m.Usage.Daily, &m.Usage.Weekly, &m.Usage.Monthly, &m.Usage.Total, &day, &week, &month, &m.JoinedAt); err != nil {
					rows.Close()
					return err
				}
				m.Role = "member"
				if m.UserID == t.OwnerID {
					m.Role = "owner"
				}
				normalizeTeamUsage(&m.Usage, day, week, month)
				m.Resets = service.TeamResets{Daily: m.Usage.DailyResetAt, Weekly: m.Usage.WeeklyResetAt, Monthly: m.Usage.MonthlyResetAt}
				if out.Role == "owner" || m.UserID == id {
					out.Usage.Daily += m.Usage.Daily
					out.Usage.Weekly += m.Usage.Weekly
					out.Usage.Monthly += m.Usage.Monthly
					out.Usage.Total += m.Usage.Total
					if m.Active {
						out.Members = append(out.Members, m)
					}
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		rows, err := tx.QueryContext(ctx, `SELECT i.id,i.team_id,t.name,i.email,i.status,i.expires_at,i.created_at FROM team_invitations i JOIN teams t ON t.id=i.team_id WHERE t.status!='dissolving' AND i.status='pending' AND (t.owner_id=$1 OR lower(i.email)=(SELECT lower(email) FROM users WHERE id=$1)) ORDER BY i.id`, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var i service.TeamInvitation
			if err := rows.Scan(&i.ID, &i.TeamID, &i.TeamName, &i.Email, &i.Status, &i.ExpiresAt, &i.CreatedAt); err != nil {
				return err
			}
			if out.Team != nil && out.Role == "owner" && i.TeamID == out.Team.ID {
				out.Invitations = append(out.Invitations, i)
			} else {
				out.PendingInvitations = append(out.PendingInvitations, i)
			}
		}
		return rows.Err()
	})
	return out, err
}
func normalizeTeamUsage(u *service.TeamUsage, day, week, month *time.Time) {
	for _, w := range []struct {
		start    *time.Time
		duration time.Duration
		used     *float64
		reset    **time.Time
	}{{day, 24 * time.Hour, &u.Daily, &u.DailyResetAt}, {week, 7 * 24 * time.Hour, &u.Weekly, &u.WeeklyResetAt}, {month, 30 * 24 * time.Hour, &u.Monthly, &u.MonthlyResetAt}} {
		if w.start == nil {
			*w.used = 0
			continue
		}
		reset := w.start.Add(w.duration)
		if w.duration == 24*time.Hour {
			reset = timezone.StartOfDay(*w.start).AddDate(0, 0, 1)
		}
		*w.reset = &reset
		if !time.Now().Before(reset) {
			*w.used = 0
		}
	}
}
func (r *teamRepository) Update(ctx context.Context, id int64, in service.TeamSettings) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		t, _, err := teamActor(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if in.Name != nil {
			t.Name = *in.Name
		}
		if in.Status != nil {
			t.Status = *in.Status
		}
		if in.GroupIDs != nil {
			t.GroupIDs = *in.GroupIDs
		}
		if in.DefaultLimits != nil {
			t.DefaultLimits = *in.DefaultLimits
		}
		groups, err := json.Marshal(t.GroupIDs)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE teams SET name=$2,status=$3,group_ids=$4,daily_limit=$5,weekly_limit=$6,monthly_limit=$7 WHERE id=$1`, t.ID, t.Name, t.Status, string(groups), t.DefaultLimits.Daily, t.DefaultLimits.Weekly, t.DefaultLimits.Monthly)
		return err
	})
}
func (r *teamRepository) Invite(ctx context.Context, id int64, email, hash string, inviteID int64) (*service.TeamInvitation, error) {
	var out service.TeamInvitation
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		t, _, err := teamActor(ctx, tx, id, true)
		if err != nil {
			return err
		}
		out.TeamID = t.ID
		out.TeamName = t.Name
		if inviteID != 0 {
			err = tx.QueryRowContext(ctx, `UPDATE team_invitations SET token_hash=$3,expires_at=NOW()+INTERVAL '7 days' WHERE id=$1 AND team_id=$2 AND status='pending' RETURNING id,email,status,expires_at,created_at`, inviteID, t.ID, hash).Scan(&out.ID, &out.Email, &out.Status, &out.ExpiresAt, &out.CreatedAt)
		} else {
			err = tx.QueryRowContext(ctx, `INSERT INTO team_invitations(team_id,email,token_hash,expires_at) VALUES($1,$2,$3,NOW()+INTERVAL '7 days') ON CONFLICT(team_id,email) WHERE status='pending' DO UPDATE SET token_hash=EXCLUDED.token_hash,expires_at=EXCLUDED.expires_at RETURNING id,email,status,expires_at,created_at`, t.ID, email, hash).Scan(&out.ID, &out.Email, &out.Status, &out.ExpiresAt, &out.CreatedAt)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrTeamForbidden
		}
		return err
	})
	return &out, err
}
func (r *teamRepository) RevokeInvite(ctx context.Context, id, invite int64) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		t, _, err := teamActor(ctx, tx, id, true)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE team_invitations SET status='revoked' WHERE id=$1 AND team_id=$2 AND status='pending'`, invite, t.ID)
		return teamAffected(res, err)
	})
}
func teamAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrTeamForbidden
	}
	return nil
}
func (r *teamRepository) Accept(ctx context.Context, id int64, hash string, invite int64) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		if err := lockTeamUser(ctx, tx, id); err != nil {
			return err
		}
		if err := noActiveTeam(ctx, tx, id); err != nil {
			return err
		}
		var team, inv int64
		var l service.TeamLimits
		err := tx.QueryRowContext(ctx, `SELECT t.id,i.id,t.daily_limit,t.weekly_limit,t.monthly_limit FROM team_invitations i JOIN teams t ON t.id=i.team_id JOIN users u ON lower(u.email)=i.email WHERE u.id=$1 AND (i.token_hash=$2 OR ($3>0 AND i.id=$3)) AND i.status='pending' AND i.expires_at>NOW() AND t.status!='dissolving' FOR UPDATE OF t,i`, id, hash, invite).Scan(&team, &inv, &l.Daily, &l.Weekly, &l.Monthly)
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrTeamConflict
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO team_members(team_id,user_id,daily_limit,weekly_limit,monthly_limit) VALUES($1,$2,$3,$4,$5) ON CONFLICT(team_id,user_id) DO UPDATE SET active=TRUE`, team, id, l.Daily, l.Weekly, l.Monthly)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE team_invitations SET status='accepted' WHERE id=$1`, inv)
		return err
	})
}
func (r *teamRepository) SetLimits(ctx context.Context, id, member int64, l service.TeamLimits) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		t, _, err := teamActor(ctx, tx, id, true)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE team_members SET daily_limit=$3,weekly_limit=$4,monthly_limit=$5 WHERE user_id=$1 AND team_id=$2 AND active`, member, t.ID, l.Daily, l.Weekly, l.Monthly)
		return teamAffected(res, err)
	})
}
func (r *teamRepository) Leave(ctx context.Context, id, member int64) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		t, _, err := teamActor(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if member == 0 {
			member = id
		}
		if member != id && t.OwnerID != id {
			return service.ErrTeamForbidden
		}
		res, err := tx.ExecContext(ctx, `UPDATE team_members SET active=FALSE WHERE user_id=$1 AND team_id=$2 AND user_id!=$3 AND active`, member, t.ID, t.OwnerID)
		if err = teamAffected(res, err); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE team_api_keys SET revoked=TRUE WHERE member_id IN (SELECT id FROM team_members WHERE user_id=$1 AND team_id=$2)`, member, t.ID)
		return err
	})
}
func (r *teamRepository) Keys(ctx context.Context, id int64) ([]service.TeamKey, error) {
	out := []service.TeamKey{}
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		t, m, err := teamActor(ctx, tx, id, false)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT k.id,m.id,m.user_id,k.name,k.key,k.status,k.quota_used,k.created_at,u.email,k.last_used_at FROM team_api_keys tk JOIN api_keys k ON k.id=tk.api_key_id JOIN team_members m ON m.id=tk.member_id JOIN users u ON u.id=m.user_id WHERE m.team_id=$1 AND ($2=$3 OR m.id=$4) AND NOT tk.revoked AND k.deleted_at IS NULL ORDER BY k.id`, t.ID, id, t.OwnerID, m)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k service.TeamKey
			if err := rows.Scan(&k.ID, &k.MemberID, &k.UserID, &k.Name, &k.Key, &k.Status, &k.Usage, &k.CreatedAt, &k.Email, &k.LastUsedAt); err != nil {
				return err
			}
			out = append(out, k)
		}
		return rows.Err()
	})
	return out, err
}
func (r *teamRepository) CreateKey(ctx context.Context, id int64, name, key string) (*service.TeamKey, error) {
	var out service.TeamKey
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		t, m, err := teamActor(ctx, tx, id, false)
		if err != nil {
			return err
		}
		out.MemberID = m
		out.UserID = id
		out.Name = name
		out.Key = key
		out.Status = "active"
		if err = tx.QueryRowContext(ctx, `SELECT email FROM users WHERE id=$1`, id).Scan(&out.Email); err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,name,key,status,routing_mode,created_at,updated_at) VALUES($1,$2,$3,'active','team',NOW(),NOW()) RETURNING id,created_at`, t.OwnerID, name, key).Scan(&out.ID, &out.CreatedAt)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO team_api_keys(api_key_id,member_id) VALUES($1,$2)`, out.ID, m)
		return err
	})
	return &out, err
}
func (r *teamRepository) UpdateKey(ctx context.Context, id, key int64, name, status string, remove bool) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		t, m, err := teamActor(ctx, tx, id, false)
		if err != nil {
			return err
		}
		var found int64
		err = tx.QueryRowContext(ctx, `SELECT tk.api_key_id FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id WHERE tk.api_key_id=$1 AND m.team_id=$2 AND ($3=$4 OR m.id=$5) AND NOT tk.revoked`, key, t.ID, id, t.OwnerID, m).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrTeamForbidden
		}
		if err != nil {
			return err
		}
		if remove {
			_, err = tx.ExecContext(ctx, `UPDATE team_api_keys SET revoked=TRUE WHERE api_key_id=$1`, key)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE api_keys SET name=$2,status=$3,updated_at=NOW() WHERE id=$1`, key, name, status)
		}
		return err
	})
}
func (r *teamRepository) KeyAttribution(ctx context.Context, key int64) (*service.TeamAttribution, error) {
	var a service.TeamAttribution
	var groups []byte
	var active, revoked bool
	var status, keyStatus string
	err := r.db.QueryRowContext(ctx, `SELECT t.id,m.id,m.user_id,t.owner_id,t.group_ids,m.active,tk.revoked,t.status,k.status FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id JOIN teams t ON t.id=m.team_id JOIN api_keys k ON k.id=tk.api_key_id WHERE tk.api_key_id=$1`, key).Scan(&a.TeamID, &a.MemberID, &a.MemberUserID, &a.OwnerID, &groups, &active, &revoked, &status, &keyStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !active || revoked || status != "active" || keyStatus != "active" {
		return nil, service.ErrTeamUnavailable
	}
	if err = json.Unmarshal(groups, &a.GroupIDs); err != nil {
		return nil, err
	}
	return &a, nil
}
func (r *teamRepository) Admit(ctx context.Context, a *service.TeamAttribution, key int64) error {
	if a == nil || a.RequestID == "" || len(a.RequestID) > 64 {
		return service.ErrTeamForbidden
	}
	return r.transaction(ctx, func(tx *sql.Tx) error {
		var active bool
		var status string
		var revoked bool
		var u service.TeamUsage
		var l service.TeamLimits
		var day, week, month *time.Time
		err := tx.QueryRowContext(ctx, `SELECT m.active,t.status,(tk.revoked OR k.status!='active' OR k.deleted_at IS NOT NULL),m.daily_limit,m.weekly_limit,m.monthly_limit,m.daily_used,m.weekly_used,m.monthly_used,m.daily_start,m.weekly_start,m.monthly_start FROM team_members m JOIN teams t ON t.id=m.team_id JOIN team_api_keys tk ON tk.member_id=m.id JOIN api_keys k ON k.id=tk.api_key_id WHERE m.id=$1 AND t.id=$2 AND tk.api_key_id=$3 FOR UPDATE OF t,m`, a.MemberID, a.TeamID, key).Scan(&active, &status, &revoked, &l.Daily, &l.Weekly, &l.Monthly, &u.Daily, &u.Weekly, &u.Monthly, &day, &week, &month)
		if err != nil {
			return err
		}
		if !active || revoked || status != "active" {
			return service.ErrTeamUnavailable
		}
		var pendingBill bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM team_requests r WHERE r.member_id=$1 AND (r.unresolved OR EXISTS(SELECT 1 FROM team_billing_pending p WHERE p.admission_id=r.id)))`, a.MemberID).Scan(&pendingBill); err != nil {
			return err
		}
		if pendingBill {
			return service.ErrTeamRequestsPending
		}
		normalizeTeamUsage(&u, day, week, month)
		if (l.Daily > 0 && u.Daily >= l.Daily) || (l.Weekly > 0 && u.Weekly >= l.Weekly) || (l.Monthly > 0 && u.Monthly >= l.Monthly) {
			return service.ErrTeamLimit
		}
		_, err = tx.ExecContext(ctx, `UPDATE team_members SET daily_used=$2,weekly_used=$3,monthly_used=$4,daily_start=$5,weekly_start=CASE WHEN weekly_start IS NULL OR weekly_start+INTERVAL '7 days'<=NOW() THEN NOW() ELSE weekly_start END,monthly_start=CASE WHEN monthly_start IS NULL OR monthly_start+INTERVAL '30 days'<=NOW() THEN NOW() ELSE monthly_start END WHERE id=$1`, a.MemberID, u.Daily, u.Weekly, u.Monthly, timezone.StartOfDay(time.Now()))
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO team_requests(id,member_id,api_key_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, a.RequestID, a.MemberID, key)
		return err
	})
}
func (r *teamRepository) Dissolve(ctx context.Context, id int64) error {
	return r.transaction(ctx, func(tx *sql.Tx) error {
		t, _, err := teamActor(ctx, tx, id, true)
		if err != nil {
			return err
		}
		var pending bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM team_requests r JOIN team_members m ON m.id=r.member_id WHERE m.team_id=$1)`, t.ID).Scan(&pending); err != nil {
			return err
		}
		if pending {
			return service.ErrTeamRequestsPending
		}
		if r.redis != nil {
			rows, err := tx.QueryContext(ctx, `SELECT tk.api_key_id FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id WHERE m.team_id=$1`, t.ID)
			if err != nil {
				return err
			}
			ids := map[int64]bool{}
			for rows.Next() {
				var id int64
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return err
				}
				ids[id] = true
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			if err = deleteTeamImageTaskCache(ctx, r.redis, ids); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE teams SET status='dissolving' WHERE id=$1`, t.ID); err != nil {
			return err
		}
		return cleanupTeam(ctx, tx, t.ID)
	})
}
func cleanupTeam(ctx context.Context, tx *sql.Tx, team int64) error {
	var ready bool
	if err := tx.QueryRowContext(ctx, `SELECT status='dissolving' AND NOT EXISTS(SELECT 1 FROM team_requests r JOIN team_members m ON m.id=r.member_id WHERE m.team_id=t.id) FROM teams t WHERE id=$1 FOR UPDATE`, team).Scan(&ready); err != nil {
		return err
	}
	if !ready {
		return nil
	}
	for _, table := range []string{"prompt_audit_jobs", "content_moderation_logs", "ops_error_logs", "ops_system_logs", "ops_ingress_reject_aggregates", "deleted_api_key_audits"} {
		// Prompt events cascade with their job. Keep personal-key audit records.
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE api_key_id IN(SELECT tk.api_key_id FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id WHERE m.team_id=$1)`, team); err != nil {
			return err
		}
	}
	for _, q := range []string{`DELETE FROM usage_logs WHERE api_key_id IN(SELECT tk.api_key_id FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id WHERE m.team_id=$1)`, `DELETE FROM usage_billing_dedup WHERE api_key_id IN(SELECT tk.api_key_id FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id WHERE m.team_id=$1)`, `DELETE FROM usage_billing_dedup_archive WHERE api_key_id IN(SELECT tk.api_key_id FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id WHERE m.team_id=$1)`, `DELETE FROM api_keys WHERE id IN(SELECT tk.api_key_id FROM team_api_keys tk JOIN team_members m ON m.id=tk.member_id WHERE m.team_id=$1)`, `DELETE FROM teams WHERE id=$1`} {
		if _, err := tx.ExecContext(ctx, q, team); err != nil {
			return err
		}
	}
	return nil
}
func (r *teamRepository) Release(ctx context.Context, id string) error {
	return r.CloseRequest(ctx, id, false)
}
