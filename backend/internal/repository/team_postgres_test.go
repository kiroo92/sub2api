package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestTeamPostgresMembershipBillingAndCleanup(t *testing.T) {
	dsn := os.Getenv("TEAM_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEAM_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("team_test_%d", time.Now().UnixNano())
	_, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		_, err := base.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
		require.NoError(t, base.Close())
	})
	require.NoError(t, client.Schema.Create(ctx))
	// The shared usage writer also consumes SQL-only fields not modeled by Ent.
	for _, file := range []string{"046_add_usage_log_reasoning_effort.sql", "060_add_usage_log_openai_ws_mode.sql", "061_add_usage_log_request_type.sql", "070_add_usage_log_service_tier.sql", "074_add_usage_log_endpoints.sql", "089_usage_log_image_output_tokens.sql", "179_usage_log_image_input_tokens.sql", "194_add_usage_log_upstream_response_model.sql", "231_add_usage_log_native_compaction_v2.sql", "231_add_usage_log_requested_reasoning_effort.sql", "232_add_usage_log_upstream_request_id.sql"} {
		b, e := migrations.FS.ReadFile(file)
		require.NoError(t, e)
		_, e = db.ExecContext(ctx, string(b))
		require.NoError(t, e)
	}
	_, err = db.ExecContext(ctx, `ALTER TABLE usage_logs ADD COLUMN account_stats_cost NUMERIC(20,10), ADD COLUMN session_id VARCHAR(255); CREATE UNIQUE INDEX team_test_usage_dedup ON usage_logs(request_id,api_key_id)`)
	require.NoError(t, err)
	for _, file := range []string{"033_ops_monitoring_vnext.sql", "054_ops_system_logs.sql", "071_add_usage_billing_dedup.sql", "073_add_usage_billing_dedup_archive.sql", "135_content_moderation.sql", "145_deleted_api_key_audit.sql", "154_add_ops_system_logs_api_key_id.sql", "181_prompt_audit.sql", "183_ops_ingress_reject_aggregates.sql", "241_team_billing.sql", "242_team_billing_recovery.sql"} {
		b, err := migrations.FS.ReadFile(file)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, string(b))
		require.NoError(t, err)
	}
	owner := mustCreateAPIKeyRepoUser(t, ctx, client, "team-owner@example.com")
	member := mustCreateAPIKeyRepoUser(t, ctx, client, "team-member@example.com")
	outsider := mustCreateAPIKeyRepoUser(t, ctx, client, "team-outsider@example.com")
	_, err = client.User.UpdateOneID(owner.ID).SetBalance(100).Save(ctx)
	require.NoError(t, err)
	repo := NewTeamRepository(db, nil)
	require.NoError(t, repo.Create(ctx, owner.ID, "Shared"))
	adminItems, adminTotal, err := repo.AdminList(ctx, "team-owner@example.com", "active", 1, 20)
	require.NoError(t, err)
	require.Equal(t, 1, adminTotal)
	require.Len(t, adminItems, 1)
	adminTeamID := adminItems[0].ID
	resolvedOwner, err := repo.AdminOwner(ctx, adminTeamID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, resolvedOwner)
	adminCtx := service.WithTeamAdminScope(ctx, adminTeamID)
	wrongAdminCtx := service.WithTeamAdminScope(ctx, adminTeamID+999)
	_, err = repo.Snapshot(wrongAdminCtx, owner.ID)
	require.ErrorIs(t, err, service.ErrTeamForbidden)
	renamed := "Shared by admin"
	require.ErrorIs(t, repo.Update(wrongAdminCtx, owner.ID, service.TeamSettings{Name: &renamed}), service.ErrTeamForbidden)
	require.NoError(t, repo.Update(adminCtx, owner.ID, service.TeamSettings{Name: &renamed}))
	require.ErrorIs(t, repo.Create(ctx, owner.ID, "Again"), service.ErrTeamConflict)
	require.ErrorIs(t, repo.SetLimits(ctx, member.ID, owner.ID, service.TeamLimits{Daily: 1}), service.ErrTeamForbidden)
	invite, err := repo.Invite(ctx, owner.ID, member.Email, "invite-hash", 0)
	require.NoError(t, err)
	snapshot, err := repo.Snapshot(ctx, member.ID)
	require.NoError(t, err)
	require.Len(t, snapshot.PendingInvitations, 1)
	require.Empty(t, snapshot.Invitations)
	require.ErrorIs(t, repo.Accept(ctx, outsider.ID, "invite-hash", invite.ID), service.ErrTeamConflict)
	_, err = repo.Invite(ctx, member.ID, "", "forged-resend", invite.ID)
	require.ErrorIs(t, err, service.ErrTeamForbidden)
	require.ErrorIs(t, repo.RevokeInvite(ctx, member.ID, invite.ID), service.ErrTeamForbidden)
	_, err = db.ExecContext(ctx, `UPDATE team_invitations SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, invite.ID)
	require.NoError(t, err)
	require.ErrorIs(t, repo.Accept(ctx, member.ID, "", invite.ID), service.ErrTeamConflict)
	_, err = repo.Invite(ctx, owner.ID, "", "renewed-hash", invite.ID)
	require.NoError(t, err)
	require.ErrorIs(t, repo.Accept(ctx, member.ID, "invite-hash", 0), service.ErrTeamConflict)
	require.NoError(t, repo.Accept(ctx, member.ID, "renewed-hash", 0))
	require.ErrorIs(t, repo.Create(ctx, member.ID, "Another"), service.ErrTeamConflict)
	require.NoError(t, repo.SetLimits(ctx, owner.ID, member.ID, service.TeamLimits{Daily: 1, Weekly: 2, Monthly: 3}))
	key, err := repo.CreateKey(ctx, member.ID, "Member key", "sk-team-member-test")
	require.NoError(t, err)
	ownerKey, err := repo.CreateKey(ctx, owner.ID, "Owner key", "sk-team-owner-test")
	require.NoError(t, err)
	ownerKeys, err := repo.Keys(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, ownerKeys, 2)
	adminKeys, err := repo.Keys(adminCtx, owner.ID)
	require.NoError(t, err)
	require.Len(t, adminKeys, 2)
	_, err = repo.Keys(wrongAdminCtx, owner.ID)
	require.ErrorIs(t, err, service.ErrTeamForbidden)
	memberKeys, err := repo.Keys(ctx, member.ID)
	require.NoError(t, err)
	require.Len(t, memberKeys, 1)
	require.Equal(t, key.Key, memberKeys[0].Key)
	require.ErrorIs(t, repo.UpdateKey(ctx, member.ID, ownerKey.ID, "hack", "active", false), service.ErrTeamForbidden)
	a, err := repo.KeyAttribution(ctx, key.ID)
	require.NoError(t, err)
	a.RequestID = "request-1"
	require.NoError(t, repo.Admit(ctx, a, key.ID))
	require.ErrorIs(t, repo.Dissolve(ctx, owner.ID), service.ErrTeamRequestsPending)
	require.ErrorIs(t, repo.Dissolve(adminCtx, owner.ID), service.ErrTeamRequestsPending, "platform admin cannot bypass settlement")
	snapshot, err = repo.Snapshot(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "active", snapshot.Team.Status)
	billing := NewUsageBillingRepository(client, db)
	command := &service.UsageBillingCommand{RequestID: "bill-1", APIKeyID: key.ID, UserID: owner.ID, TeamMemberID: a.MemberID, TeamRequestID: a.RequestID, BalanceCost: 0.123456785}
	result, err := billing.Apply(ctx, command)
	require.NoError(t, err)
	require.True(t, result.Applied)
	result, err = billing.Apply(ctx, command)
	require.NoError(t, err)
	require.False(t, result.Applied)
	var balance, used, keyUsed float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, owner.ID).Scan(&balance))
	require.Equal(t, 99.87654321, balance)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT daily_used FROM team_members WHERE id=$1`, a.MemberID).Scan(&used))
	require.Equal(t, 0.12345679, used)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT quota_used FROM api_keys WHERE id=$1`, key.ID).Scan(&keyUsed))
	require.Equal(t, used, keyUsed)
	// A billing error must roll back both the member meter and debit.
	bad := &service.UsageBillingCommand{RequestID: "bad", APIKeyID: key.ID, UserID: outsider.ID, TeamMemberID: a.MemberID, TeamRequestID: a.RequestID, BalanceCost: 1}
	_, err = billing.Apply(ctx, bad)
	require.Error(t, err)
	require.NoError(t, repo.Release(ctx, a.RequestID))
	// Subscription-funded team requests charge the exact owner subscription,
	// not the member's wallet, with the same member money meter.
	group, err := client.Group.Create().SetName("Team subscription").SetSubscriptionType(service.SubscriptionTypeSubscription).Save(ctx)
	require.NoError(t, err)
	sub, err := client.UserSubscription.Create().SetUserID(owner.ID).SetGroupID(group.ID).SetStartsAt(time.Now()).SetExpiresAt(time.Now().Add(time.Hour)).Save(ctx)
	require.NoError(t, err)
	a.RequestID = "sub-request"
	require.NoError(t, repo.Admit(ctx, a, key.ID))
	subCommand := &service.UsageBillingCommand{RequestID: "sub-bill", APIKeyID: key.ID, UserID: owner.ID, TeamMemberID: a.MemberID, TeamRequestID: a.RequestID, SubscriptionID: &sub.ID, AdmittedSubscription: true, SubscriptionCost: 0.25}
	_, err = billing.Apply(ctx, subCommand)
	require.NoError(t, err)
	charged, err := client.UserSubscription.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, 0.25, charged.DailyUsageUsd)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, owner.ID).Scan(&balance))
	require.Equal(t, 99.87654321, balance)
	require.NoError(t, repo.Release(ctx, a.RequestID))
	a.RequestID = "request-2"
	// A failed debit leaves an exact, credential-free command outside the failed transaction.
	ownerAdmission, err := repo.KeyAttribution(ctx, ownerKey.ID)
	require.NoError(t, err)
	ownerAdmission.RequestID = "recoverable-request"
	require.NoError(t, repo.Admit(ctx, ownerAdmission, ownerKey.ID))
	_, err = db.ExecContext(ctx, `CREATE FUNCTION reject_team_test_debit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected debit failure'; END $$;
