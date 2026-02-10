package core_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// testDB returns a connection to the test database.
// Set LISTMONK_TEST_DB env var to override, defaults to local test db.
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

// seedTestSubscriber creates a test subscriber and returns its ID.
func seedTestSubscriber(t *testing.T, db *sqlx.DB, email string) int {
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

// seedTestCampaign creates a test campaign and returns its ID.
func seedTestCampaign(t *testing.T, db *sqlx.DB, name string) int {
	t.Helper()
	var id int
	err := db.QueryRow(`INSERT INTO campaigns (uuid, name, subject, from_email, body, altbody, content_type, type, messenger, send_at, status, tags, headers)
		VALUES (gen_random_uuid(), $1, 'Test Subject', 'test@test.com', 'body', '', 'richtext', 'regular', 'email', NOW(), 'draft', '{}', '[]')
		RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatalf("failed to seed campaign: %v", err)
	}
	return id
}

func TestInsertConversion(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	subID := seedTestSubscriber(t, db, fmt.Sprintf("conv-test-%d@example.com", os.Getpid()))
	campID := seedTestCampaign(t, db, fmt.Sprintf("conv-test-campaign-%d", os.Getpid()))

	props := json.RawMessage(`{"product_name":"Widget","category":"tools"}`)
	var convID int64
	err := db.QueryRow(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		campID, subID, "product_viewed", props, 0.0).Scan(&convID)
	if err != nil {
		t.Fatalf("insert conversion failed: %v", err)
	}
	if convID < 1 {
		t.Error("expected valid conversion ID")
	}

	// Insert another with revenue.
	var convID2 int64
	err = db.QueryRow(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		campID, subID, "order_completed", json.RawMessage(`{"order_id":"ORD-123"}`), 99.99).Scan(&convID2)
	if err != nil {
		t.Fatalf("insert order conversion failed: %v", err)
	}

	// Verify get-campaign-conversions.
	type convRow struct {
		ID              int64           `db:"id"`
		CampaignID      sql.NullInt64   `db:"campaign_id"`
		SubscriberID    int             `db:"subscriber_id"`
		EventType       string          `db:"event_type"`
		EventProperties json.RawMessage `db:"event_properties"`
		Revenue         float64         `db:"revenue"`
		SubscriberEmail string          `db:"subscriber_email"`
	}
	var convs []convRow
	err = db.Select(&convs, `SELECT c.id, c.campaign_id, c.subscriber_id, c.event_type, c.event_properties, c.revenue,
		s.email AS subscriber_email
		FROM conversions c LEFT JOIN subscribers s ON c.subscriber_id = s.id
		WHERE c.campaign_id = $1 ORDER BY c.created_at DESC`, campID)
	if err != nil {
		t.Fatalf("get campaign conversions failed: %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("expected 2 conversions, got %d", len(convs))
	}
	// Most recent first.
	if convs[0].EventType != "order_completed" {
		t.Errorf("expected first event to be order_completed, got %q", convs[0].EventType)
	}
	if convs[0].Revenue != 99.99 {
		t.Errorf("expected revenue 99.99, got %f", convs[0].Revenue)
	}

	// Cleanup.
	db.Exec("DELETE FROM conversions WHERE subscriber_id = $1", subID)
	db.Exec("DELETE FROM campaigns WHERE id = $1", campID)
	db.Exec("DELETE FROM subscribers WHERE id = $1", subID)
}

func TestGetCampaignConversionSummary(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	subID := seedTestSubscriber(t, db, fmt.Sprintf("summary-test-%d@example.com", os.Getpid()))
	campID := seedTestCampaign(t, db, fmt.Sprintf("summary-test-campaign-%d", os.Getpid()))

	// Insert multiple events.
	for i := 0; i < 3; i++ {
		db.Exec(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
			VALUES ($1, $2, 'product_viewed', '{}', 0)`, campID, subID)
	}
	db.Exec(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, 'order_completed', '{}', 50.00)`, campID, subID)
	db.Exec(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, 'order_completed', '{}', 75.50)`, campID, subID)

	type summaryRow struct {
		EventType    string  `db:"event_type"`
		Count        int     `db:"count"`
		TotalRevenue float64 `db:"total_revenue"`
	}
	var summary []summaryRow
	err := db.Select(&summary, `SELECT event_type, COUNT(*) AS "count", COALESCE(SUM(revenue), 0) AS total_revenue
		FROM conversions WHERE campaign_id = $1 GROUP BY event_type ORDER BY "count" DESC`, campID)
	if err != nil {
		t.Fatalf("get summary failed: %v", err)
	}
	if len(summary) != 2 {
		t.Fatalf("expected 2 event types, got %d", len(summary))
	}

	// product_viewed: 3 events, 0 revenue.
	if summary[0].EventType != "product_viewed" || summary[0].Count != 3 || summary[0].TotalRevenue != 0 {
		t.Errorf("unexpected product_viewed summary: %+v", summary[0])
	}
	// order_completed: 2 events, 125.50 revenue.
	if summary[1].EventType != "order_completed" || summary[1].Count != 2 || summary[1].TotalRevenue != 125.50 {
		t.Errorf("unexpected order_completed summary: %+v", summary[1])
	}

	// Cleanup.
	db.Exec("DELETE FROM conversions WHERE subscriber_id = $1", subID)
	db.Exec("DELETE FROM campaigns WHERE id = $1", campID)
	db.Exec("DELETE FROM subscribers WHERE id = $1", subID)
}

