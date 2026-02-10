package main

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/knadh/listmonk/internal/posthog"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"gopkg.in/volatiletech/null.v6"
)

// maxPostHogBodySize is the maximum allowed PostHog webhook payload size (1MB).
const maxPostHogBodySize = 1 << 20

// PostHogWebhook handles incoming PostHog webhook events for conversion tracking.
func (a *App) PostHogWebhook(c echo.Context) error {
	// Validate shared secret using constant-time comparison to prevent timing attacks.
	secret := a.cfg.PostHogWebhookSecret
	if secret == "" {
		return echo.NewHTTPError(http.StatusForbidden, "PostHog webhook not configured")
	}
	headerSecret := c.Request().Header.Get("X-PostHog-Webhook-Secret")
	if subtle.ConstantTimeCompare([]byte(headerSecret), []byte(secret)) != 1 {
		return echo.NewHTTPError(http.StatusForbidden, "invalid webhook secret")
	}

	// Limit body size to prevent DoS.
	rawReq, err := io.ReadAll(io.LimitReader(c.Request().Body, maxPostHogBodySize))
	if err != nil {
		a.log.Printf("error reading PostHog webhook body: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "error reading request body")
	}

	// PostHog can send a single event object or an array.
	events, err := posthog.ParsePayload(rawReq)
	if err != nil {
		a.log.Printf("error parsing PostHog webhook payload: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	for _, ev := range events {
		a.processPostHogEvent(ev)
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// processPostHogEvent processes a single PostHog event: resolves the subscriber,
// records the conversion, and enriches subscriber attribs.
func (a *App) processPostHogEvent(ev models.PostHogEvent) {
	// Resolve subscriber email from the event.
	email := posthog.ExtractEmail(ev)
	if email == "" {
		return
	}

	// Look up subscriber by email.
	sub, err := a.core.GetSubscriber(0, "", email)
	if err != nil {
		a.log.Printf("PostHog webhook: subscriber lookup failed for %q: %v", email, err)
		return
	}

	// Extract campaign UUID from UTM params in the event URL.
	var campID null.Int
	if campUUID := posthog.ExtractCampaignUUID(ev); campUUID != "" {
		camp, err := a.core.GetCampaign(0, campUUID, "")
		if err == nil {
			campID = null.IntFrom(camp.ID)
		}
	}

	// Extract revenue from event properties.
	revenue := posthog.ExtractRevenue(ev)

	// Serialize event properties.
	props, err := json.Marshal(ev.Properties)
	if err != nil {
		props = []byte("{}")
	}

	// Record the conversion.
	if _, err := a.core.RecordConversion(campID, sub.ID, ev.Event, props, revenue); err != nil {
		a.log.Printf("error recording PostHog conversion for subscriber %d: %v", sub.ID, err)
		return
	}

	// Enrich subscriber attribs with PostHog data.
	a.enrichSubscriberAttribs(sub.ID, ev)
}

// enrichSubscriberAttribs updates the subscriber's PostHog attribs based on the event.
func (a *App) enrichSubscriberAttribs(subscriberID int, ev models.PostHogEvent) {
	data := map[string]any{
		"last_seen": time.Now().UTC().Format(time.RFC3339),
	}

	switch ev.Event {
	case models.PostHogEventProductViewed:
		if name, _ := ev.Properties["product_name"].(string); name != "" {
			data["last_viewed_product"] = name
		}
		if cat, _ := ev.Properties["category"].(string); cat != "" {
			data["last_viewed_category"] = cat
		}

	case models.PostHogEventCategoryViewed:
		if cat, _ := ev.Properties["category"].(string); cat != "" {
			data["last_viewed_category"] = cat
		}

	case models.PostHogEventProductAddToCart:
		data["cart_status"] = "has_items"
		if items, ok := ev.Properties["cart_items"].(float64); ok {
			data["cart_items"] = int(items)
		}

	case models.PostHogEventOrderCompleted:
		data["cart_status"] = "completed"
		data["last_purchase_date"] = time.Now().UTC().Format("2006-01-02")
		if rev := posthog.ExtractRevenue(ev); rev > 0 {
			data["last_purchase_revenue"] = rev
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		a.log.Printf("error marshaling PostHog attribs: %v", err)
		return
	}

	if err := a.core.UpdateSubscriberPostHogAttribs(subscriberID, jsonData); err != nil {
		a.log.Printf("error updating subscriber PostHog attribs: %v", err)
	}
}
