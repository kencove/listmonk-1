package models

import (
	"encoding/json"
	"time"

	"gopkg.in/volatiletech/null.v6"
)

// PostHog event type constants.
const (
	PostHogEventProductViewed    = "product_viewed"
	PostHogEventProductAddToCart = "product_added_to_cart"
	PostHogEventOrderCompleted   = "order_completed"
	PostHogEventPageview         = "$pageview"
	PostHogEventCategoryViewed   = "category_viewed"
)

// PostHogEvent represents a single event from PostHog's webhook payload.
type PostHogEvent struct {
	Event      string                 `json:"event"`
	DistinctID string                 `json:"distinct_id"`
	Properties map[string]any `json:"properties"`
	Timestamp  string                 `json:"timestamp"`
}

// PostHogWebhookPayload represents the webhook payload from PostHog.
// PostHog sends an array of events or a single event wrapped in an object.
type PostHogWebhookPayload struct {
	Data []PostHogEvent `json:"data"`
}

// Conversion represents a post-click conversion event tracked via PostHog.
type Conversion struct {
	ID              int64           `db:"id" json:"id"`
	CampaignID      null.Int        `db:"campaign_id" json:"campaign_id"`
	SubscriberID    int             `db:"subscriber_id" json:"subscriber_id"`
	EventType       string          `db:"event_type" json:"event_type"`
	EventProperties json.RawMessage `db:"event_properties" json:"event_properties"`
	Revenue         float64         `db:"revenue" json:"revenue"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`

	// Joined fields for API responses.
	SubscriberEmail string `db:"subscriber_email" json:"subscriber_email,omitempty"`
	CampaignName    string `db:"campaign_name" json:"campaign_name,omitempty"`
}

// ConversionSummary represents aggregated conversion stats for a campaign.
type ConversionSummary struct {
	EventType    string  `db:"event_type" json:"event_type"`
	Count        int     `db:"count" json:"count"`
	TotalRevenue float64 `db:"total_revenue" json:"total_revenue"`
}
