package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// runMigrations runs all Bun migrations
func runMigrations(ctx context.Context, db *bun.DB) error {
	// Create a simple migrations tracking table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS bun_schema_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version TEXT NOT NULL UNIQUE,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Check which migrations have been applied
	type AppliedMigration struct {
		bun.BaseModel `bun:"table:bun_schema_migrations"`
		Version       string `bun:"version"`
	}
	var applied []AppliedMigration
	err = db.NewSelect().
		Model(&applied).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to check applied migrations: %w", err)
	}

	appliedMap := make(map[string]bool)
	for _, m := range applied {
		appliedMap[m.Version] = true
	}

	// Run migrations in order
	migrations := []struct {
		version string
		name    string
		up      func(context.Context, *bun.DB) error
	}{
		{"001", "initial_schema", init001CreateDocumentsTable},
		{"002", "add_fulltext_search", init002AddFullTextSearch},
		{"003", "add_word_cloud", init003AddWordCloud},
		{"004", "create_jobs_table", init004CreateJobsTable},
		{"005", "add_tagging_system", init005AddTaggingSystem},
	}

	for _, m := range migrations {
		if appliedMap[m.version] {
			continue
		}

		Logger.Info("Running migration", "version", m.version, "name", m.name)
		if err := m.up(ctx, db); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", m.version, err)
		}

		// Mark as applied
		_, err = db.NewInsert().
			Model(&AppliedMigration{Version: m.version}).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark migration %s as applied: %w", m.version, err)
		}
	}

	Logger.Info("All migrations completed successfully")
	return nil
}

// Migration 001: Create initial schema (documents and server_config tables)
func init001CreateDocumentsTable(ctx context.Context, db *bun.DB) error {
	Logger.Info("Running migration 001: Create initial schema")

	// Detect database dialect - check if it's PostgreSQL by checking dialect features
	_, isPostgres := db.Dialect().(interface{ SupportsReturning() bool })

	// Create documents table
	var createTableSQL string
	if isPostgres {
		createTableSQL = `
			CREATE TABLE IF NOT EXISTS documents (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				path TEXT NOT NULL UNIQUE,
				ingress_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				folder TEXT NOT NULL,
				hash TEXT NOT NULL,
				ulid TEXT NOT NULL UNIQUE,
				document_type TEXT NOT NULL,
				full_text TEXT,
				url TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
	} else {
		createTableSQL = `
			CREATE TABLE IF NOT EXISTS documents (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				path TEXT NOT NULL UNIQUE,
				ingress_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				folder TEXT NOT NULL,
				hash TEXT NOT NULL,
				ulid TEXT NOT NULL UNIQUE,
				document_type TEXT NOT NULL,
				full_text TEXT,
				url TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
	}

	_, err := db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create documents table: %w", err)
	}

	// Create indexes for documents
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(hash)",
		"CREATE INDEX IF NOT EXISTS idx_documents_ulid ON documents(ulid)",
		"CREATE INDEX IF NOT EXISTS idx_documents_folder ON documents(folder)",
		"CREATE INDEX IF NOT EXISTS idx_documents_ingress_time ON documents(ingress_time DESC)",
	}

	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Create server_config table
	var createConfigSQL string
	var insertConfigSQL string
	if isPostgres {
		createConfigSQL = `
			CREATE TABLE IF NOT EXISTS server_config (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				listen_addr_ip TEXT DEFAULT '',
				listen_addr_port TEXT NOT NULL DEFAULT '8000',
				ingress_path TEXT NOT NULL DEFAULT '',
				ingress_delete BOOLEAN NOT NULL DEFAULT false,
				ingress_move_folder TEXT NOT NULL DEFAULT '',
				ingress_preserve BOOLEAN NOT NULL DEFAULT true,
				document_path TEXT NOT NULL DEFAULT '',
				new_document_folder TEXT DEFAULT '',
				new_document_folder_rel TEXT DEFAULT '',
				web_ui_pass BOOLEAN NOT NULL DEFAULT false,
				client_username TEXT DEFAULT '',
				client_password TEXT DEFAULT '',
				pushbullet_token TEXT DEFAULT '',
				tesseract_path TEXT DEFAULT '',
				use_reverse_proxy BOOLEAN NOT NULL DEFAULT false,
				base_url TEXT DEFAULT '',
				ingress_interval INTEGER NOT NULL DEFAULT 10,
				new_document_number INTEGER NOT NULL DEFAULT 5,
				server_api_url TEXT DEFAULT '',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
		insertConfigSQL = `INSERT INTO server_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING`
	} else {
		createConfigSQL = `
			CREATE TABLE IF NOT EXISTS server_config (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				listen_addr_ip TEXT DEFAULT '',
				listen_addr_port TEXT NOT NULL DEFAULT '8000',
				ingress_path TEXT NOT NULL DEFAULT '',
				ingress_delete BOOLEAN NOT NULL DEFAULT 0,
				ingress_move_folder TEXT NOT NULL DEFAULT '',
				ingress_preserve BOOLEAN NOT NULL DEFAULT 1,
				document_path TEXT NOT NULL DEFAULT '',
				new_document_folder TEXT DEFAULT '',
				new_document_folder_rel TEXT DEFAULT '',
				web_ui_pass BOOLEAN NOT NULL DEFAULT 0,
				client_username TEXT DEFAULT '',
				client_password TEXT DEFAULT '',
				pushbullet_token TEXT DEFAULT '',
				tesseract_path TEXT DEFAULT '',
				use_reverse_proxy BOOLEAN NOT NULL DEFAULT 0,
				base_url TEXT DEFAULT '',
				ingress_interval INTEGER NOT NULL DEFAULT 10,
				new_document_number INTEGER NOT NULL DEFAULT 5,
				server_api_url TEXT DEFAULT '',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
		insertConfigSQL = `INSERT OR IGNORE INTO server_config (id) VALUES (1)`
	}

	_, err = db.ExecContext(ctx, createConfigSQL)
	if err != nil {
		return fmt.Errorf("failed to create server_config table: %w", err)
	}

	// Insert default config row
	_, err = db.ExecContext(ctx, insertConfigSQL)
	if err != nil {
		return fmt.Errorf("failed to insert default config: %w", err)
	}

	Logger.Info("Migration 001 completed successfully")
	return nil
}

