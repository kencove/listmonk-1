package models

import (
	"fmt"
	"html/template"
	"strings"
	txttpl "text/template"
	"time"

	null "gopkg.in/volatiletech/null.v6"
)

const (
	BaseTpl                    = "base"
	ContentTpl                 = "content"
	TemplateTypeCampaign       = "campaign"
	TemplateTypeCampaignVisual = "campaign_visual"
	TemplateTypeTx             = "tx"
)

// Template represents a reusable e-mail template.
type Template struct {
	Base

	Name string `db:"name" json:"name"`
	// Subject is only for type=tx.
	Subject    string      `db:"subject" json:"subject"`
	Type       string      `db:"type" json:"type"`
	Body       string      `db:"body" json:"body,omitempty"`
	BodySource null.String `db:"body_source" json:"body_source,omitempty"`
	IsDefault  bool        `db:"is_default" json:"is_default"`

	// Only relevant to tx (transactional) templates.
	SubjectTpl *txttpl.Template   `json:"-"`
	Tpl        *template.Template `json:"-"`
}

// Compile compiles a template body and subject (only for tx templates) and
// caches the templat references to be executed later.
func (t *Template) Compile(f template.FuncMap) error {
	tpl, err := template.New(BaseTpl).Funcs(f).Parse(t.Body)
	if err != nil {
		return fmt.Errorf("error compiling transactional template: %v", err)
	}
	t.Tpl = tpl

	// If the subject line has a template string, compile it.
	if strings.Contains(t.Subject, "{{") {
		subj := t.Subject

		subjTpl, err := txttpl.New(BaseTpl).Funcs(txttpl.FuncMap(f)).Parse(subj)
		if err != nil {
			return fmt.Errorf("error compiling subject: %v", err)
		}
		t.SubjectTpl = subjTpl
	}

	return nil
}

type CampaignStats struct {
	ID        int       `db:"id" json:"id"`
	Status    string    `db:"status" json:"status"`
	ToSend    int       `db:"to_send" json:"to_send"`
	Sent      int       `db:"sent" json:"sent"`
	Started   null.Time `db:"started_at" json:"started_at"`
	UpdatedAt null.Time `db:"updated_at" json:"updated_at"`
	Rate      int       `json:"rate"`
	NetRate   int       `json:"net_rate"`
}

type CampaignAnalyticsCount struct {
	CampaignID int       `db:"campaign_id" json:"campaign_id"`
	Count      int       `db:"count" json:"count"`
	Timestamp  time.Time `db:"timestamp" json:"timestamp"`
}

type CampaignAnalyticsLink struct {
	URL      string `db:"url" json:"url"`
	Count    int    `db:"count" json:"count"`
	Metadata JSON   `db:"metadata" json:"metadata"`
}

// CampaignAnalyticsTagCount represents click counts grouped by metadata tag key/value.
type CampaignAnalyticsTagCount struct {
	TagKey   string `db:"tag_key" json:"tag_key"`
	TagValue string `db:"tag_value" json:"tag_value"`
	Count    int    `db:"count" json:"count"`
}

// CampaignAnalyticsSummary represents a comprehensive analytics summary for a campaign.
type CampaignAnalyticsSummary struct {
	CampaignID         int       `db:"campaign_id" json:"campaign_id"`
	Name               string    `db:"name" json:"name"`
	Subject            string    `db:"subject" json:"subject"`
	Status             string    `db:"status" json:"status"`
	ToSend             int       `db:"to_send" json:"to_send"`
	Sent               int       `db:"sent" json:"sent"`
	CreatedAt          null.Time `db:"created_at" json:"created_at"`
	StartedAt          null.Time `db:"started_at" json:"started_at"`
	TotalViews         int       `db:"total_views" json:"total_views"`
	UniqueViews        int       `db:"unique_views" json:"unique_views"`
	TotalClicks        int       `db:"total_clicks" json:"total_clicks"`
	UniqueClicks       int       `db:"unique_clicks" json:"unique_clicks"`
	UniqueLinksClicked int       `db:"unique_links_clicked" json:"unique_links_clicked"`
	TotalBounces       int       `db:"total_bounces" json:"total_bounces"`
	HardBounces        int       `db:"hard_bounces" json:"hard_bounces"`
	SoftBounces        int       `db:"soft_bounces" json:"soft_bounces"`
	Complaints         int       `db:"complaints" json:"complaints"`
	OpenRate           float64   `db:"open_rate" json:"open_rate"`
	ClickRate          float64   `db:"click_rate" json:"click_rate"`
	ClickToOpenRate    float64   `db:"click_to_open_rate" json:"click_to_open_rate"`
	BounceRate         float64   `db:"bounce_rate" json:"bounce_rate"`
}

// SubscriberEngagementEvent represents a single engagement event for a subscriber.
type SubscriberEngagementEvent struct {
	CampaignID      int       `db:"campaign_id" json:"campaign_id"`
	CampaignName    string    `db:"campaign_name" json:"campaign_name"`
	CampaignSubject string    `db:"campaign_subject" json:"campaign_subject"`
	EventType       string    `db:"event_type" json:"event_type"`
	EventAt         null.Time `db:"event_at" json:"event_at"`
}

// SubscriberEngagementScore represents computed engagement metrics for a subscriber.
type SubscriberEngagementScore struct {
	Views90d        int       `db:"views_90d" json:"views_90d"`
	Clicks90d       int       `db:"clicks_90d" json:"clicks_90d"`
	LastActivityAt  null.Time `db:"last_activity_at" json:"last_activity_at"`
	EngagementScore int       `db:"engagement_score" json:"engagement_score"`
}
