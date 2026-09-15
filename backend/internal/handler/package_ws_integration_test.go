package handler

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type packageWSAccounts struct {
	*openAIWSFailoverHandlerAccountRepoStub
	composite bool
}

func (r *packageWSAccounts) ListSchedulableByGroupID(_ context.Context, id int64) ([]service.Account, error) {
	if r.composite {
		return r.accounts, nil
	}
	return []service.Account{r.accounts[id-1]}, nil
}
func (r *packageWSAccounts) ListSchedulableByGroupIDAndPlatform(ctx context.Context, id int64, platform string) ([]service.Account, error) {
	accounts, err := r.ListSchedulableByGroupID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Platform == platform {
			result = append(result, account)
		}
	}
	return result, nil
}

type packageWSGroups struct {
	service.GroupRepository
	composite bool
}

func (r packageWSGroups) GetByID(_ context.Context, id int64) (*service.Group, error) {
	platform := service.PlatformOpenAI
	if r.composite {
		platform = service.PlatformComposite
	}
	return &service.Group{ID: id, Name: fmt.Sprint(id), Platform: platform, Status: service.StatusActive, RateMultiplier: float64(id)}, nil
}

type packageWSRoutes struct {
	service.CompositeModelRouteRepository
}

func (packageWSRoutes) ListByGroup(context.Context, int64, bool) ([]service.CompositeModelRoute, error) {
	return []service.CompositeModelRoute{
		{ID: 1, GroupID: 1, PublicModel: "first-alias", MatchType: service.CompositeRouteMatchExact, TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-5.1", Endpoint: service.CompositeRouteEndpointResponses, Enabled: true},
		{ID: 2, GroupID: 1, PublicModel: "second-alias", MatchType: service.CompositeRouteMatchExact, TargetPlatform: service.PlatformGrok, UpstreamModel: "grok-4.6", Endpoint: service.CompositeRouteEndpointResponses, Enabled: true},
	}, nil
}

type packageWSHTTP struct{ service.HTTPUpstream }

func (packageWSHTTP) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

type packageWSKeys struct {
	service.APIKeyRepository
	db  *sql.DB
	key *service.APIKey
}

func (r packageWSKeys) GetByID(ctx context.Context, id int64) (*service.APIKey, error) {
	key := *r.key
	err := r.db.QueryRowContext(ctx, `SELECT quota,quota_used,status FROM api_keys WHERE id=$1`, id).Scan(&key.Quota, &key.QuotaUsed, &key.Status)
	return &key, err
}

func packageWSDatabase(t *testing.T) (*sql.DB, *dbent.Client) {
	t.Helper()
	dsn := os.Getenv("PACKAGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PACKAGE_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("package_ws_test_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		_, err := base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, err)
		require.NoError(t, base.Close())
	})
	_, err = db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,8) DEFAULT 100);
