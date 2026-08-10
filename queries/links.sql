-- links
-- name: create-link
INSERT INTO links (uuid, url, metadata) VALUES($1, $2, $3)
    ON CONFLICT (url) DO UPDATE SET metadata = EXCLUDED.metadata
    RETURNING uuid;

-- name: register-link-click
WITH link AS(
    SELECT id, url, metadata FROM links WHERE uuid = $1
)
INSERT INTO link_clicks (campaign_id, subscriber_id, link_id, metadata) VALUES(
    (SELECT id FROM campaigns WHERE uuid = $2),
    (SELECT id FROM subscribers WHERE
        (CASE WHEN $3::TEXT != '' THEN subscribers.uuid = $3::UUID ELSE FALSE END)
    ),
    (SELECT id FROM link),
    (SELECT metadata FROM link)
) RETURNING (SELECT url FROM link);
