-- conversions

-- name: insert-conversion
INSERT INTO conversions (campaign_id, subscriber_id, event_type, event_properties, revenue)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: get-campaign-conversions
-- Get conversion events for a campaign with subscriber info.
SELECT c.id, c.campaign_id, c.subscriber_id, c.event_type, c.event_properties, c.revenue, c.created_at,
       COALESCE(s.email, '') AS subscriber_email
FROM conversions c
LEFT JOIN subscribers s ON c.subscriber_id = s.id
WHERE c.campaign_id = $1
ORDER BY c.created_at DESC
LIMIT 1000;

-- name: get-subscriber-conversions
-- Get conversion events for a subscriber with campaign info.
SELECT c.id, c.campaign_id, c.subscriber_id, c.event_type, c.event_properties, c.revenue, c.created_at,
       COALESCE(camp.name, '') AS campaign_name
FROM conversions c
LEFT JOIN campaigns camp ON c.campaign_id = camp.id
WHERE c.subscriber_id = $1
ORDER BY c.created_at DESC
LIMIT 1000;

-- name: get-campaign-conversion-summary
-- Aggregate conversion stats by event type for a campaign.
SELECT event_type, COUNT(*) AS "count", COALESCE(SUM(revenue), 0) AS total_revenue
FROM conversions
WHERE campaign_id = $1
GROUP BY event_type
ORDER BY "count" DESC;

-- name: update-subscriber-posthog-attribs
-- Merge PostHog behavioral data into subscriber attribs without overwriting other keys.
UPDATE subscribers
SET attribs = jsonb_set(
    COALESCE(attribs, '{}'::jsonb),
    '{posthog}',
    CASE
        WHEN COALESCE(attribs, '{}'::jsonb) ? 'posthog'
            THEN (COALESCE(attribs, '{}'::jsonb)->'posthog') || $2::jsonb
        ELSE $2::jsonb
    END
),
updated_at = NOW()
WHERE id = $1;
