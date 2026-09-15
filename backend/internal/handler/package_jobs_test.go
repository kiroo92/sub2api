package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPackageAsyncImagePersistsBeforeGeneration(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	db, client := packageWSDatabase(t)
	raw, err := migrations.FS.ReadFile("242_package_jobs.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	packages := service.NewPackageService(client, packageWSGroups{})
	keys := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
	keys.SetPackageService(packages)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	h := NewAsyncImageHandler(tasks, &OpenAIGatewayHandler{apiKeyService: keys})
	key := &service.APIKey{ID: 1, UserID: 1, RoutingMode: service.APIKeyRoutingAllPackages, User: &service.User{ID: 1},
		Group:            &service.Group{ID: 1, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		PackageSelection: &service.PackageSelection{UserID: 1, GroupID: 1, PackageID: 1, PeriodID: 1, QuotaUSD: 1}}
	started := make(chan string, 1)
	finish := make(chan struct{})
	finished := make(chan struct{})
	h.execute = func(_ string, c *gin.Context) {
		defer close(finished)
		var resourceID string
		if err := db.QueryRow(`SELECT resource_id FROM package_jobs WHERE kind='image'`).Scan(&resourceID); err != nil {
			started <- ""
			return
		}
		started <- resourceID
		<-finish
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{{"url": "https://example.test/generated.png"}}})
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), key)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)
	submit := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}
	w := submit()
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	var accepted struct {
		ID      string `json:"id"`
		PollURL string `json:"poll_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	select {
	case id := <-started:
		require.Equal(t, accepted.ID, id, "creation attribution must be durable before upstream execution")
	case <-time.After(time.Second):
		t.Fatal("generation did not start")
	}
	_, err = db.Exec(`UPDATE user_packages SET sort_order=-id,expires_at=NOW()-INTERVAL '1 second'; UPDATE package_periods SET used_usd=quota_usd`)
	require.NoError(t, err)
	restored, err := keys.RestorePackageJob(context.Background(), key, "image", accepted.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), restored.PackageSelection.PackageID)
	require.Equal(t, int64(1), restored.PackageSelection.PeriodID)
	close(finish)
	<-finished
	require.Eventually(t, func() bool {
		task, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 1, APIKeyID: 1}, accepted.ID)
		return err == nil && task.Status == service.ImageTaskStatusCompleted
	}, time.Second, 10*time.Millisecond)
	poll := httptest.NewRecorder()
	router.ServeHTTP(poll, httptest.NewRequest(http.MethodGet, accepted.PollURL, nil))
	require.Equal(t, http.StatusOK, poll.Code)
	require.Contains(t, poll.Body.String(), "generated.png")
	_, err = tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 2, APIKeyID: 1}, accepted.ID)
	require.ErrorIs(t, err, service.ErrImageTaskNotFound)

	// A failed durable admission must never launch generation.
	_, err = db.Exec(`DROP TABLE package_jobs`)
	require.NoError(t, err)
	w = submit()
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	select {
	case <-started:
		t.Fatal("generation started without durable attribution")
	default:
	}
}

func TestPackageVideoCompletionBillsCreationPeriodOnce(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	db, client := packageWSDatabase(t)
	raw, err := migrations.FS.ReadFile("242_package_jobs.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	keys := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
	keys.SetPackageService(service.NewPackageService(client, packageWSGroups{}))
	key := &service.APIKey{ID: 1, UserID: 1, RoutingMode: service.APIKeyRoutingAllPackages, User: &service.User{ID: 1},
		Group:            &service.Group{ID: 1, Platform: service.PlatformGrok},
		PackageSelection: &service.PackageSelection{UserID: 1, GroupID: 1, PackageID: 1, PeriodID: 1}}
	job, err := keys.PreparePackageJob(context.Background(), key, "grok_video", 31)
	require.NoError(t, err)
	job.VideoPending = &service.GrokVideoPendingBilling{Model: "grok-imagine-video", VideoDurationSeconds: 6, VideoResolution: "720p"}
	require.NoError(t, keys.CompletePackageJob(context.Background(), key, job, "completed-video"))
	_, err = db.Exec(`UPDATE user_packages SET sort_order=-id,expires_at=NOW()-INTERVAL '1 second';
UPDATE package_periods SET ends_at=NOW()-INTERVAL '1 second' WHERE id=1`)
	require.NoError(t, err)
	billing := repository.NewUsageBillingRepository(client, db)
	for i := 0; i < 2; i++ {
		restored, err := keys.RestorePackageJob(context.Background(), key, "grok_video", "completed-video")
		require.NoError(t, err)
		selection := restored.PackageSelection
		result, err := billing.Apply(context.Background(), &service.UsageBillingCommand{
			RequestID: "video:completed-video", APIKeyID: restored.ID, UserID: restored.UserID,
			PackageID: selection.PackageID, PackagePeriodID: selection.PeriodID, PackageCost: 0.25,
		})
		require.NoError(t, err)
		require.Equal(t, i == 0, result.Applied)
	}
	var first, next, balance float64
	require.NoError(t, db.QueryRow(`SELECT used_usd FROM package_periods WHERE id=1`).Scan(&first))
	require.NoError(t, db.QueryRow(`SELECT used_usd FROM package_periods WHERE id=2`).Scan(&next))
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 0.25, first)
	require.Zero(t, next)
	require.Equal(t, 100.0, balance)
}
