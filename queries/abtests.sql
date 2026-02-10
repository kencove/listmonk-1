-- A/B tests

-- name: create-ab-test
INSERT INTO ab_tests (uuid, name, test_pct, winner_metric, test_duration)
    VALUES($1, $2, $3, $4, $5::INTERVAL)
    RETURNING id;

-- name: get-ab-tests
SELECT ab_tests.*,
    (
        SELECT COALESCE(JSON_AGG(JSON_BUILD_OBJECT(
            'id', v.id,
            'campaign_id', v.campaign_id,
            'list_id', v.list_id,
            'variant_label', v.variant_label,
            'split_pct', v.split_pct,
            'campaign_name', c.name,
            'campaign_subject', c.subject,
            'campaign_status', c.status,
            'sent', c.sent
        ) ORDER BY v.id), '[]')
        FROM ab_test_variants v
        LEFT JOIN campaigns c ON c.id = v.campaign_id
        WHERE v.ab_test_id = ab_tests.id
    ) AS variants
FROM ab_tests
WHERE ($1 = 0 OR ab_tests.id = $1)
ORDER BY ab_tests.created_at DESC;

-- name: get-ab-test
SELECT ab_tests.*,
    (
        SELECT COALESCE(JSON_AGG(JSON_BUILD_OBJECT(
            'id', v.id,
            'campaign_id', v.campaign_id,
            'list_id', v.list_id,
            'variant_label', v.variant_label,
            'split_pct', v.split_pct,
            'campaign_name', c.name,
            'campaign_subject', c.subject,
            'campaign_status', c.status,
            'sent', c.sent
        ) ORDER BY v.id), '[]')
        FROM ab_test_variants v
        LEFT JOIN campaigns c ON c.id = v.campaign_id
        WHERE v.ab_test_id = ab_tests.id
    ) AS variants
FROM ab_tests
WHERE ab_tests.id = $1;

-- name: create-ab-test-variant
INSERT INTO ab_test_variants (ab_test_id, campaign_id, variant_label, split_pct)
    VALUES($1, $2, $3, $4)
    RETURNING id;

-- name: update-ab-test-variant-list
UPDATE ab_test_variants SET list_id = $2 WHERE id = $1;

-- name: update-ab-test-status
UPDATE ab_tests SET status = $2::ab_test_status, updated_at = NOW() WHERE id = $1;

-- name: update-ab-test-winner
UPDATE ab_tests SET winner_variant_id = $2, status = 'finished', updated_at = NOW() WHERE id = $1;

-- name: update-ab-test-total-subscribers
UPDATE ab_tests SET total_subscribers = $2, updated_at = NOW() WHERE id = $1;

-- name: get-ab-test-results
-- Returns comparative results for all variants in an A/B test.
WITH variant_campaigns AS (
    SELECT v.id AS variant_id, v.variant_label, v.campaign_id, v.split_pct
    FROM ab_test_variants v
    WHERE v.ab_test_id = $1
),
variant_views AS (
    SELECT vc.variant_id, COUNT(*) AS total_views, COUNT(DISTINCT cv.subscriber_id) AS unique_views
    FROM variant_campaigns vc
    JOIN campaign_views cv ON cv.campaign_id = vc.campaign_id
    GROUP BY vc.variant_id
),
variant_clicks AS (
    SELECT vc.variant_id, COUNT(*) AS total_clicks, COUNT(DISTINCT lc.subscriber_id) AS unique_clicks
    FROM variant_campaigns vc
    JOIN link_clicks lc ON lc.campaign_id = vc.campaign_id
    GROUP BY vc.variant_id
),
variant_bounces AS (
    SELECT vc.variant_id, COUNT(*) AS total_bounces
    FROM variant_campaigns vc
    JOIN bounces b ON b.campaign_id = vc.campaign_id
    GROUP BY vc.variant_id
)
SELECT
    vc.variant_id,
    vc.variant_label,
    vc.campaign_id,
    vc.split_pct,
    c.name AS campaign_name,
    c.subject AS campaign_subject,
    c.sent,
    COALESCE(v.total_views, 0) AS total_views,
    COALESCE(v.unique_views, 0) AS unique_views,
    COALESCE(cl.total_clicks, 0) AS total_clicks,
    COALESCE(cl.unique_clicks, 0) AS unique_clicks,
    COALESCE(b.total_bounces, 0) AS total_bounces,
    CASE WHEN c.sent > 0 THEN ROUND(COALESCE(v.unique_views, 0)::NUMERIC / c.sent * 100, 2) ELSE 0 END AS open_rate,
    CASE WHEN c.sent > 0 THEN ROUND(COALESCE(cl.unique_clicks, 0)::NUMERIC / c.sent * 100, 2) ELSE 0 END AS click_rate,
    CASE WHEN COALESCE(v.unique_views, 0) > 0 THEN ROUND(COALESCE(cl.unique_clicks, 0)::NUMERIC / v.unique_views * 100, 2) ELSE 0 END AS click_to_open_rate
FROM variant_campaigns vc
LEFT JOIN campaigns c ON c.id = vc.campaign_id
LEFT JOIN variant_views v ON v.variant_id = vc.variant_id
LEFT JOIN variant_clicks cl ON cl.variant_id = vc.variant_id
LEFT JOIN variant_bounces b ON b.variant_id = vc.variant_id
ORDER BY vc.variant_id;

-- name: delete-ab-test
DELETE FROM ab_tests WHERE id = $1 AND status = 'draft';
