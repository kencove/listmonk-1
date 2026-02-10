package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V9_0_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	// Conversions table for PostHog post-click behavior tracking.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS conversions (
			id BIGSERIAL PRIMARY KEY,
			campaign_id INTEGER REFERENCES campaigns(id) ON DELETE SET NULL,
			subscriber_id INTEGER NOT NULL REFERENCES subscribers(id) ON DELETE CASCADE,
			event_type TEXT NOT NULL,
			event_properties JSONB NOT NULL DEFAULT '{}',
			revenue NUMERIC(12,2) DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_conversions_campaign ON conversions(campaign_id);
		CREATE INDEX IF NOT EXISTS idx_conversions_subscriber ON conversions(subscriber_id);
		CREATE INDEX IF NOT EXISTS idx_conversions_event_type ON conversions(event_type);
		CREATE INDEX IF NOT EXISTS idx_conversions_created ON conversions(created_at);
		CREATE INDEX IF NOT EXISTS idx_conversions_campaign_event ON conversions(campaign_id, event_type);
		CREATE INDEX IF NOT EXISTS idx_conversions_subscriber_created ON conversions(subscriber_id, created_at DESC);

		-- Add PostHog webhook secret to settings.
		UPDATE settings SET value = value || '{"posthog.webhook_secret": ""}'::jsonb WHERE key = 'app';
	`)
	return err
}
