package database

import (
	"context"
	"database/sql"
	"fmt"
)

// runMigrations runs all database migrations using raw SQL.
// All DDL is written in PostgreSQL syntax. For pglike (go-postgres/SQLite),
// the driver translates automatically. PostgreSQL-specific features (tsvector,
// triggers, plpgsql) are skipped when isPglike is true.
func runMigrations(ctx context.Context, db *sql.DB, isPglike bool) error {
	// Create migrations tracking table (version is the natural key)
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Check which migrations have been applied
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("failed to check applied migrations: %w", err)
	}
	defer rows.Close()

	appliedMap := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return err
		}
		appliedMap[version] = true
	}

	migrations := []struct {
		version string
		name    string
		up      func(context.Context, *sql.DB, bool) error
	}{
		{"001", "initial_schema", migrate001InitialSchema},
		{"002", "add_fulltext_search", migrate002AddFullTextSearch},
		{"003", "add_word_cloud", migrate003AddWordCloud},
		{"004", "create_jobs_table", migrate004CreateJobsTable},
		{"005", "add_tagging_system", migrate005AddTaggingSystem},
		{"006", "unify_tags_dimensions", migrate006UnifyTagsDimensions},
		{"007", "create_saved_searches", migrate007CreateSavedSearches},
		{"008", "add_tagged_saved_search", migrate008AddTaggedSavedSearch},
		{"009", "add_document_date", migrate009AddDocumentDate},
		{"010", "add_stories", migrate010AddStories},
		{"011", "add_document_metadata", migrate011AddDocumentMetadata},
		{"012", "add_hide_tag", migrate012AddHideTag},
	}

	for _, m := range migrations {
		if appliedMap[m.version] {
			continue
		}

		Logger.Info("Running migration", "version", m.version, "name", m.name)
		if err := m.up(ctx, db, isPglike); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", m.version, err)
		}

		_, err = db.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, m.version)
		if err != nil {
			return fmt.Errorf("failed to mark migration %s as applied: %w", m.version, err)
		}
	}

	Logger.Info("All migrations completed successfully")
	return nil
}

