// Package posthog provides helpers for parsing PostHog webhook payloads
// and extracting subscriber/campaign attribution data.
package posthog

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/knadh/listmonk/models"
)

var (
	reEmail = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	reUUID  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// ParsePayload handles both single-event and batch payloads from PostHog.
func ParsePayload(raw []byte) ([]models.PostHogEvent, error) {
	// Try as a batch payload with a "data" array.
	var batch models.PostHogWebhookPayload
	if err := json.Unmarshal(raw, &batch); err == nil && len(batch.Data) > 0 {
		return batch.Data, nil
	}

	// Try as an array of events.
	var events []models.PostHogEvent
	if err := json.Unmarshal(raw, &events); err == nil && len(events) > 0 {
		return events, nil
	}

	// Try as a single event.
	var single models.PostHogEvent
	if err := json.Unmarshal(raw, &single); err == nil && single.Event != "" {
		return []models.PostHogEvent{single}, nil
	}

	return nil, fmt.Errorf("unrecognized PostHog payload format")
}

// ExtractEmail tries to find a subscriber email from PostHog event data.
func ExtractEmail(ev models.PostHogEvent) string {
	// Check properties.$set.email
	if set, ok := ev.Properties["$set"]; ok {
		if setMap, ok := set.(map[string]any); ok {
			if email, ok := setMap["email"].(string); ok && reEmail.MatchString(email) {
				return strings.ToLower(email)
			}
		}
	}

	// Check properties.email
	if email, ok := ev.Properties["email"].(string); ok && reEmail.MatchString(email) {
		return strings.ToLower(email)
	}

	// Check distinct_id if it looks like an email.
	if reEmail.MatchString(ev.DistinctID) {
		return strings.ToLower(ev.DistinctID)
	}

	return ""
}

// ExtractCampaignUUID extracts the campaign UUID from UTM params in the event's URL.
func ExtractCampaignUUID(ev models.PostHogEvent) string {
	// Check $current_url property.
	if currentURL, ok := ev.Properties["$current_url"].(string); ok {
		if u, err := url.Parse(currentURL); err == nil {
			if uuid := u.Query().Get("utm_campaign"); uuid != "" && reUUID.MatchString(uuid) {
				return uuid
			}
		}
	}

	// Check utm_campaign directly in properties (PostHog sometimes extracts these).
	if uuid, ok := ev.Properties["utm_campaign"].(string); ok && reUUID.MatchString(uuid) {
		return uuid
	}

	// Check $initial_utm_campaign.
	if uuid, ok := ev.Properties["$initial_utm_campaign"].(string); ok && reUUID.MatchString(uuid) {
		return uuid
	}

	return ""
}

// ExtractRevenue extracts revenue from event properties for order_completed events.
func ExtractRevenue(ev models.PostHogEvent) float64 {
	if ev.Event != models.PostHogEventOrderCompleted {
		return 0
	}

	switch v := ev.Properties["revenue"].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}

	switch v := ev.Properties["total"].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}

	return 0
}
