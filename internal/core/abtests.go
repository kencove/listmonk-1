package core

import (
	"fmt"
	"math/rand"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// GetABTests retrieves all A/B tests.
func (c *Core) GetABTests() ([]models.ABTest, error) {
	out := []models.ABTest{}
	if err := c.q.GetABTests.Select(&out, 0); err != nil {
		c.log.Printf("error fetching A/B tests: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "A/B tests", "error", pqErrMsg(err)))
	}
	return out, nil
}

// GetABTest retrieves a single A/B test by ID.
func (c *Core) GetABTest(id int) (models.ABTest, error) {
	var out []models.ABTest
	if err := c.q.GetABTest.Select(&out, id); err != nil {
		c.log.Printf("error fetching A/B test: %v", err)
		return models.ABTest{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "A/B test", "error", pqErrMsg(err)))
	}
	if len(out) == 0 {
		return models.ABTest{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.notFound", "name", "A/B test"))
	}
	return out[0], nil
}

// CreateABTest creates a new A/B test with its variants.
func (c *Core) CreateABTest(name string, testPct int, winnerMetric, testDuration string,
	campaignIDs []int, splitPcts []int) (models.ABTest, error) {

	if len(campaignIDs) < 2 {
		return models.ABTest{}, echo.NewHTTPError(http.StatusBadRequest, "at least 2 campaign variants required")
	}
	if len(campaignIDs) != len(splitPcts) {
		return models.ABTest{}, echo.NewHTTPError(http.StatusBadRequest, "campaign_ids and split_pcts must have equal length")
	}

	// Validate total split is 100.
	total := 0
	for _, p := range splitPcts {
		total += p
	}
	if total != 100 {
		return models.ABTest{}, echo.NewHTTPError(http.StatusBadRequest, "split percentages must sum to 100")
	}

	// Validate all campaigns exist and are in draft status.
	for _, cID := range campaignIDs {
		camp, err := c.GetCampaign(cID, "", "")
		if err != nil {
			return models.ABTest{}, err
		}
		if camp.Status != models.CampaignStatusDraft {
			return models.ABTest{}, echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("campaign %d (%s) must be in draft status", cID, camp.Name))
		}
	}

	uu, err := uuid.NewV4()
	if err != nil {
		return models.ABTest{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUUID", "error", err.Error()))
	}

	var testID int
	if err := c.q.CreateABTest.Get(&testID, uu, name, testPct, winnerMetric, testDuration); err != nil {
		c.log.Printf("error creating A/B test: %v", err)
		return models.ABTest{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "A/B test", "error", pqErrMsg(err)))
	}

	// Create variants.
	labels := []string{"A", "B", "C", "D", "E"}
	for i, cID := range campaignIDs {
		label := labels[i%len(labels)]
		var variantID int
		if err := c.q.CreateABTestVariant.Get(&variantID, testID, cID, label, splitPcts[i]); err != nil {
			c.log.Printf("error creating A/B test variant: %v", err)
			return models.ABTest{}, echo.NewHTTPError(http.StatusInternalServerError,
				c.i18n.Ts("globals.messages.errorCreating", "name", "A/B test variant", "error", pqErrMsg(err)))
		}
	}

	return c.GetABTest(testID)
}

// StartABTest starts an A/B test by:
// 1. Gathering subscribers from the variant campaigns' lists
// 2. Creating temporary lists for each variant
// 3. Randomly assigning subscribers to variant lists
// 4. Starting each variant campaign
func (c *Core) StartABTest(id int, listIDs []int) error {
	test, err := c.GetABTest(id)
	if err != nil {
		return err
	}

	if test.Status != models.ABTestStatusDraft {
		return echo.NewHTTPError(http.StatusBadRequest, "A/B test must be in draft status to start")
	}

	if len(listIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one list_id is required")
	}

	// Parse variants from the JSON.
	variants, err := c.getABTestVariants(id)
	if err != nil {
		return err
	}

	// Get all eligible subscriber IDs from the specified lists.
	subIDs, err := c.getSubscriberIDsFromLists(listIDs)
	if err != nil {
		return err
	}

	if len(subIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no subscribers found in the specified lists")
	}

	// Calculate the test population size.
	testSize := len(subIDs) * test.TestPct / 100
	if testSize < len(variants) {
		testSize = len(variants)
	}

	// Shuffle and split.
	rand.Shuffle(len(subIDs), func(i, j int) {
		subIDs[i], subIDs[j] = subIDs[j], subIDs[i]
	})

	testSubs := subIDs[:testSize]

	// Split test subscribers among variants based on split percentages.
	variantSubs := splitSubscribers(testSubs, variants)

	// For each variant, create a temporary list and assign subscribers.
	for i, v := range variants {
		// Create a temporary list for this variant.
		list, err := c.CreateList(models.List{
			Name:   fmt.Sprintf("_ab_test_%d_variant_%s", id, v.VariantLabel),
			Type:   models.ListTypePrivate,
			Optin:  models.ListOptinSingle,
			Status: models.ListStatusActive,
		})
		if err != nil {
			return fmt.Errorf("error creating variant list: %v", err)
		}

		// Update variant with list_id.
		if _, err := c.q.UpdateABTestVariantList.Exec(v.ID, list.ID); err != nil {
			return fmt.Errorf("error updating variant list: %v", err)
		}

		// Add subscribers to the variant list.
		if len(variantSubs[i]) > 0 {
			if err := c.AddSubscriptions(variantSubs[i], []int{list.ID}, "confirmed"); err != nil {
				return fmt.Errorf("error adding subscribers to variant list: %v", err)
			}
		}

		// Update the variant campaign to target this list.
		camp, err := c.GetCampaign(v.CampaignID, "", "")
		if err != nil {
			return err
		}

		// Update campaign with the variant list and start it.
		if _, err := c.UpdateCampaign(v.CampaignID, camp, []int{list.ID}, nil); err != nil {
			return fmt.Errorf("error updating variant campaign: %v", err)
		}

		if _, err := c.UpdateCampaignStatus(v.CampaignID, models.CampaignStatusRunning); err != nil {
			return fmt.Errorf("error starting variant campaign: %v", err)
		}
	}

	// Update test status and total subscribers.
	if _, err := c.q.UpdateABTestTotalSubscribers.Exec(id, len(subIDs)); err != nil {
		return fmt.Errorf("error updating test subscriber count: %v", err)
	}
	if _, err := c.q.UpdateABTestStatus.Exec(id, models.ABTestStatusRunning); err != nil {
		return fmt.Errorf("error updating test status: %v", err)
	}

	return nil
}

// GetABTestResults returns comparative performance results for all variants.
func (c *Core) GetABTestResults(id int) ([]models.ABTestResult, error) {
	out := []models.ABTestResult{}
	if err := c.q.GetABTestResults.Select(&out, id); err != nil {
		c.log.Printf("error fetching A/B test results: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "A/B test results", "error", pqErrMsg(err)))
	}
	return out, nil
}

// SelectABTestWinner declares a winner for the A/B test.
func (c *Core) SelectABTestWinner(testID, variantID int) error {
	test, err := c.GetABTest(testID)
	if err != nil {
		return err
	}

	if test.Status != models.ABTestStatusRunning && test.Status != models.ABTestStatusSelectingWinner {
		return echo.NewHTTPError(http.StatusBadRequest, "A/B test must be running to select a winner")
	}

	if _, err := c.q.UpdateABTestWinner.Exec(testID, variantID); err != nil {
		c.log.Printf("error setting A/B test winner: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "A/B test", "error", pqErrMsg(err)))
	}

	return nil
}

// DeleteABTest deletes a draft A/B test.
func (c *Core) DeleteABTest(id int) error {
	if _, err := c.q.DeleteABTest.Exec(id); err != nil {
		c.log.Printf("error deleting A/B test: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "A/B test", "error", pqErrMsg(err)))
	}
	return nil
}

// getABTestVariants parses variant info from a test's JSON.
func (c *Core) getABTestVariants(testID int) ([]models.ABTestVariant, error) {
	out := []models.ABTestVariant{}
	if err := c.db.Select(&out, `SELECT id, ab_test_id, campaign_id, COALESCE(list_id, 0) as list_id,
		variant_label, split_pct FROM ab_test_variants WHERE ab_test_id = $1 ORDER BY id`, testID); err != nil {
		return nil, fmt.Errorf("error fetching variants: %v", err)
	}
	return out, nil
}

// getSubscriberIDsFromLists returns all confirmed subscriber IDs from the given lists.
func (c *Core) getSubscriberIDsFromLists(listIDs []int) ([]int, error) {
	var out []int
	err := c.db.Select(&out, `
		SELECT DISTINCT s.id FROM subscribers s
		JOIN subscriber_lists sl ON s.id = sl.subscriber_id
		WHERE sl.list_id = ANY($1)
			AND sl.status != 'unsubscribed'
			AND s.status != 'blocklisted'
		ORDER BY s.id
	`, pq.Array(listIDs))
	if err != nil {
		return nil, fmt.Errorf("error fetching subscriber IDs: %v", err)
	}
	return out, nil
}

// splitSubscribers splits a list of subscriber IDs among variants based on their split percentages.
func splitSubscribers(subIDs []int, variants []models.ABTestVariant) [][]int {
	result := make([][]int, len(variants))
	for i := range result {
		result[i] = []int{}
	}

	offset := 0
	for i, v := range variants {
		count := len(subIDs) * v.SplitPct / 100
		// Last variant gets the remainder.
		if i == len(variants)-1 {
			count = len(subIDs) - offset
		}
		if offset+count > len(subIDs) {
			count = len(subIDs) - offset
		}
		result[i] = subIDs[offset : offset+count]
		offset += count
	}

	return result
}