// Migration 001: Create initial schema (documents and server_config tables)
func migrate001InitialSchema(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 001: Create initial schema")

	// Create documents table - PostgreSQL syntax, go-postgres translates SERIAL for SQLite
	_, err := db.ExecContext(ctx, `
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
		)`)
	if err != nil {
		return fmt.Errorf("failed to create documents table: %w", err)
	}

	// Create indexes
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
	_, err = db.ExecContext(ctx, `
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
		)`)
	if err != nil {
		return fmt.Errorf("failed to create server_config table: %w", err)
	}

	// Insert default config row
	_, err = db.ExecContext(ctx,
		`INSERT INTO server_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("failed to insert default config: %w", err)
	}

	Logger.Info("Migration 001 completed successfully")
	return nil
}

// Migration 002: Add full-text search support
func migrate002AddFullTextSearch(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 002: Add full-text search")

	if !isPglike {
		// PostgreSQL: Add tsvector column, GIN index, and trigger
		_, err := db.ExecContext(ctx,
			`ALTER TABLE documents ADD COLUMN IF NOT EXISTS full_text_search tsvector`)
		if err != nil {
			Logger.Warn("Could not add full_text_search column (might already exist)", "error", err)
		}

		_, err = db.ExecContext(ctx,
			`CREATE INDEX IF NOT EXISTS idx_documents_full_text_search ON documents USING GIN(full_text_search)`)
		if err != nil {
			return fmt.Errorf("failed to create full_text_search GIN index: %w", err)
		}

		_, err = db.ExecContext(ctx, `
			CREATE OR REPLACE FUNCTION update_full_text_search()
			RETURNS TRIGGER AS $$
			BEGIN
				NEW.full_text_search = to_tsvector('english', COALESCE(NEW.full_text, '') || ' ' || COALESCE(NEW.name, ''));
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql`)
		if err != nil {
			return fmt.Errorf("failed to create update_full_text_search function: %w", err)
		}

		db.ExecContext(ctx, `DROP TRIGGER IF EXISTS trigger_update_full_text_search ON documents`)

		_, err = db.ExecContext(ctx, `
			CREATE TRIGGER trigger_update_full_text_search
				BEFORE INSERT OR UPDATE OF full_text, name ON documents
				FOR EACH ROW
				EXECUTE FUNCTION update_full_text_search()`)
		if err != nil {
			return fmt.Errorf("failed to create trigger: %w", err)
		}

		// Update existing documents
		_, err = db.ExecContext(ctx, `
			UPDATE documents
			SET full_text_search = to_tsvector('english', COALESCE(full_text, '') || ' ' || COALESCE(name, ''))`)
		if err != nil {
			Logger.Warn("Could not update existing documents (table might be empty)", "error", err)
		}
	} else {
		// pglike/SQLite: Add a plain column for compatibility (search uses LIKE)
		_, err := db.ExecContext(ctx,
			`ALTER TABLE documents ADD COLUMN full_text_search TEXT`)
		if err != nil {
			Logger.Warn("Could not add full_text_search column (might already exist)", "error", err)
		}
	}

	Logger.Info("Migration 002 completed successfully")
	return nil
}

// Migration 003: Add word cloud tables
func migrate003AddWordCloud(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 003: Add word cloud tables")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS word_frequencies (
			word TEXT PRIMARY KEY,
			frequency INTEGER DEFAULT 1,
			last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create word_frequencies table: %w", err)
	}

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_word_frequencies_frequency ON word_frequencies(frequency DESC)",
		"CREATE INDEX IF NOT EXISTS idx_word_frequencies_updated ON word_frequencies(last_updated DESC)",
	}
	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS word_cloud_metadata (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			last_full_calculation TIMESTAMP,
			total_documents_processed INTEGER DEFAULT 0,
			total_words_indexed INTEGER DEFAULT 0,
			version INTEGER DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create word_cloud_metadata table: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO word_cloud_metadata (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("failed to insert default metadata: %w", err)
	}

	Logger.Info("Migration 003 completed successfully")
	return nil
}

// Migration 004: Create jobs table
func migrate004CreateJobsTable(ctx context.Context, db *sql.DB, isPglike bool) error {
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
		)`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type)",
		"CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC)",
	}
	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			Logger.Warn("Could not create index", "error", err)
		}
	}

	// Partial index (PostgreSQL supports this, SQLite 3.8.0+ too)
	_, err = db.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_jobs_completed_at ON jobs(completed_at) WHERE completed_at IS NOT NULL`)
	if err != nil {
		Logger.Warn("Could not create partial index", "error", err)
	}

	Logger.Info("Migration 004 completed successfully")
	return nil
}

// Migration 005: Add tagging system with dimensions
func migrate005AddTaggingSystem(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 005: Add tagging system")

	// Create tags table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tags (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			color TEXT DEFAULT '#3498db',
			description TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create tags table: %w", err)
	}

	// Create document_tags junction table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS document_tags (
			document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (document_id, tag_id)
		)`)
	if err != nil {
		return fmt.Errorf("failed to create document_tags table: %w", err)
	}

	// Create dimensions table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS dimensions (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			description TEXT,
			dimension_type TEXT NOT NULL DEFAULT 'single',
			is_required BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create dimensions table: %w", err)
	}

	// Create dimension_values table
	_, err = db.ExecContext(ctx, `
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
		)`)
	if err != nil {
		return fmt.Errorf("failed to create dimension_values table: %w", err)
	}

	// Create document_dimensions table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS document_dimensions (
			id SERIAL PRIMARY KEY,
			document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			dimension_id INTEGER NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
			dimension_value_id INTEGER NOT NULL REFERENCES dimension_values(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(document_id, dimension_id)
		)`)
	if err != nil {
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
	_, err = db.ExecContext(ctx, `
		INSERT INTO dimensions (name, display_name, description, dimension_type, is_required) VALUES
			('person', 'Person', 'Document owner or related person', 'single', false),
			('location', 'Location', 'Physical or organizational location', 'single', false),
			('year', 'Year', 'Document year for archival purposes', 'single', false),
			('importance', 'Importance', 'Document importance level', 'single', false),
			('retention', 'Retention', 'How long to keep this document', 'single', false)
		ON CONFLICT (name) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("failed to insert dimensions: %w", err)
	}

	// Insert dimension values for each dimension
	dimValues := map[string][]struct {
		value, displayName, description, color string
		sortOrder                              int
	}{
		"person": {
			{"parent1", "Parent 1", "Documents belonging to parent 1", "#3498db", 1},
			{"parent2", "Parent 2", "Documents belonging to parent 2", "#e91e63", 2},
			{"child1", "Child 1", "Documents for first child", "#9c27b0", 3},
			{"child2", "Child 2", "Documents for second child", "#673ab7", 4},
			{"child3", "Child 3", "Documents for third child", "#2196f3", 5},
			{"family", "Family", "Family documents (all members)", "#4caf50", 6},
			{"business", "Business", "Business-related documents", "#ff9800", 7},
			{"other", "Other", "Other person-related documents", "#9e9e9e", 99},
		},
		"location": {
			{"home", "Home", "Home-related documents", "#4caf50", 1},
			{"office", "Office", "Office/workplace documents", "#2196f3", 2},
			{"bank", "Bank", "Banking and financial documents", "#ff9800", 3},
			{"medical", "Medical", "Medical and health documents", "#f44336", 4},
			{"legal", "Legal", "Legal documents", "#9c27b0", 5},
			{"insurance", "Insurance", "Insurance-related documents", "#00bcd4", 6},
			{"tax", "Tax", "Tax-related documents", "#795548", 7},
			{"education", "Education", "Education-related documents", "#607d8b", 8},
			{"other", "Other", "Other locations", "#9e9e9e", 99},
		},
		"importance": {
			{"low", "Low", "Low importance - can be discarded easily", "#9e9e9e", 1},
			{"medium", "Medium", "Medium importance - keep for reference", "#3498db", 2},
			{"high", "High", "High importance - important to keep", "#ff9800", 3},
			{"critical", "Critical", "Critical - must keep and protect", "#f44336", 4},
		},
		"retention": {
			{"temporary", "Temporary", "Delete after processing (< 1 year)", "#9e9e9e", 1},
			{"keep_1_year", "1 Year", "Keep for 1 year", "#03a9f4", 2},
			{"keep_3_years", "3 Years", "Keep for 3 years", "#2196f3", 3},
			{"keep_7_years", "7 Years", "Keep for 7 years (tax records)", "#ff9800", 4},
			{"keep_10_years", "10 Years", "Keep for 10 years", "#ff5722", 5},
			{"keep_permanent", "Permanent", "Keep permanently (birth certificates, etc.)", "#4caf50", 6},
		},
	}

	for dimName, values := range dimValues {
		var dimID int
		err := db.QueryRowContext(ctx,
			`SELECT id FROM dimensions WHERE name = $1`, dimName).Scan(&dimID)
		if err != nil {
			return fmt.Errorf("failed to get %s dimension ID: %w", dimName, err)
		}

		for _, v := range values {
			_, err := db.ExecContext(ctx, `
				INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (dimension_id, value) DO NOTHING`,
				dimID, v.value, v.displayName, v.description, v.color, v.sortOrder)
			if err != nil {
				return fmt.Errorf("failed to insert %s value %s: %w", dimName, v.value, err)
			}
		}
	}

	Logger.Info("Migration 005 completed successfully")
	return nil
}