func TestUpdateSubscriberPostHogAttribs(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	subID := seedTestSubscriber(t, db, fmt.Sprintf("attribs-test-%d@example.com", os.Getpid()))

	// First update: set posthog attribs.
	posthogData := `{"last_seen":"2026-02-10T12:00:00Z","last_viewed_category":"fencing"}`
	_, err := db.Exec(`UPDATE subscribers
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
		t.Fatalf("first attribs update failed: %v", err)
	}

	// Verify first update.
	var attribs json.RawMessage
	err = db.Get(&attribs, `SELECT attribs FROM subscribers WHERE id = $1`, subID)
	if err != nil {
		t.Fatalf("get attribs failed: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal(attribs, &parsed)
	ph, ok := parsed["posthog"].(map[string]any)
	if !ok {
		t.Fatalf("posthog key not found in attribs: %s", string(attribs))
	}
	if ph["last_viewed_category"] != "fencing" {
		t.Errorf("expected last_viewed_category=fencing, got %v", ph["last_viewed_category"])
	}

	// Second update: merge additional data (should preserve last_viewed_category).
	posthogData2 := `{"last_seen":"2026-02-10T13:00:00Z","cart_status":"has_items"}`
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
		WHERE id = $1`, subID, posthogData2)
	if err != nil {
		t.Fatalf("second attribs update failed: %v", err)
	}

	// Verify merge: should have both last_viewed_category AND cart_status.
	err = db.Get(&attribs, `SELECT attribs FROM subscribers WHERE id = $1`, subID)
	if err != nil {
		t.Fatalf("get attribs after merge failed: %v", err)
	}
	json.Unmarshal(attribs, &parsed)
	ph = parsed["posthog"].(map[string]any)
	if ph["last_viewed_category"] != "fencing" {
		t.Errorf("merge lost last_viewed_category: %v", ph)
	}
	if ph["cart_status"] != "has_items" {
		t.Errorf("merge missing cart_status: %v", ph)
	}
	if ph["last_seen"] != "2026-02-10T13:00:00Z" {
		t.Errorf("merge didn't update last_seen: %v", ph)
	}

	// Cleanup.
	db.Exec("DELETE FROM subscribers WHERE id = $1", subID)
}

func TestGetSubscriberConversions(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	subID := seedTestSubscriber(t, db, fmt.Sprintf("sub-conv-test-%d@example.com", os.Getpid()))
	campID1 := seedTestCampaign(t, db, fmt.Sprintf("sub-conv-camp1-%d", os.Getpid()))
	campID2 := seedTestCampaign(t, db, fmt.Sprintf("sub-conv-camp2-%d", os.Getpid()))

	db.Exec(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, 'product_viewed', '{}', 0)`, campID1, subID)
	db.Exec(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES ($1, $2, 'order_completed', '{}', 200.00)`, campID2, subID)

	type convRow struct {
		ID           int64          `db:"id"`
		CampaignID   sql.NullInt64  `db:"campaign_id"`
		SubscriberID int            `db:"subscriber_id"`
		EventType    string         `db:"event_type"`
		Revenue      float64        `db:"revenue"`
		CampaignName sql.NullString `db:"campaign_name"`
	}
	var convs []convRow
	err := db.Select(&convs, `SELECT c.id, c.campaign_id, c.subscriber_id, c.event_type, c.revenue,
		camp.name AS campaign_name
		FROM conversions c LEFT JOIN campaigns camp ON c.campaign_id = camp.id
		WHERE c.subscriber_id = $1 ORDER BY c.created_at DESC`, subID)
	if err != nil {
		t.Fatalf("get subscriber conversions failed: %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("expected 2 conversions, got %d", len(convs))
	}

	// Cleanup.
	db.Exec("DELETE FROM conversions WHERE subscriber_id = $1", subID)
	db.Exec("DELETE FROM campaigns WHERE id = $1", campID1)
	db.Exec("DELETE FROM campaigns WHERE id = $1", campID2)
	db.Exec("DELETE FROM subscribers WHERE id = $1", subID)
}

func TestConversionNullCampaign(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	subID := seedTestSubscriber(t, db, fmt.Sprintf("null-camp-test-%d@example.com", os.Getpid()))

	// Insert conversion with NULL campaign_id (anonymous visit).
	var convID int64
	err := db.QueryRow(`INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
		VALUES (NULL, $1, 'product_viewed', '{}', 0) RETURNING id`, subID).Scan(&convID)
	if err != nil {
		t.Fatalf("insert conversion with null campaign failed: %v", err)
	}
	if convID < 1 {
		t.Error("expected valid conversion ID for null campaign")
	}

	// Cleanup.
	db.Exec("DELETE FROM conversions WHERE subscriber_id = $1", subID)
	db.Exec("DELETE FROM subscribers WHERE id = $1", subID)
}
