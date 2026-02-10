package main

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type abTestReq struct {
	Name         string `json:"name"`
	TestPct      int    `json:"test_pct"`
	WinnerMetric string `json:"winner_metric"`
	TestDuration string `json:"test_duration"`
	CampaignIDs  []int  `json:"campaign_ids"`
	SplitPcts    []int  `json:"split_pcts"`
}

type abTestStartReq struct {
	ListIDs []int `json:"list_ids"`
}

type abTestWinnerReq struct {
	VariantID int `json:"variant_id"`
}

// GetABTests returns all A/B tests.
func (a *App) GetABTests(c echo.Context) error {
	out, err := a.core.GetABTests()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{out})
}

// GetABTest returns a single A/B test.
func (a *App) GetABTest(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if id < 1 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidID"))
	}

	out, err := a.core.GetABTest(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{out})
}

// CreateABTest creates a new A/B test.
func (a *App) CreateABTest(c echo.Context) error {
	var o abTestReq
	if err := c.Bind(&o); err != nil {
		return err
	}

	if o.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("globals.messages.missingFields", "name", "`name`"))
	}
	if o.TestPct < 1 || o.TestPct > 100 {
		o.TestPct = 30
	}
	if o.WinnerMetric == "" {
		o.WinnerMetric = "clicks"
	}
	if o.TestDuration == "" {
		o.TestDuration = "4 hours"
	}

	out, err := a.core.CreateABTest(o.Name, o.TestPct, o.WinnerMetric, o.TestDuration, o.CampaignIDs, o.SplitPcts)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// StartABTest starts an A/B test.
func (a *App) StartABTest(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if id < 1 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidID"))
	}

	var o abTestStartReq
	if err := c.Bind(&o); err != nil {
		return err
	}

	if err := a.core.StartABTest(id, o.ListIDs); err != nil {
		return err
	}

	out, err := a.core.GetABTest(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{out})
}

// GetABTestResults returns comparative results for an A/B test.
func (a *App) GetABTestResults(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if id < 1 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidID"))
	}

	out, err := a.core.GetABTestResults(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{out})
}

// SelectABTestWinner declares a winner for an A/B test.
func (a *App) SelectABTestWinner(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if id < 1 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidID"))
	}

	var o abTestWinnerReq
	if err := c.Bind(&o); err != nil {
		return err
	}

	if err := a.core.SelectABTestWinner(id, o.VariantID); err != nil {
		return err
	}

	out, err := a.core.GetABTest(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{out})
}

// DeleteABTest deletes a draft A/B test.
func (a *App) DeleteABTest(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if id < 1 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidID"))
	}

	if err := a.core.DeleteABTest(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}