// Migration 006: Unify Tags and Dimensions
func migrate006UnifyTagsDimensions(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 006: Unify tags and dimensions")

	// Add tag_group and sort_order columns to tags
	for _, col := range []string{
		`ALTER TABLE tags ADD COLUMN tag_group TEXT`,
		`ALTER TABLE tags ADD COLUMN sort_order INTEGER DEFAULT 0`,
	} {
		_, err := db.ExecContext(ctx, col)
		if err != nil {
			Logger.Warn("Could not add column (might already exist)", "sql", col, "error", err)
		}
	}

	_, err := db.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_tags_group ON tags(tag_group)`)
	if err != nil {
		Logger.Warn("Could not create idx_tags_group index", "error", err)
	}

	// Migrate dimension values to grouped tags
	// Use GROUP BY which works on both PG and SQLite (avoids DISTINCT ON)
	_, err = db.ExecContext(ctx, `
		INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
		SELECT
			dv.display_name as name,
			MIN(dv.color) as color,
			MIN(COALESCE(dv.description, '')) as description,
			MIN(d.display_name) as tag_group,
			MIN(dv.sort_order) as sort_order,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP
		FROM dimension_values dv
		JOIN dimensions d ON dv.dimension_id = d.id
		GROUP BY dv.display_name
		ON CONFLICT (name) DO NOTHING`)
	if err != nil {
		Logger.Warn("Could not migrate dimension values to tags", "error", err)
	}

	// Migrate document_dimensions to document_tags
	_, err = db.ExecContext(ctx, `
		INSERT INTO document_tags (document_id, tag_id, created_at)
		SELECT
			dd.document_id,
			t.id as tag_id,
			dd.created_at
		FROM document_dimensions dd
		JOIN dimension_values dv ON dd.dimension_value_id = dv.id
		JOIN dimensions d ON dd.dimension_id = d.id
		JOIN tags t ON t.name = dv.display_name AND t.tag_group = d.display_name
		ON CONFLICT (document_id, tag_id) DO NOTHING`)
	if err != nil {
		Logger.Warn("Could not migrate document dimensions to tags", "error", err)
	}

	// Create one-tag-per-group constraint (PostgreSQL only - uses plpgsql)
	if !isPglike {
		_, err = db.ExecContext(ctx, `
			CREATE OR REPLACE FUNCTION enforce_one_tag_per_group()
			RETURNS TRIGGER AS $$
			DECLARE
				v_tag_group TEXT;
			BEGIN
				SELECT tag_group INTO v_tag_group FROM tags WHERE id = NEW.tag_id;
				IF v_tag_group IS NOT NULL THEN
					DELETE FROM document_tags
					WHERE document_id = NEW.document_id
					AND tag_id IN (SELECT id FROM tags WHERE tag_group = v_tag_group)
					AND tag_id != NEW.tag_id;
				END IF;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql`)
		if err != nil {
			return fmt.Errorf("failed to create enforce_one_tag_per_group function: %w", err)
		}

		db.ExecContext(ctx, `DROP TRIGGER IF EXISTS enforce_one_tag_per_group_trigger ON document_tags`)

		_, err = db.ExecContext(ctx, `
			CREATE TRIGGER enforce_one_tag_per_group_trigger
				AFTER INSERT ON document_tags
				FOR EACH ROW
				EXECUTE FUNCTION enforce_one_tag_per_group()`)
		if err != nil {
			return fmt.Errorf("failed to create trigger: %w", err)
		}
	}

	Logger.Info("Migration 006 completed successfully")
	return nil
}

// Migration 007: Create Saved Searches table
func migrate007CreateSavedSearches(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 007: Create saved searches table")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS saved_searches (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT '',
			query TEXT NOT NULL,
			icon TEXT DEFAULT '',
			sort_order INTEGER DEFAULT 0,
			is_system BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create saved_searches table: %w", err)
	}

	// Insert default saved searches
	defaults := []struct {
		name, description, query, icon string
		sortOrder                      int
	}{
		{"Inbox", "Documents without any tags", "!untagged", "📥", 1},
		{"All Documents", "Browse all documents", "*", "📄", 2},
	}

	for _, s := range defaults {
		_, err := db.ExecContext(ctx, `
			INSERT INTO saved_searches (name, description, query, icon, sort_order, is_system)
			VALUES ($1, $2, $3, $4, $5, TRUE)
			ON CONFLICT (name) DO NOTHING`,
			s.name, s.description, s.query, s.icon, s.sortOrder)
		if err != nil {
			Logger.Warn("Could not insert saved search", "name", s.name, "error", err)
		}
	}

	Logger.Info("Migration 007 completed successfully")
	return nil
}

// Migration 008: Add Tagged saved search
func migrate008AddTaggedSavedSearch(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 008: Add Tagged saved search")

	_, err := db.ExecContext(ctx, `
		INSERT INTO saved_searches (name, description, query, icon, sort_order, is_system)
		VALUES ('Tagged', 'Documents with at least one tag', '!tagged', '🏷️', 2, TRUE)
		ON CONFLICT (name) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("failed to insert Tagged saved search: %w", err)
	}

	Logger.Info("Migration 008 completed successfully")
	return nil
}