func init001RollbackDocumentsTable(ctx context.Context, db *bun.DB) error {
	Logger.Info("Rolling back migration 001")

	_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS server_config")
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "DROP TABLE IF EXISTS documents")
	return err
}

// Migration 002: Add full-text search support
func init002AddFullTextSearch(ctx context.Context, db *bun.DB) error {
	Logger.Info("Running migration 002: Add full-text search")

	// Detect database dialect
	_, isPostgres := db.Dialect().(interface{ SupportsReturning() bool })

	if isPostgres {
		// PostgreSQL: Add tsvector column and GIN index
		_, err := db.ExecContext(ctx, `
			ALTER TABLE documents ADD COLUMN IF NOT EXISTS full_text_search tsvector
		`)
		if err != nil {
			Logger.Warn("Could not add full_text_search column (might already exist)", "error", err)
		}

		// Create GIN index for fast full-text searching
		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_documents_full_text_search ON documents USING GIN(full_text_search)
		`)
		if err != nil {
			return fmt.Errorf("failed to create full_text_search GIN index: %w", err)
		}

		// Create function to update search vector
		_, err = db.ExecContext(ctx, `
			CREATE OR REPLACE FUNCTION update_full_text_search()
			RETURNS TRIGGER AS $$
			BEGIN
				NEW.full_text_search = to_tsvector('english', COALESCE(NEW.full_text, '') || ' ' || COALESCE(NEW.name, ''));
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql
		`)
		if err != nil {
			return fmt.Errorf("failed to create update_full_text_search function: %w", err)
		}

		// Create trigger to update search vector on insert/update
		_, err = db.ExecContext(ctx, `
			DROP TRIGGER IF EXISTS trigger_update_full_text_search ON documents
		`)
		if err != nil {
			Logger.Warn("Could not drop trigger (might not exist)", "error", err)
		}

		_, err = db.ExecContext(ctx, `
			CREATE TRIGGER trigger_update_full_text_search
				BEFORE INSERT OR UPDATE OF full_text, name ON documents
				FOR EACH ROW
				EXECUTE FUNCTION update_full_text_search()
		`)
		if err != nil {
			return fmt.Errorf("failed to create trigger: %w", err)
		}

		// Update existing documents to populate the search vector
		_, err = db.ExecContext(ctx, `
			UPDATE documents
			SET full_text_search = to_tsvector('english', COALESCE(full_text, '') || ' ' || COALESCE(name, ''))
		`)
		if err != nil {
			Logger.Warn("Could not update existing documents (table might be empty)", "error", err)
		}
	} else {
		// SQLite: Add a simple full_text_search column for LIKE queries
		_, err := db.ExecContext(ctx, `
			ALTER TABLE documents ADD COLUMN full_text_search TEXT
		`)
		if err != nil {
			// Column might already exist, ignore error
			Logger.Warn("Could not add full_text_search column (might already exist)", "error", err)
		}

		// Create index for faster LIKE queries
		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_documents_full_text_search ON documents(full_text_search)
		`)
		if err != nil {
			return fmt.Errorf("failed to create full_text_search index: %w", err)
		}
	}

	Logger.Info("Migration 002 completed successfully")
	return nil
}

