package service

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestParseLastUsedPayloadKeepsLatestTimestamp(t *testing.T) {
	payload := map[string]any{
		"last_used": map[string]any{
			"1":  json.Number("100"),
			"2":  float64(200),
			"x":  "bad",
			"-1": float64(300),
		},
	}

	got := parseLastUsedPayload(payload)
	if len(got) != 2 {
		t.Fatalf("parseLastUsedPayload length = %d, want 2", len(got))
	}

	if got[1] != time.Unix(100, 0) {
		t.Fatalf("parseLastUsedPayload[1] = %v, want %v", got[1], time.Unix(100, 0))
	}
	if got[2] != time.Unix(200, 0) {
		t.Fatalf("parseLastUsedPayload[2] = %v, want %v", got[2], time.Unix(200, 0))
	}
}

func TestCoalesceSchedulerOutboxEvents(t *testing.T) {
	accountID := int64(10)
	groupID := int64(5)
	events := []SchedulerOutboxEvent{
		{
			ID:        11,
			EventType: SchedulerOutboxEventAccountChanged,
			AccountID: &accountID,
			Payload: map[string]any{
				"group_ids": []any{float64(1), float64(2)},
			},
			CreatedAt: time.Unix(10, 0),
		},
		{
			ID:        12,
			EventType: SchedulerOutboxEventAccountGroupsChanged,
			AccountID: &accountID,
			Payload: map[string]any{
				"group_ids": []any{float64(2), float64(3)},
			},
			CreatedAt: time.Unix(11, 0),
		},
		{
			ID:        13,
			EventType: SchedulerOutboxEventAccountBulkChanged,
			Payload: map[string]any{
				"account_ids": []any{float64(11), float64(12), float64(12)},
				"group_ids":   []any{float64(4)},
			},
			CreatedAt: time.Unix(12, 0),
		},
		{
			ID:        14,
			EventType: SchedulerOutboxEventGroupChanged,
			GroupID:   &groupID,
			CreatedAt: time.Unix(13, 0),
		},
		{
			ID:        15,
			EventType: SchedulerOutboxEventAccountLastUsed,
			Payload: map[string]any{
				"last_used": map[string]any{
					"10": float64(100),
					"11": float64(101),
				},
			},
			CreatedAt: time.Unix(14, 0),
		},
		{
			ID:        16,
			EventType: SchedulerOutboxEventAccountLastUsed,
			Payload: map[string]any{
				"last_used": map[string]any{
					"10": float64(110),
				},
			},
			CreatedAt: time.Unix(15, 0),
		},
	}

	got := coalesceSchedulerOutboxEvents(events)

	if got.oldest.ID != 11 {
		t.Fatalf("oldest ID = %d, want 11", got.oldest.ID)
	}
	if got.lastID != 16 {
		t.Fatalf("lastID = %d, want 16", got.lastID)
	}

	wantAccounts := []int64{10, 11, 12}
	if !reflect.DeepEqual(got.accountIDs, wantAccounts) {
		t.Fatalf("accountIDs = %v, want %v", got.accountIDs, wantAccounts)
	}

	wantGroups := []int64{1, 2, 3, 4, 5}
	if !reflect.DeepEqual(got.groupIDs, wantGroups) {
		t.Fatalf("groupIDs = %v, want %v", got.groupIDs, wantGroups)
	}

	if len(got.lastUsed) != 2 {
		t.Fatalf("lastUsed length = %d, want 2", len(got.lastUsed))
	}
	if got.lastUsed[10] != time.Unix(110, 0) {
		t.Fatalf("lastUsed[10] = %v, want %v", got.lastUsed[10], time.Unix(110, 0))
	}
	if got.lastUsed[11] != time.Unix(101, 0) {
		t.Fatalf("lastUsed[11] = %v, want %v", got.lastUsed[11], time.Unix(101, 0))
	}
}

func TestCoalesceSchedulerOutboxEventsWithFullRebuild(t *testing.T) {
	events := []SchedulerOutboxEvent{
		{ID: 1, EventType: SchedulerOutboxEventAccountChanged, AccountID: ptrInt64(1), CreatedAt: time.Unix(1, 0)},
		{ID: 2, EventType: SchedulerOutboxEventFullRebuild, CreatedAt: time.Unix(2, 0)},
	}

	got := coalesceSchedulerOutboxEvents(events)
	if !got.fullRebuild {
		t.Fatal("fullRebuild = false, want true")
	}
}

func TestSchedulerSnapshotServiceOutboxCooldowns(t *testing.T) {
	svc := &SchedulerSnapshotService{}
	start := time.Unix(1000, 0)

	if !svc.shouldLogOutboxLagWarning(start) {
		t.Fatal("first lag warning should be allowed")
	}
	if svc.shouldLogOutboxLagWarning(start.Add(10 * time.Second)) {
		t.Fatal("lag warning within cooldown should be suppressed")
	}
	if !svc.shouldLogOutboxLagWarning(start.Add(31 * time.Second)) {
		t.Fatal("lag warning after cooldown should be allowed")
	}

	if !svc.shouldTriggerOutboxRebuild(start) {
		t.Fatal("first rebuild should be allowed")
	}
	if svc.shouldTriggerOutboxRebuild(start.Add(10 * time.Second)) {
		t.Fatal("rebuild within cooldown should be suppressed")
	}
	if !svc.shouldTriggerOutboxRebuild(start.Add(31 * time.Second)) {
		t.Fatal("rebuild after cooldown should be allowed")
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