// Migration 009: Add document_date column
func migrate009AddDocumentDate(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 009: Add document_date column")

	_, err := db.ExecContext(ctx,
		`ALTER TABLE documents ADD COLUMN document_date TIMESTAMP`)
	if err != nil {
		Logger.Warn("Could not add document_date column (might already exist)", "error", err)
	}

	_, err = db.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_documents_document_date ON documents(document_date)`)
	if err != nil {
		return fmt.Errorf("failed to create document_date index: %w", err)
	}

	Logger.Info("Migration 009 completed successfully")
	return nil
}

// Migration 010: Add stories and story_tags tables
func migrate010AddStories(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 010: Add stories tables")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS stories (
			id SERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			start_date DATE,
			end_date DATE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(tag_id)
		)`)
	if err != nil {
		return fmt.Errorf("failed to create stories table: %w", err)
	}

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_stories_tag_id ON stories(tag_id)",
		"CREATE INDEX IF NOT EXISTS idx_stories_start_date ON stories(start_date)",
	}
	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS story_tags (
			story_id INTEGER NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (story_id, tag_id)
		)`)
	if err != nil {
		return fmt.Errorf("failed to create story_tags table: %w", err)
	}

	Logger.Info("Migration 010 completed successfully")
	return nil
}

// Migration 011: Add document metadata columns for remote import (e.g. Evernote)
func migrate011AddDocumentMetadata(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 011: Add document metadata columns")

	columns := []string{
		`ALTER TABLE documents ADD COLUMN created_date TIMESTAMP`,
		`ALTER TABLE documents ADD COLUMN updated_date TIMESTAMP`,
		`ALTER TABLE documents ADD COLUMN author TEXT DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN source_url TEXT DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN source TEXT DEFAULT ''`,
	}
	for _, col := range columns {
		_, err := db.ExecContext(ctx, col)
		if err != nil {
			Logger.Warn("Could not add column (might already exist)", "sql", col, "error", err)
		}
	}

	Logger.Info("Migration 011 completed successfully")
	return nil
}

// Migration 012: Add "Hide" system tag
func migrate012AddHideTag(ctx context.Context, db *sql.DB, isPglike bool) error {
	Logger.Info("Running migration 012: Add Hide system tag")

	_, err := db.ExecContext(ctx, `
		INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
		VALUES ('Hide', '#6c757d', 'Hidden documents — excluded from default views', 'System', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (name) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("failed to insert Hide tag: %w", err)
	}

	Logger.Info("Migration 012 completed successfully")
	return nil
}