func init002RollbackFullTextSearch(ctx context.Context, db *bun.DB) error {
	Logger.Info("Rolling back migration 002")

	// SQLite doesn't support DROP COLUMN easily, so we skip it
	// The column will remain but won't be used

	Logger.Info("Migration 002 rollback completed (column retained for SQLite compatibility)")
	return nil
}

// Migration 003: Add word cloud tables
func init003AddWordCloud(ctx context.Context, db *bun.DB) error {
	Logger.Info("Running migration 003: Add word cloud tables")

	// Detect database dialect
	_, isPostgres := db.Dialect().(interface{ SupportsReturning() bool })

	// Create word_frequencies table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS word_frequencies (
			word TEXT PRIMARY KEY,
			frequency INTEGER DEFAULT 1,
			last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create word_frequencies table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_word_frequencies_frequency ON word_frequencies(frequency DESC)",
		"CREATE INDEX IF NOT EXISTS idx_word_frequencies_updated ON word_frequencies(last_updated DESC)",
	}

	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Create word_cloud_metadata table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS word_cloud_metadata (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			last_full_calculation TIMESTAMP,
			total_documents_processed INTEGER DEFAULT 0,
			total_words_indexed INTEGER DEFAULT 0,
			version INTEGER DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create word_cloud_metadata table: %w", err)
	}

	// Insert default metadata row
	var insertMetadataSQL string
	if isPostgres {
		insertMetadataSQL = `INSERT INTO word_cloud_metadata (id) VALUES (1) ON CONFLICT (id) DO NOTHING`
	} else {
		insertMetadataSQL = `INSERT OR IGNORE INTO word_cloud_metadata (id) VALUES (1)`
	}

	_, err = db.ExecContext(ctx, insertMetadataSQL)
	if err != nil {
		return fmt.Errorf("failed to insert default metadata: %w", err)
	}

	Logger.Info("Migration 003 completed successfully")
	return nil
}

func init003RollbackWordCloud(ctx context.Context, db *bun.DB) error {
	Logger.Info("Rolling back migration 003")

	_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS word_cloud_metadata")
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "DROP TABLE IF EXISTS word_frequencies")
	return err
}

// Migration 004: Create jobs table
func init004CreateJobsTable(ctx context.Context, db *bun.DB) error {
	Logger.Info("Running migration 004: Create jobs table")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			progress INTEGER DEFAULT 0,
			current_step TEXT DEFAULT '',
			total_steps INTEGER DEFAULT 0,
			message TEXT DEFAULT '',
			error TEXT,
			result TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at TIMESTAMP,
			completed_at TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_completed_at ON jobs(completed_at) WHERE completed_at IS NOT NULL",
	}

	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			// Partial indexes might not be supported in all SQLite versions
			Logger.Warn("Could not create index (might not be supported)", "error", err)
		}
	}

	Logger.Info("Migration 004 completed successfully")
	return nil
}

