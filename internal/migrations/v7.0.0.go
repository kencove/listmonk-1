package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V7_0_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	// Add metadata JSONB column to links table for per-link attribution tagging.
	_, err := db.Exec(`ALTER TABLE links ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'`)
	if err != nil {
		return err
	}

	// Add metadata JSONB column to link_clicks to snapshot the metadata at click time.
	_, err = db.Exec(`ALTER TABLE link_clicks ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'`)
	if err != nil {
		return err
	}

	// Create index on link metadata for querying by tag values.
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_links_metadata ON links USING GIN (metadata)`)
	if err != nil {
		return err
	}

	// Create index on link_clicks metadata for analytics queries.
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clicks_metadata ON link_clicks USING GIN (metadata)`)
	if err != nil {
		return err
	}

	return nil
}
