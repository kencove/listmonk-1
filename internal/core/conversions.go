package core

import (
	"encoding/json"
	"net/http"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"gopkg.in/volatiletech/null.v6"
)

// RecordConversion inserts a conversion event into the database.
// campaignID is nullable — pass null.Int{} when no campaign is associated.
func (c *Core) RecordConversion(campaignID null.Int, subscriberID int, eventType string, properties json.RawMessage, revenue float64) (int64, error) {
	var id int64
	if err := c.q.InsertConversion.Get(&id, campaignID, subscriberID, eventType, properties, revenue); err != nil {
		c.log.Printf("error recording conversion: %v", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "conversion", "error", pqErrMsg(err)))
	}
	return id, nil
}

// UpdateSubscriberPostHogAttribs merges PostHog behavioral data into
// subscriber attribs without overwriting other keys.
func (c *Core) UpdateSubscriberPostHogAttribs(subscriberID int, posthogData json.RawMessage) error {
	if _, err := c.q.UpdateSubscriberPostHogAttribs.Exec(subscriberID, posthogData); err != nil {
		c.log.Printf("error updating subscriber PostHog attribs: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.subscriber}", "error", pqErrMsg(err)))
	}
	return nil
}

// GetCampaignConversions returns conversion events for a campaign.
func (c *Core) GetCampaignConversions(campaignID int) ([]models.Conversion, error) {
	out := []models.Conversion{}
	if err := c.q.GetCampaignConversions.Select(&out, campaignID); err != nil {
		c.log.Printf("error fetching campaign conversions: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "conversions", "error", pqErrMsg(err)))
	}
	return out, nil
}

// GetCampaignConversionSummary returns aggregated conversion stats for a campaign.
func (c *Core) GetCampaignConversionSummary(campaignID int) ([]models.ConversionSummary, error) {
	out := []models.ConversionSummary{}
	if err := c.q.GetCampaignConversionSummary.Select(&out, campaignID); err != nil {
		c.log.Printf("error fetching campaign conversion summary: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "conversions", "error", pqErrMsg(err)))
	}
	return out, nil
}

// GetSubscriberConversions returns conversion events for a subscriber.
func (c *Core) GetSubscriberConversions(subscriberID int) ([]models.Conversion, error) {
	out := []models.Conversion{}
	if err := c.q.GetSubscriberConversions.Select(&out, subscriberID); err != nil {
		c.log.Printf("error fetching subscriber conversions: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "conversions", "error", pqErrMsg(err)))
	}
	return out, nil
}