CREATE TRIGGER reject_team_test_debit BEFORE UPDATE OF balance ON users FOR EACH ROW EXECUTE FUNCTION reject_team_test_debit()`)
	require.NoError(t, err)
	retryCommand := &service.UsageBillingCommand{RequestID: "recoverable-bill", APIKeyID: ownerKey.ID, UserID: owner.ID, TeamMemberID: ownerAdmission.MemberID, TeamRequestID: ownerAdmission.RequestID, BalanceCost: 0.5}
	accountID := mustCreateAPIKeyRepoAccount(t, ctx, client, "Recovery account")
	retryCommand.AccountID = accountID
	retryCommand.TeamUsageLog = &service.UsageLog{UserID: owner.ID, APIKeyID: ownerKey.ID, AccountID: accountID, RequestID: "recoverable-bill", Model: "gpt-5", ActualCost: 0.5, TotalCost: 0.5, RateMultiplier: 1, CreatedAt: time.Now()}
	failedLog := *retryCommand.TeamUsageLog
	failedLog.ActualCost = 0
	_, err = (&usageLogRepository{sql: db}).createSingle(ctx, db, &failedLog)
	require.NoError(t, err)
	_, err = billing.Apply(ctx, retryCommand)
	require.Error(t, err)
	blockedAdmission := *ownerAdmission
	blockedAdmission.RequestID = "must-not-bypass-unpaid-bill"
	require.ErrorIs(t, repo.Admit(ctx, &blockedAdmission, ownerKey.ID), service.ErrTeamRequestsPending)
	snapshot, err = repo.Snapshot(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, snapshot.PendingBilling, "active execution must not be retried")
	n, err := repo.RecoverBilling(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, n)
	require.NoError(t, repo.Release(ctx, ownerAdmission.RequestID))
	snapshot, err = repo.Snapshot(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, 1, snapshot.PendingBilling)
	_, err = repo.RecoverBilling(ctx, member.ID)
	require.ErrorIs(t, err, service.ErrTeamForbidden)
	_, err = repo.RecoverBilling(ctx, outsider.ID)
	require.ErrorIs(t, err, service.ErrTeamForbidden)
	_, err = repo.RecoverBilling(ctx, owner.ID)
	require.Error(t, err, "retry failure retains pending debit")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, owner.ID).Scan(&balance))
	require.Equal(t, 99.87654321, balance)
	_, err = db.ExecContext(ctx, `DROP TRIGGER reject_team_test_debit ON users; DROP FUNCTION reject_team_test_debit()`)
	require.NoError(t, err)
	// Concurrent recovery calls serialize at the team and do not double debit.
	var recoveryWG sync.WaitGroup
	recovered := make(chan int, 2)
	recoveryErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		recoveryWG.Add(1)
		go func() {
			defer recoveryWG.Done()
			n, e := repo.RecoverBilling(ctx, owner.ID)
			recovered <- n
			recoveryErrors <- e
		}()
	}
	recoveryWG.Wait()
	close(recovered)
	close(recoveryErrors)
	for e := range recoveryErrors {
		require.NoError(t, e)
	}
	totalRecovered := 0
	for n := range recovered {
		totalRecovered += n
	}
	require.Equal(t, 1, totalRecovered)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, owner.ID).Scan(&balance))
	require.Equal(t, 99.37654321, balance)
	var recoveredCost float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT actual_cost FROM usage_logs WHERE request_id='recoverable-bill' AND api_key_id=$1`, ownerKey.ID).Scan(&recoveredCost))
	require.Equal(t, 0.5, recoveredCost)
	result, err = billing.Apply(ctx, retryCommand)
	require.NoError(t, err)
	require.False(t, result.Applied, "late retry uses original dedup after recovery released the lease")
	snapshot, err = repo.Snapshot(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, snapshot.PendingBilling)
	require.NoError(t, repo.Admit(ctx, a, key.ID))
	// Late settlement retains the original member after departure.
	require.NoError(t, repo.Leave(ctx, member.ID, 0))
	_, err = repo.KeyAttribution(ctx, key.ID)
	require.ErrorIs(t, err, service.ErrTeamUnavailable)
	command = &service.UsageBillingCommand{RequestID: "bill-2", APIKeyID: key.ID, UserID: owner.ID, TeamMemberID: a.MemberID, TeamRequestID: a.RequestID, BalanceCost: 1}
	_, err = billing.Apply(ctx, command)
	require.NoError(t, err)
	require.NoError(t, repo.Release(ctx, a.RequestID))
	invite, err = repo.Invite(ctx, owner.ID, member.Email, "invite-again", 0)
	require.NoError(t, err)
	require.NoError(t, repo.Accept(ctx, member.ID, "", invite.ID))
	newKey, err := repo.CreateKey(ctx, member.ID, "Rejoined", "sk-team-rejoin-test")
	require.NoError(t, err)
	next, err := repo.KeyAttribution(ctx, newKey.ID)
	require.NoError(t, err)
	next.RequestID = "blocked"
	require.Equal(t, a.MemberID, next.MemberID)
	require.ErrorIs(t, repo.Admit(ctx, next, newKey.ID), service.ErrTeamLimit)
	// The meter is shared by keys and resets only when its actual window elapses.
	_, err = db.ExecContext(ctx, `UPDATE team_members SET daily_start=NOW()-INTERVAL '2 days' WHERE id=$1`, a.MemberID)
	require.NoError(t, err)
	next.RequestID = "after-reset"
	require.NoError(t, repo.Admit(ctx, next, newKey.ID))
	require.NoError(t, repo.Release(ctx, next.RequestID))
	snapshot, err = repo.Snapshot(ctx, member.ID)
	require.NoError(t, err)
	require.Len(t, snapshot.Members, 1)
	require.Zero(t, snapshot.Members[0].Usage.Daily)
	require.Equal(t, 1.37345679, snapshot.Members[0].Usage.Weekly)
	// A management request waiting behind removal must recheck membership,
	// rather than create a key that could become valid on a later rejoin.
	removal, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = removal.Rollback() }()
	_, err = removal.ExecContext(ctx, `SELECT id FROM teams WHERE id=$1 FOR UPDATE`, a.TeamID)
	require.NoError(t, err)
	raceCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	keyResult := make(chan error, 1)
	go func() {
		_, err := repo.CreateKey(raceCtx, member.ID, "Removed race", "sk-team-removed-race")
		keyResult <- err
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT t.id,t.owner_id,t.name,%')`).Scan(&blocked)
		return err == nil && blocked
	}, 5*time.Second, 10*time.Millisecond)
	_, err = removal.ExecContext(ctx, `UPDATE team_members SET active=FALSE WHERE id=$1`, a.MemberID)
	require.NoError(t, err)
	_, err = removal.ExecContext(ctx, `UPDATE team_api_keys SET revoked=TRUE WHERE member_id=$1`, a.MemberID)
	require.NoError(t, err)
	require.NoError(t, removal.Commit())
	require.ErrorIs(t, <-keyResult, service.ErrTeamForbidden)
	paused := "paused"
	require.NoError(t, repo.Update(ctx, owner.ID, service.TeamSettings{Status: &paused}))
	_, err = repo.KeyAttribution(ctx, ownerKey.ID)
	require.ErrorIs(t, err, service.ErrTeamUnavailable)
	active := "active"
	require.NoError(t, repo.Update(ctx, owner.ID, service.TeamSettings{Status: &active}))
	_, err = repo.KeyAttribution(ctx, ownerKey.ID)
	require.NoError(t, err)
	personal, err := client.APIKey.Create().SetUserID(owner.ID).SetName("Personal").SetKey("sk-personal-preserved").Save(ctx)
	require.NoError(t, err)
	for _, keyID := range []int64{key.ID, personal.ID} {
		var job int64
		require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO prompt_audit_jobs(api_key_id,user_id) VALUES($1,$2) RETURNING id`, keyID, owner.ID).Scan(&job))
		_, err = db.ExecContext(ctx, `INSERT INTO prompt_audit_events(job_id,api_key_id,user_id) VALUES($1,$2,$3)`, job, keyID, owner.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO content_moderation_logs(api_key_id,user_id) VALUES($1,$2)`, keyID, owner.ID)
		require.NoError(t, err)
	}
	require.NoError(t, repo.Dissolve(ctx, owner.ID))
	for _, table := range []string{"prompt_audit_jobs", "prompt_audit_events", "content_moderation_logs"} {
		var count int
		var remainingKey int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*),MIN(api_key_id) FROM `+table).Scan(&count, &remainingKey))
		require.Equal(t, 1, count, table)
		require.Equal(t, personal.ID, remainingKey, table)
	}
	for _, table := range []string{"teams", "team_members", "team_api_keys", "team_requests", "team_invitations", "usage_billing_dedup"} {
		var n int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n))
		require.Zero(t, n, table)
	}
	_, err = client.APIKey.Get(ctx, personal.ID)
	require.NoError(t, err)
	// Create/accept races have exactly one winner, across processes via the user lock.
	require.NoError(t, repo.Create(ctx, owner.ID, "Race"))
	invite, err = repo.Invite(ctx, owner.ID, outsider.Email, "race", 0)
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); results <- repo.Create(ctx, outsider.ID, "Competing") }()
	go func() { defer wg.Done(); results <- repo.Accept(ctx, outsider.ID, "", invite.ID) }()
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	require.Equal(t, 1, wins)
	unknownKey, err := repo.CreateKey(ctx, owner.ID, "Unknown usage", "sk-unknown-usage-test")
	require.NoError(t, err)
	unknown, err := repo.KeyAttribution(ctx, unknownKey.ID)
	require.NoError(t, err)
	unknown.RequestID = "unknown-request"
	require.NoError(t, repo.Admit(ctx, unknown, unknownKey.ID))
	require.NoError(t, repo.CloseRequest(ctx, unknown.RequestID, true))
	_, err = repo.RecoverBilling(ctx, owner.ID)
	require.NoError(t, err)
	require.ErrorIs(t, repo.Dissolve(ctx, owner.ID), service.ErrTeamRequestsPending, "unknown usage must never be forgiven by retry")
}
