package models

import (
	"encoding/json"

	null "gopkg.in/volatiletech/null.v6"
)

const (
	ABTestStatusDraft           = "draft"
	ABTestStatusRunning         = "running"
	ABTestStatusSelectingWinner = "selecting_winner"
	ABTestStatusFinished        = "finished"
	ABTestStatusCancelled       = "cancelled"

	ABTestMetricOpens  = "opens"
	ABTestMetricClicks = "clicks"
)

// ABTest represents an A/B test that groups multiple campaign variants.
type ABTest struct {
	Base

	UUID             string          `db:"uuid" json:"uuid"`
	Name             string          `db:"name" json:"name"`
	Status           string          `db:"status" json:"status"`
	TestPct          int             `db:"test_pct" json:"test_pct"`
	WinnerMetric     string          `db:"winner_metric" json:"winner_metric"`
	TestDuration     string          `db:"test_duration" json:"test_duration"`
	WinnerVariantID  null.Int        `db:"winner_variant_id" json:"winner_variant_id"`
	TotalSubscribers int             `db:"total_subscribers" json:"total_subscribers"`
	Variants         json.RawMessage `db:"variants" json:"variants"`
}

// ABTestVariant represents a single variant in an A/B test.
type ABTestVariant struct {
	ID             int    `db:"id" json:"id"`
	ABTestID       int    `db:"ab_test_id" json:"ab_test_id"`
	CampaignID     int    `db:"campaign_id" json:"campaign_id"`
	ListID         int    `db:"list_id" json:"list_id"`
	VariantLabel   string `db:"variant_label" json:"variant_label"`
	SplitPct       int    `db:"split_pct" json:"split_pct"`
	CampaignName   string `db:"campaign_name" json:"campaign_name,omitempty"`
	CampaignSubj   string `db:"campaign_subject" json:"campaign_subject,omitempty"`
	CampaignStatus string `db:"campaign_status" json:"campaign_status,omitempty"`
	Sent           int    `db:"sent" json:"sent,omitempty"`
}

// ABTestResult represents comparative results for a single variant.
type ABTestResult struct {
	VariantID        int     `db:"variant_id" json:"variant_id"`
	VariantLabel     string  `db:"variant_label" json:"variant_label"`
	CampaignID       int     `db:"campaign_id" json:"campaign_id"`
	SplitPct         int     `db:"split_pct" json:"split_pct"`
	CampaignName     string  `db:"campaign_name" json:"campaign_name"`
	CampaignSubject  string  `db:"campaign_subject" json:"campaign_subject"`
	Sent             int     `db:"sent" json:"sent"`
	TotalViews       int     `db:"total_views" json:"total_views"`
	UniqueViews      int     `db:"unique_views" json:"unique_views"`
	TotalClicks      int     `db:"total_clicks" json:"total_clicks"`
	UniqueClicks     int     `db:"unique_clicks" json:"unique_clicks"`
	TotalBounces     int     `db:"total_bounces" json:"total_bounces"`
	OpenRate         float64 `db:"open_rate" json:"open_rate"`
	ClickRate        float64 `db:"click_rate" json:"click_rate"`
	ClickToOpenRate  float64 `db:"click_to_open_rate" json:"click_to_open_rate"`
}