func init004RollbackJobsTable(ctx context.Context, db *bun.DB) error {
	Logger.Info("Rolling back migration 004")

	_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS jobs")
	return err
}

// Migration 005: Add tagging system with dimensions
func init005AddTaggingSystem(ctx context.Context, db *bun.DB) error {
	Logger.Info("Running migration 005: Add tagging system")

	// Detect database dialect
	_, isPostgres := db.Dialect().(interface{ SupportsReturning() bool })

	// Create tags table
	var tagsTableSQL string
	if isPostgres {
		tagsTableSQL = `
			CREATE TABLE IF NOT EXISTS tags (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				color TEXT DEFAULT '#3498db',
				description TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
	} else {
		tagsTableSQL = `
			CREATE TABLE IF NOT EXISTS tags (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				color TEXT DEFAULT '#3498db',
				description TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
	}

	if _, err := db.ExecContext(ctx, tagsTableSQL); err != nil {
		return fmt.Errorf("failed to create tags table: %w", err)
	}

	// Create document_tags junction table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS document_tags (
			document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (document_id, tag_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create document_tags table: %w", err)
	}

	// Create dimensions table
	var dimensionsTableSQL string
	if isPostgres {
		dimensionsTableSQL = `
			CREATE TABLE IF NOT EXISTS dimensions (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				display_name TEXT NOT NULL,
				description TEXT,
				dimension_type TEXT NOT NULL DEFAULT 'single',
				is_required BOOLEAN NOT NULL DEFAULT false,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
	} else {
		dimensionsTableSQL = `
			CREATE TABLE IF NOT EXISTS dimensions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				display_name TEXT NOT NULL,
				description TEXT,
				dimension_type TEXT NOT NULL DEFAULT 'single',
				is_required BOOLEAN NOT NULL DEFAULT 0,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
	}

	if _, err := db.ExecContext(ctx, dimensionsTableSQL); err != nil {
		return fmt.Errorf("failed to create dimensions table: %w", err)
	}

	// Create dimension_values table
	var dimensionValuesTableSQL string
	if isPostgres {
		dimensionValuesTableSQL = `
			CREATE TABLE IF NOT EXISTS dimension_values (
				id SERIAL PRIMARY KEY,
				dimension_id INTEGER NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
				value TEXT NOT NULL,
				display_name TEXT NOT NULL,
				description TEXT,
				color TEXT DEFAULT '#95a5a6',
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(dimension_id, value)
			)
		`
	} else {
		dimensionValuesTableSQL = `
			CREATE TABLE IF NOT EXISTS dimension_values (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				dimension_id INTEGER NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
				value TEXT NOT NULL,
				display_name TEXT NOT NULL,
				description TEXT,
				color TEXT DEFAULT '#95a5a6',
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(dimension_id, value)
			)
		`
	}

	if _, err := db.ExecContext(ctx, dimensionValuesTableSQL); err != nil {
		return fmt.Errorf("failed to create dimension_values table: %w", err)
	}

	// Create document_dimensions table
	var documentDimensionsTableSQL string
	if isPostgres {
		documentDimensionsTableSQL = `
			CREATE TABLE IF NOT EXISTS document_dimensions (
				id SERIAL PRIMARY KEY,
				document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
				dimension_id INTEGER NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
				dimension_value_id INTEGER NOT NULL REFERENCES dimension_values(id) ON DELETE CASCADE,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(document_id, dimension_id)
			)
		`
	} else {
		documentDimensionsTableSQL = `
			CREATE TABLE IF NOT EXISTS document_dimensions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
				dimension_id INTEGER NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
				dimension_value_id INTEGER NOT NULL REFERENCES dimension_values(id) ON DELETE CASCADE,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(document_id, dimension_id)
			)
		`
	}

	if _, err := db.ExecContext(ctx, documentDimensionsTableSQL); err != nil {
		return fmt.Errorf("failed to create document_dimensions table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_document_tags_document ON document_tags(document_id)",
		"CREATE INDEX IF NOT EXISTS idx_document_tags_tag ON document_tags(tag_id)",
		"CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name)",
		"CREATE INDEX IF NOT EXISTS idx_document_dimensions_document ON document_dimensions(document_id)",
		"CREATE INDEX IF NOT EXISTS idx_document_dimensions_dimension ON document_dimensions(dimension_id)",
		"CREATE INDEX IF NOT EXISTS idx_document_dimensions_value ON document_dimensions(dimension_value_id)",
		"CREATE INDEX IF NOT EXISTS idx_dimension_values_dimension ON dimension_values(dimension_id)",
	}

	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Insert predefined dimensions
	var insertDimensionsSQL string
	if isPostgres {
		insertDimensionsSQL = `
			INSERT INTO dimensions (name, display_name, description, dimension_type, is_required) VALUES
				('person', 'Person', 'Document owner or related person', 'single', false),
				('location', 'Location', 'Physical or organizational location', 'single', false),
				('year', 'Year', 'Document year for archival purposes', 'single', false),
				('importance', 'Importance', 'Document importance level', 'single', false),
				('retention', 'Retention', 'How long to keep this document', 'single', false)
			ON CONFLICT (name) DO NOTHING
		`
	} else {
		insertDimensionsSQL = `
			INSERT OR IGNORE INTO dimensions (name, display_name, description, dimension_type, is_required) VALUES
				('person', 'Person', 'Document owner or related person', 'single', 0),
				('location', 'Location', 'Physical or organizational location', 'single', 0),
				('year', 'Year', 'Document year for archival purposes', 'single', 0),
				('importance', 'Importance', 'Document importance level', 'single', 0),
				('retention', 'Retention', 'How long to keep this document', 'single', 0)
		`
	}

	if _, err := db.ExecContext(ctx, insertDimensionsSQL); err != nil {
		return fmt.Errorf("failed to insert dimensions: %w", err)
	}

	// Insert Person dimension values
	personValues := []struct {
		value       string
		displayName string
		description string
		color       string
		sortOrder   int
	}{
		{"husband", "Husband", "Documents belonging to husband", "#3498db", 1},
		{"wife", "Wife", "Documents belonging to wife", "#e91e63", 2},
		{"child1", "Child 1", "Documents for first child", "#9c27b0", 3},
		{"child2", "Child 2", "Documents for second child", "#673ab7", 4},
		{"child3", "Child 3", "Documents for third child", "#2196f3", 5},
		{"family", "Family", "Family documents (all members)", "#4caf50", 6},
		{"business", "Business", "Business-related documents", "#ff9800", 7},
		{"other", "Other", "Other person-related documents", "#9e9e9e", 99},
	}

	for _, v := range personValues {
		var insertSQL string
		if isPostgres {
			insertSQL = `
				INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, $1, $2, $3, $4, $5 FROM dimensions WHERE name = 'person'
				ON CONFLICT (dimension_id, value) DO NOTHING
			`
		} else {
			insertSQL = `
				INSERT OR IGNORE INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, ?, ?, ?, ?, ? FROM dimensions WHERE name = 'person'
			`
		}
		if _, err := db.ExecContext(ctx, insertSQL, v.value, v.displayName, v.description, v.color, v.sortOrder); err != nil {
			return fmt.Errorf("failed to insert person value %s: %w", v.value, err)
		}
	}

	// Insert Location dimension values
	locationValues := []struct {
		value       string
		displayName string
		description string
		color       string
		sortOrder   int
	}{
		{"home", "Home", "Home-related documents", "#4caf50", 1},
		{"office", "Office", "Office/workplace documents", "#2196f3", 2},
		{"bank", "Bank", "Banking and financial documents", "#ff9800", 3},
		{"medical", "Medical", "Medical and health documents", "#f44336", 4},
		{"legal", "Legal", "Legal documents", "#9c27b0", 5},
		{"insurance", "Insurance", "Insurance-related documents", "#00bcd4", 6},
		{"tax", "Tax", "Tax-related documents", "#795548", 7},
		{"education", "Education", "Education-related documents", "#607d8b", 8},
		{"other", "Other", "Other locations", "#9e9e9e", 99},
	}

	for _, v := range locationValues {
		var insertSQL string
		if isPostgres {
			insertSQL = `
				INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, $1, $2, $3, $4, $5 FROM dimensions WHERE name = 'location'
				ON CONFLICT (dimension_id, value) DO NOTHING
			`
		} else {
			insertSQL = `
				INSERT OR IGNORE INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, ?, ?, ?, ?, ? FROM dimensions WHERE name = 'location'
			`
		}
		if _, err := db.ExecContext(ctx, insertSQL, v.value, v.displayName, v.description, v.color, v.sortOrder); err != nil {
			return fmt.Errorf("failed to insert location value %s: %w", v.value, err)
		}
	}

	// Insert Importance dimension values
	importanceValues := []struct {
		value       string
		displayName string
		description string
		color       string
		sortOrder   int
	}{
		{"low", "Low", "Low importance - can be discarded easily", "#9e9e9e", 1},
		{"medium", "Medium", "Medium importance - keep for reference", "#3498db", 2},
		{"high", "High", "High importance - important to keep", "#ff9800", 3},
		{"critical", "Critical", "Critical - must keep and protect", "#f44336", 4},
	}

	for _, v := range importanceValues {
		var insertSQL string
		if isPostgres {
			insertSQL = `
				INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, $1, $2, $3, $4, $5 FROM dimensions WHERE name = 'importance'
				ON CONFLICT (dimension_id, value) DO NOTHING
			`
		} else {
			insertSQL = `
				INSERT OR IGNORE INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, ?, ?, ?, ?, ? FROM dimensions WHERE name = 'importance'
			`
		}
		if _, err := db.ExecContext(ctx, insertSQL, v.value, v.displayName, v.description, v.color, v.sortOrder); err != nil {
			return fmt.Errorf("failed to insert importance value %s: %w", v.value, err)
		}
	}

	// Insert Retention dimension values
	retentionValues := []struct {
		value       string
		displayName string
		description string
		color       string
		sortOrder   int
	}{
		{"temporary", "Temporary", "Delete after processing (< 1 year)", "#9e9e9e", 1},
		{"keep_1_year", "1 Year", "Keep for 1 year", "#03a9f4", 2},
		{"keep_3_years", "3 Years", "Keep for 3 years", "#2196f3", 3},
		{"keep_7_years", "7 Years", "Keep for 7 years (tax records)", "#ff9800", 4},
		{"keep_10_years", "10 Years", "Keep for 10 years", "#ff5722", 5},
		{"keep_permanent", "Permanent", "Keep permanently (birth certificates, etc.)", "#4caf50", 6},
	}

	for _, v := range retentionValues {
		var insertSQL string
		if isPostgres {
			insertSQL = `
				INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, $1, $2, $3, $4, $5 FROM dimensions WHERE name = 'retention'
				ON CONFLICT (dimension_id, value) DO NOTHING
			`
		} else {
			insertSQL = `
				INSERT OR IGNORE INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				SELECT id, ?, ?, ?, ?, ? FROM dimensions WHERE name = 'retention'
			`
		}
		if _, err := db.ExecContext(ctx, insertSQL, v.value, v.displayName, v.description, v.color, v.sortOrder); err != nil {
			return fmt.Errorf("failed to insert retention value %s: %w", v.value, err)
		}
	}

	Logger.Info("Migration 005 completed successfully")
	return nil
}

func init005RollbackTaggingSystem(ctx context.Context, db *bun.DB) error {
	Logger.Info("Rolling back migration 005")

	tables := []string{
		"document_dimensions",
		"document_tags",
		"dimension_values",
		"dimensions",
		"tags",
	}

	for _, table := range tables {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	Logger.Info("Migration 005 rollback completed")
	return nil
}