CREATE TABLE api_keys(id BIGINT PRIMARY KEY,user_id BIGINT,quota NUMERIC(20,8) DEFAULT 0,quota_used NUMERIC(20,8) DEFAULT 0,status TEXT DEFAULT 'active',deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
CREATE TABLE user_packages(id BIGINT PRIMARY KEY,user_id BIGINT,group_id BIGINT,order_id BIGINT,plan_snapshot JSONB,group_buy_id BIGINT,status TEXT,starts_at TIMESTAMPTZ,expires_at TIMESTAMPTZ,sort_order INT);
CREATE TABLE package_periods(id BIGINT PRIMARY KEY,package_id BIGINT,period_index INT,starts_at TIMESTAMPTZ,ends_at TIMESTAMPTZ,quota_usd NUMERIC(20,8),used_usd NUMERIC(20,8) DEFAULT 0);
INSERT INTO users VALUES(1,100);INSERT INTO api_keys(id,user_id) VALUES(1,1);
INSERT INTO user_packages SELECT i,1,i,i,'{"name":"test"}',NULL,'active',NOW()-INTERVAL '1 hour',NOW()+INTERVAL '1 day',i FROM generate_series(1,2)i;
INSERT INTO package_periods SELECT i,i,0,NOW()-INTERVAL '1 hour',NOW()+INTERVAL '1 day',0.00000001,0 FROM generate_series(1,2)i;`)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile("241_package_usage_billing.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	return db, dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
}

func TestPackageWSHandoffAcrossTransports(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModeHTTPBridge, service.OpenAIWSIngressModePassthrough} {
		for _, scenario := range []string{"normal", "missing_history", "same_group", "key_quota", "composite_alias", "first_missing_history"} {
			t.Run(mode+"_"+scenario, func(t *testing.T) { runPackageWSHandoff(t, mode, scenario) })
		}
	}
}

func runPackageWSHandoff(t *testing.T, mode, scenario string) {
	t.Helper()
	missingHistory, keyQuota, composite := scenario == "missing_history", scenario == "key_quota", scenario == "composite_alias"
	sameGroup := scenario == "same_group" || composite
	gin.SetMode(gin.TestMode)
	db, client := packageWSDatabase(t)
	secondGroup, rate := int64(2), 2.0
	if sameGroup {
		_, err := db.Exec(`UPDATE user_packages SET group_id=1 WHERE id=2`)
		require.NoError(t, err)
		secondGroup, rate = 1, 1
	}
	type upstreamCall struct {
		group int
		body  []byte
	}
	requests := make(chan upstreamCall, 8)
	var mu sync.Mutex
	turn := 0
	upstreams := make([]*httptest.Server, 0, 2)
	for group := 1; group <= 2; group++ {
		group := group
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respond := func(payload []byte) string {
				requests <- upstreamCall{group, payload}
				mu.Lock()
				turn++
				n := turn
				mu.Unlock()
				return fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_package_%d","model":%q,"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer-%d"}]},{"type":"function_call","call_id":"call_%d","name":"read","arguments":"{}"}],"usage":{"input_tokens":2,"output_tokens":1}}}`, n, gjson.GetBytes(payload, "model").String(), n, n)
			}
			if mode == service.OpenAIWSIngressModeHTTPBridge || (composite && group == 2) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "event: response.completed\ndata: %s\n\n", respond(body))
				return
			}
			conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
			if err != nil {
				return
			}
			defer func() { _ = conn.CloseNow() }()
			for {
				_, body, err := conn.Read(r.Context())
				if err != nil {
					return
				}
				if err = conn.Write(r.Context(), coderws.MessageText, []byte(respond(body))); err != nil {
					return
				}
			}
		}))
		upstreams = append(upstreams, upstream)
		t.Cleanup(upstream.Close)
	}
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	accounts := &packageWSAccounts{openAIWSFailoverHandlerAccountRepoStub: &openAIWSFailoverHandlerAccountRepoStub{}, composite: composite}
	for i, up := range upstreams {
		platform := service.PlatformOpenAI
		if composite && i == 1 {
			platform = service.PlatformGrok
		}
		accounts.accounts = append(accounts.accounts, service.Account{ID: int64(i + 1), Platform: platform, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test", "base_url": up.URL}, Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true, "openai_apikey_responses_websockets_v2_mode": mode}})
	}
	billing := repository.NewUsageBillingRepository(client, db)
	key := &service.APIKey{ID: 1, UserID: 1, RoutingMode: service.APIKeyRoutingAllPackages, Status: service.StatusActive, User: &service.User{ID: 1, Status: service.StatusActive}}
	if keyQuota {
		key.Quota = 0.00000001
		_, err := db.Exec(`UPDATE api_keys SET quota=$1 WHERE id=1`, key.Quota)
		require.NoError(t, err)
	}
	groups := packageWSGroups{composite: composite}
	keys := service.NewAPIKeyService(packageWSKeys{db: db, key: key}, nil, groups, nil, nil, nil, cfg)
	keys.SetPackageService(service.NewPackageService(client, groups))
	keys.SetPackageBillingRepository(billing)
	keys.SetPackageRoutingRepositories(accounts, service.NewCompositeRouteResolver(packageWSRoutes{}))
	usage := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
	cache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(cache.Stop)
	gw := service.NewOpenAIGatewayService(accounts, usage, billing, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, cache, packageWSHTTP{}, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	concurrency := &concurrencyCacheMock{acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil }, acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil }}
	h := &OpenAIGatewayHandler{gatewayService: gw, billingCacheService: cache, apiKeyService: keys, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(concurrency), SSEPingFormatNone, time.Second), cfg: cfg, maxAccountSwitches: 2}
	// An occupied worker proves package charges don't enter the normal async
	// queue: the next frame is sent immediately after the visible terminal.
	pool := newUsageRecordTestPool(t)
	busy, release := make(chan struct{}), make(chan struct{})
	pool.Submit(func(context.Context) { close(busy); <-release })
	<-busy
	t.Cleanup(func() { close(release) })
	h.usageRecordWorkerPool = pool
	done := make(chan struct{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), key)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})
		c.Request = c.Request.WithContext(service.WithPackageRequest(c.Request.Context()))
		c.Next()
	})
	router.GET("/v1/responses", func(c *gin.Context) { defer close(done); h.ResponsesWebSocket(c) })
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	write := func(body string) { require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(body))) }
	read := func() []byte {
		for {
			_, body, err := conn.Read(ctx)
			require.NoError(t, err)
			if gjson.GetBytes(body, "type").String() == "response.completed" {
				return body
			}
		}
	}
	bill := func() *service.UsageLog {
		select {
		case log := <-usage.created:
			return log
		case <-ctx.Done():
			t.Fatal("billing did not complete")
			return nil
		}
	}
	model, nextModel := "gpt-5.1", ""
	if composite {
		model, nextModel = "first-alias", `"model":"second-alias",`
	}
	if scenario == "first_missing_history" {
		write(`{"type":"response.create","model":"gpt-5.1","previous_response_id":"resp_unknown","input":[{"type":"function_call_output","call_id":"missing","output":"done"}]}`)
		_, _, err := conn.Read(ctx)
		var closeErr coderws.CloseError
		require.ErrorAs(t, err, &closeErr)
		require.Contains(t, closeErr.Reason, "package_context_required")
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("gateway did not close after rejecting missing first-frame history")
		}
		require.Empty(t, requests, "unknown tool history must fail before sending a generation")
		require.Empty(t, usage.created)
		return
	}
	write(fmt.Sprintf(`{"type":"response.create","model":%q,"input":[{"role":"user","content":"hello"}]}`, model))
	read()
	previous := "resp_package_1"
	if missingHistory {
		previous = "resp_unknown"
	}
	write(fmt.Sprintf(`{"type":"response.create",%s"previous_response_id":%q,"input":[{"type":"function_call_output","call_id":"call_1","output":"done"}]}`, nextModel, previous))
	first := bill()
	require.Greater(t, first.ActualCost, 0.0)
	require.Equal(t, int64(1), *first.GroupID)
	if missingHistory || keyQuota {
		_, _, err = conn.Read(ctx)
		require.Error(t, err)
		var closeErr coderws.CloseError
		require.ErrorAs(t, err, &closeErr)
		if keyQuota {
			require.Contains(t, closeErr.Reason, service.ErrAPIKeyQuotaExhausted.Error())
		} else {
			require.Contains(t, closeErr.Reason, "package_context_required")
		}
	} else {
		read()
		second := bill()
		require.Equal(t, secondGroup, *second.GroupID)
		if composite {
			require.Greater(t, second.ActualCost, 0.0)
		} else {
			require.InDelta(t, first.ActualCost*rate, second.ActualCost, 1e-8)
		}
		_, err = db.Exec(`UPDATE package_periods SET used_usd=0 WHERE id=1`)
		require.NoError(t, err)
		write(fmt.Sprintf(`{"type":"response.create","model":%q,"previous_response_id":"resp_package_2","input":[{"role":"user","content":"again"}]}`, model))
		read()
		third := bill()
		require.Equal(t, int64(1), *third.GroupID)
		require.NotEqual(t, first.RequestID, third.RequestID)
		require.Nil(t, third.SubscriptionID)
		_ = conn.Close(coderws.StatusNormalClosure, "done")
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("gateway did not release downstream connection")
	}
	count := 1
	if !missingHistory && !keyQuota {
		count = 3
	}
	require.Len(t, requests, count)
	for i := 0; i < count; i++ {
		request := <-requests
		if composite {
			want := "gpt-5.1"
			if i == 1 {
				want = "grok-4.6"
			}
			require.Equal(t, want, gjson.GetBytes(request.body, "model").String(), "the configured alias must be mapped for this turn")
		}
		if i == 1 {
			if composite {
				require.Equal(t, 2, request.group, "same-group platform switch must select a new account")
			} else {
				require.Equal(t, int(secondGroup), request.group)
			}
			if !sameGroup || composite {
				require.False(t, gjson.GetBytes(request.body, "previous_response_id").Exists())
				require.Contains(t, string(request.body), "answer-1")
			}
			require.Contains(t, string(request.body), "call_1")
		}
		if i == 2 {
			require.Equal(t, 1, request.group)
			if !sameGroup || composite {
				require.Contains(t, string(request.body), "answer-1")
				require.Contains(t, string(request.body), "answer-2")
			}
		}
	}
	var bills int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM package_usage_billing WHERE applied`).Scan(&bills))
	require.Equal(t, count, bills)
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 100.0, balance)
	require.Nil(t, key.GroupID)
	require.Nil(t, key.PackageSelection)
}
