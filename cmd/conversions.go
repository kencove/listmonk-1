package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// GetCampaignConversions returns conversion events for a campaign.
func (a *App) GetCampaignConversions(c echo.Context) error {
	out, err := a.core.GetCampaignConversions(getID(c))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// GetCampaignConversionSummary returns aggregated conversion stats for a campaign.
func (a *App) GetCampaignConversionSummary(c echo.Context) error {
	out, err := a.core.GetCampaignConversionSummary(getID(c))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// GetSubscriberConversions returns conversion events for a subscriber.
func (a *App) GetSubscriberConversions(c echo.Context) error {
	out, err := a.core.GetSubscriberConversions(getID(c))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}
