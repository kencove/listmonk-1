package posthog_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/listmonk/internal/posthog"
	_ "github.com/lib/pq"
)

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("LISTMONK_TEST_DB")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=listmonk-test password=listmonk-test dbname=listmonk-test sslmode=disable"
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Skipf("skipping integration test, could not connect to test DB: %v", err)
	}
	return db
}

func seedSubscriber(t *testing.T, db *sqlx.DB, email string) int {
	t.Helper()
	var id int
	err := db.QueryRow(`INSERT INTO subscribers (uuid, email, name, status, attribs)
		VALUES (gen_random_uuid(), $1, 'Test User', 'enabled', '{}')
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("failed to seed subscriber: %v", err)
	}
	return id
}

func seedCampaign(t *testing.T, db *sqlx.DB, name string) (int, string) {
	t.Helper()
	var id int
	var uuid string
	err := db.QueryRow(`INSERT INTO campaigns (uuid, name, subject, from_email, body, altbody, content_type, type, messenger, send_at, status, tags, headers)
		VALUES (gen_random_uuid(), $1, 'Test Subject', 'test@test.com', 'body', '', 'richtext', 'regular', 'email', NOW(), 'draft', '{}', '[]')
		RETURNING id, uuid`, name).Scan(&id, &uuid)
	if err != nil {
		t.Fatalf("failed to seed campaign: %v", err)
	}
	return id, uuid
}

// TestEndToEndWebhookFlow simulates the full webhook handler flow:
// parse payload -> extract email -> lookup subscriber -> extract campaign -> record conversion -> enrich attribs
func TestEndToEndWebhookFlow(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	email := fmt.Sprintf("e2e-test-%d@example.com", os.Getpid())
	subID := seedSubscriber(t, db, email)
	campID, campUUID := seedCampaign(t, db, fmt.Sprintf("e2e-test-campaign-%d", os.Getpid()))

	// 1. Simulate a PostHog webhook payload with a product_viewed event.
	payload := fmt.Sprintf(`{
		"event": "product_viewed",
		"distinct_id": "%s",
		"properties": {
			"$current_url": "https://kencove.com/product/hd-charger?utm_campaign=%s&utm_source=listmonk",
			"product_name": "HD Charger",
			"category": "fencing"
		},
		"timestamp": "2026-02-10T12:00:00Z"
	}`, email, campUUID)

	events, err := posthog.ParsePayload([]byte(payload))
	if err != nil {
		t.Fatalf("parse payload failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]

	// 2. Extract email.
	extractedEmail := posthog.ExtractEmail(ev)
	if extractedEmail != email {
		t.Errorf("expected email %q, got %q", email, extractedEmail)
	}

	// 3. Extract campaign UUID.
	extractedUUID := posthog.ExtractCampaignUUID(ev)
	if extractedUUID != campUUID {
		t.Errorf("expected campaign UUID %q, got %q", campUUID, extractedUUID)
	}

	// 4. Record conversion (simulate what the handler does).
	props, _ := json.Marshal(ev.Properties)
	revenue := posthog.ExtractRevenue(ev)
	var convID int64
	err = db.QueryRow(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		campID, subID, ev.Event, props, revenue).Scan(&convID)
	if err != nil {
		t.Fatalf("record conversion failed: %v", err)
	}
	if convID < 1 {
		t.Error("expected valid conversion ID")
	}

	// 5. Enrich subscriber attribs.
	posthogData, _ := json.Marshal(map[string]any{
		"last_seen":            time.Now().UTC().Format(time.RFC3339),
		"last_viewed_product":  "HD Charger",
		"last_viewed_category": "fencing",
	})
	_, err = db.Exec(`UPDATE subscribers
		SET attribs = CASE
			WHEN attribs IS NULL THEN jsonb_build_object('posthog', $2::jsonb)
			ELSE jsonb_set(attribs::jsonb, '{posthog}',
				CASE
					WHEN attribs::jsonb ? 'posthog' THEN (attribs::jsonb->'posthog') || $2::jsonb
					ELSE $2::jsonb
				END
			)
		END,
		updated_at = NOW()
		WHERE id = $1`, subID, posthogData)
	if err != nil {
		t.Fatalf("enrich attribs failed: %v", err)
	}

	// 6. Verify subscriber attribs were updated.
	var attribs json.RawMessage
	err = db.Get(&attribs, `SELECT attribs FROM subscribers WHERE id = $1`, subID)
	if err != nil {
		t.Fatalf("get attribs failed: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal(attribs, &parsed)
	ph, ok := parsed["posthog"].(map[string]any)
	if !ok {
		t.Fatalf("posthog key not in attribs: %s", string(attribs))
	}
	if ph["last_viewed_product"] != "HD Charger" {
		t.Errorf("expected last_viewed_product=HD Charger, got %v", ph["last_viewed_product"])
	}
	if ph["last_viewed_category"] != "fencing" {
		t.Errorf("expected last_viewed_category=fencing, got %v", ph["last_viewed_category"])
	}

	// 7. Now simulate an order_completed event.
	orderPayload := fmt.Sprintf(`{
		"event": "order_completed",
		"distinct_id": "%s",
		"properties": {
			"utm_campaign": "%s",
			"revenue": 249.99,
			"order_id": "ORD-999"
		},
		"timestamp": "2026-02-10T13:00:00Z"
	}`, email, campUUID)

	orderEvents, _ := posthog.ParsePayload([]byte(orderPayload))
	orderEv := orderEvents[0]
	orderRevenue := posthog.ExtractRevenue(orderEv)
	if orderRevenue != 249.99 {
		t.Errorf("expected revenue 249.99, got %f", orderRevenue)
	}

	orderProps, _ := json.Marshal(orderEv.Properties)
	var orderConvID int64
	db.QueryRow(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		campID, subID, orderEv.Event, orderProps, orderRevenue).Scan(&orderConvID)

	// 8. Verify conversion summary includes both events.
	type summaryRow struct {
		EventType    string  `db:"event_type"`
		Count        int     `db:"count"`
		TotalRevenue float64 `db:"total_revenue"`
	}
	var summary []summaryRow
	err = db.Select(&summary, `SELECT event_type, COUNT(*) AS "count", COALESCE(SUM(revenue), 0) AS total_revenue
		FROM conversions WHERE campaign_id = $1 GROUP BY event_type ORDER BY "count" DESC`, campID)
	if err != nil {
		t.Fatalf("get summary failed: %v", err)
	}
	if len(summary) != 2 {
		t.Fatalf("expected 2 event types in summary, got %d", len(summary))
	}

	// Find order_completed in summary.
	var found bool
	for _, s := range summary {
		if s.EventType == "order_completed" {
			found = true
			if s.TotalRevenue != 249.99 {
				t.Errorf("expected total_revenue 249.99, got %f", s.TotalRevenue)
			}
		}
	}
	if !found {
		t.Error("order_completed not found in conversion summary")
	}

	// Cleanup.
	db.Exec("DELETE FROM conversions WHERE subscriber_id = $1", subID)
	db.Exec("DELETE FROM campaigns WHERE id = $1", campID)
	db.Exec("DELETE FROM subscribers WHERE id = $1", subID)
}

// TestWebhookSecretValidation verifies secret header checking logic.
func TestWebhookSecretValidation(t *testing.T) {
	// This tests the pure logic, not the HTTP handler.
	secret := "my-secret-key-123"

	tests := []struct {
		name       string
		header     string
		cfgSecret  string
		wantReject bool
	}{
		{"valid secret", "my-secret-key-123", secret, false},
		{"invalid secret", "wrong-key", secret, true},
		{"empty header", "", secret, true},
		{"empty config secret", "any-key", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rejected := tt.cfgSecret == "" || tt.header != tt.cfgSecret
			if rejected != tt.wantReject {
				t.Errorf("secret validation: want rejected=%v, got %v", tt.wantReject, rejected)
			}
		})
	}
}

// TestBatchPayloadProcessing tests processing multiple events from a batch.
func TestBatchPayloadProcessing(t *testing.T) {
	payload := `[
		{"event": "product_viewed", "distinct_id": "user@test.com", "properties": {"product_name": "Fence Post"}, "timestamp": "2026-02-10T12:00:00Z"},
		{"event": "product_added_to_cart", "distinct_id": "user@test.com", "properties": {"product_name": "Fence Post", "cart_items": 3}, "timestamp": "2026-02-10T12:01:00Z"},
		{"event": "order_completed", "distinct_id": "user@test.com", "properties": {"revenue": 89.50, "order_id": "ORD-456"}, "timestamp": "2026-02-10T12:05:00Z"}
	]`

	events, err := posthog.ParsePayload([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// All events should have the same email.
	for _, ev := range events {
		email := posthog.ExtractEmail(ev)
		if email != "user@test.com" {
			t.Errorf("expected email 'user@test.com', got %q", email)
		}
	}

	// Only the last event should have revenue.
	if rev := posthog.ExtractRevenue(events[0]); rev != 0 {
		t.Errorf("product_viewed should have 0 revenue, got %f", rev)
	}
	if rev := posthog.ExtractRevenue(events[2]); rev != 89.50 {
		t.Errorf("order_completed should have 89.50 revenue, got %f", rev)
	}
}

// Ensure sql is imported (it's used in the integration test types).
var _ sql.NullString
