package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V8_0_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	// A/B testing tables.
	_, err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ab_test_status') THEN
				CREATE TYPE ab_test_status AS ENUM ('draft', 'running', 'selecting_winner', 'finished', 'cancelled');
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS ab_tests (
			id               SERIAL PRIMARY KEY,
			uuid             uuid NOT NULL UNIQUE,
			name             TEXT NOT NULL,
			status           ab_test_status NOT NULL DEFAULT 'draft',
			-- Percentage of total subscribers used for the test phase (rest are holdout).
			test_pct         INT NOT NULL DEFAULT 30,
			-- Metric to evaluate winner: 'opens' or 'clicks'.
			winner_metric    TEXT NOT NULL DEFAULT 'clicks',
			-- How long to wait before evaluating the winner.
			test_duration    INTERVAL NOT NULL DEFAULT '4 hours',
			-- The winning variant's campaign ID, set after evaluation.
			winner_variant_id INT NULL,
			-- Total subscribers available across all lists.
			total_subscribers INT NOT NULL DEFAULT 0,
			created_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_ab_tests_status ON ab_tests(status);

		CREATE TABLE IF NOT EXISTS ab_test_variants (
			id               SERIAL PRIMARY KEY,
			ab_test_id       INT NOT NULL REFERENCES ab_tests(id) ON DELETE CASCADE,
			campaign_id      INT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
			-- The temporary list created to hold this variant's subscriber segment.
			list_id          INT NULL REFERENCES lists(id) ON DELETE SET NULL,
			variant_label    TEXT NOT NULL DEFAULT 'A',
			-- Percentage split within the test population (e.g. 50/50 for 2 variants).
			split_pct        INT NOT NULL DEFAULT 50,
			UNIQUE(ab_test_id, campaign_id)
		);
		CREATE INDEX IF NOT EXISTS idx_ab_variants_test_id ON ab_test_variants(ab_test_id);
		CREATE INDEX IF NOT EXISTS idx_ab_variants_campaign_id ON ab_test_variants(campaign_id);
	`)
	return err
}
