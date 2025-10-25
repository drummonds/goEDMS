package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	config "github.com/drummonds/goEDMS/config"
	"github.com/oklog/ulid/v2"
)

// PostgresDB implements DBInterface for PostgreSQL
type PostgresDB struct {
	db         *sql.DB
	isEmbedded bool // Now refers to ephemeral instances
}

// SetupPostgresDatabase initializes PostgreSQL database with migrations
// If connectionString is empty, it will use ephemeral PostgreSQL
func SetupPostgresDatabase(connectionString string) (*PostgresDB, error) {
	var db *sql.DB
	var isEmbedded bool
	var err error

	if connectionString == "" {
		// Use ephemeral PostgreSQL for development
		Logger.Info("No connection string provided, using ephemeral PostgreSQL...")

		ephemeralDB, err := SetupEphemeralPostgresDatabase()
		if err != nil {
			return nil, fmt.Errorf("failed to setup ephemeral postgres: %w", err)
		}

		// Return the PostgresDB part, the ephemeral wrapper will handle cleanup
		return ephemeralDB.PostgresDB, nil
	} else {
		isEmbedded = false
		Logger.Info("Connecting to external PostgreSQL/CockroachDB server...")
	}

	// Open PostgreSQL database
	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	Logger.Info("Connected to PostgreSQL database successfully")

	// Run migrations
	if err := runPostgresMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &PostgresDB{
		db:         db,
		isEmbedded: isEmbedded,
	}, nil
}

func runPostgresMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Try to find the migrations directory
	// First try from project root
	migrationsPath, err := filepath.Abs("database/migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	// If running from within the database directory (during tests), adjust path
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		migrationsPath, err = filepath.Abs("migrations")
		if err != nil {
			return fmt.Errorf("failed to get migrations path: %w", err)
		}
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Check current version and apply migrations
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if dirty {
		// Try to force clean and retry
		Logger.Warn("Database is in dirty state, attempting to recover")
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}
	}

	// Apply latest migrations
	Logger.Info("Applying database migrations")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	Logger.Info("Database migrations completed successfully")
	return nil
}

// Close closes the database connection and stops embedded server if running
func (p *PostgresDB) Close() error {
	if p.db != nil {
		if err := p.db.Close(); err != nil {
			return err
		}
	}

	// Note: Ephemeral PostgreSQL cleanup is handled by EphemeralPostgresDB.Close()
	// This method only closes the database connection

	return nil
}

// SaveDocument saves or updates a document
func (p *PostgresDB) SaveDocument(doc *Document) error {
	query := `
		INSERT INTO documents (name, path, ingress_time, folder, hash, ulid, document_type, full_text, url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT(path) DO UPDATE SET
			name = EXCLUDED.name,
			ingress_time = EXCLUDED.ingress_time,
			folder = EXCLUDED.folder,
			hash = EXCLUDED.hash,
			ulid = EXCLUDED.ulid,
			document_type = EXCLUDED.document_type,
			full_text = EXCLUDED.full_text,
			url = EXCLUDED.url,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`

	err := p.db.QueryRow(query,
		doc.Name, doc.Path, doc.IngressTime, doc.Folder, doc.Hash,
		doc.ULID.String(), doc.DocumentType, doc.FullText, doc.URL,
	).Scan(&doc.StormID)

	return err
}

// GetDocumentByID retrieves a document by ID
func (p *PostgresDB) GetDocumentByID(id int) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE id = $1`

	doc := &Document{}
	var ulidStr string

	err := p.db.QueryRow(query, id).Scan(
		&doc.StormID, &doc.Name, &doc.Path, &doc.IngressTime,
		&doc.Folder, &doc.Hash, &ulidStr, &doc.DocumentType,
		&doc.FullText, &doc.URL,
	)

	if err != nil {
		return nil, err
	}

	ulid, err := ulid.Parse(ulidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ULID: %w", err)
	}
	doc.ULID = ulid

	return doc, nil
}

// GetDocumentByULID retrieves a document by ULID
func (p *PostgresDB) GetDocumentByULID(ulidStr string) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE ulid = $1`

	doc := &Document{}
	var docUlidStr string

	err := p.db.QueryRow(query, ulidStr).Scan(
		&doc.StormID, &doc.Name, &doc.Path, &doc.IngressTime,
		&doc.Folder, &doc.Hash, &docUlidStr, &doc.DocumentType,
		&doc.FullText, &doc.URL,
	)

	if err != nil {
		return nil, err
	}

	ulid, err := ulid.Parse(docUlidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ULID: %w", err)
	}
	doc.ULID = ulid

	return doc, nil
}

// GetDocumentByPath retrieves a document by file path
func (p *PostgresDB) GetDocumentByPath(path string) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE path = $1`

	doc := &Document{}
	var ulidStr string

	err := p.db.QueryRow(query, path).Scan(
		&doc.StormID, &doc.Name, &doc.Path, &doc.IngressTime,
		&doc.Folder, &doc.Hash, &ulidStr, &doc.DocumentType,
		&doc.FullText, &doc.URL,
	)

	if err != nil {
		return nil, err
	}

	ulid, err := ulid.Parse(ulidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ULID: %w", err)
	}
	doc.ULID = ulid

	return doc, nil
}

// GetDocumentByHash retrieves a document by hash
func (p *PostgresDB) GetDocumentByHash(hash string) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE hash = $1`

	doc := &Document{}
	var ulidStr string

	err := p.db.QueryRow(query, hash).Scan(
		&doc.StormID, &doc.Name, &doc.Path, &doc.IngressTime,
		&doc.Folder, &doc.Hash, &ulidStr, &doc.DocumentType,
		&doc.FullText, &doc.URL,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No duplicate found
	}
	if err != nil {
		return nil, err
	}

	ulid, err := ulid.Parse(ulidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ULID: %w", err)
	}
	doc.ULID = ulid

	return doc, nil
}

// scanDocuments is a helper function to scan rows into Document structs
func scanDocuments(rows *sql.Rows) ([]Document, error) {
	var documents []Document

	for rows.Next() {
		doc := Document{}
		var ulidStr string

		err := rows.Scan(
			&doc.StormID, &doc.Name, &doc.Path, &doc.IngressTime,
			&doc.Folder, &doc.Hash, &ulidStr, &doc.DocumentType,
			&doc.FullText, &doc.URL,
		)
		if err != nil {
			return nil, err
		}

		ulid, err := ulid.Parse(ulidStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ULID: %w", err)
		}
		doc.ULID = ulid

		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// GetNewestDocuments retrieves the newest documents
func (p *PostgresDB) GetNewestDocuments(limit int) ([]Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents ORDER BY ingress_time DESC LIMIT $1`

	rows, err := p.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

// GetAllDocuments retrieves all documents
func (p *PostgresDB) GetAllDocuments() ([]Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents ORDER BY id`

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

// GetDocumentsByFolder retrieves documents in a specific folder
func (p *PostgresDB) GetDocumentsByFolder(folder string) ([]Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE folder = $1`

	rows, err := p.db.Query(query, folder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

// DeleteDocument deletes a document by ULID
func (p *PostgresDB) DeleteDocument(ulidStr string) error {
	query := `DELETE FROM documents WHERE ulid = $1`
	_, err := p.db.Exec(query, ulidStr)
	return err
}

// UpdateDocumentURL updates the URL field of a document
func (p *PostgresDB) UpdateDocumentURL(ulidStr string, url string) error {
	query := `UPDATE documents SET url = $1, updated_at = CURRENT_TIMESTAMP WHERE ulid = $2`
	_, err := p.db.Exec(query, url, ulidStr)
	return err
}

// UpdateDocumentFolder updates the Folder field of a document
func (p *PostgresDB) UpdateDocumentFolder(ulidStr string, folder string) error {
	query := `UPDATE documents SET folder = $1, updated_at = CURRENT_TIMESTAMP WHERE ulid = $2`
	_, err := p.db.Exec(query, folder, ulidStr)
	return err
}

// SaveConfig saves server configuration
func (p *PostgresDB) SaveConfig(cfg *config.ServerConfig) error {
	query := `
		UPDATE server_config SET
			listen_addr_ip = $1,
			listen_addr_port = $2,
			ingress_path = $3,
			ingress_delete = $4,
			ingress_move_folder = $5,
			ingress_preserve = $6,
			document_path = $7,
			new_document_folder = $8,
			new_document_folder_rel = $9,
			web_ui_pass = $10,
			client_username = $11,
			client_password = $12,
			pushbullet_token = $13,
			tesseract_path = $14,
			use_reverse_proxy = $15,
			base_url = $16,
			ingress_interval = $17,
			new_document_number = $18,
			server_api_url = $19
		WHERE id = 1
	`

	_, err := p.db.Exec(query,
		cfg.ListenAddrIP, cfg.ListenAddrPort, cfg.IngressPath,
		cfg.IngressDelete, cfg.IngressMoveFolder, cfg.IngressPreserve,
		cfg.DocumentPath, cfg.NewDocumentFolder, cfg.NewDocumentFolderRel,
		cfg.WebUIPass, cfg.ClientUsername, cfg.ClientPassword,
		cfg.PushBulletToken, cfg.TesseractPath, cfg.UseReverseProxy,
		cfg.BaseURL, cfg.IngressInterval,
		cfg.FrontEndConfig.NewDocumentNumber, cfg.FrontEndConfig.ServerAPIURL,
	)

	return err
}

// GetConfig retrieves server configuration
func (p *PostgresDB) GetConfig() (*config.ServerConfig, error) {
	query := `
		SELECT listen_addr_ip, listen_addr_port, ingress_path, ingress_delete,
		       ingress_move_folder, ingress_preserve, document_path, new_document_folder,
		       new_document_folder_rel, web_ui_pass, client_username, client_password,
		       pushbullet_token, tesseract_path, use_reverse_proxy, base_url,
		       ingress_interval, new_document_number, server_api_url
		FROM server_config WHERE id = 1
	`

	cfg := &config.ServerConfig{}
	err := p.db.QueryRow(query).Scan(
		&cfg.ListenAddrIP, &cfg.ListenAddrPort, &cfg.IngressPath,
		&cfg.IngressDelete, &cfg.IngressMoveFolder, &cfg.IngressPreserve,
		&cfg.DocumentPath, &cfg.NewDocumentFolder, &cfg.NewDocumentFolderRel,
		&cfg.WebUIPass, &cfg.ClientUsername, &cfg.ClientPassword,
		&cfg.PushBulletToken, &cfg.TesseractPath, &cfg.UseReverseProxy,
		&cfg.BaseURL, &cfg.IngressInterval,
		&cfg.FrontEndConfig.NewDocumentNumber, &cfg.FrontEndConfig.ServerAPIURL,
	)

	if err != nil {
		return nil, err
	}

	cfg.StormID = 1
	return cfg, nil
}

// GetNewestDocumentsWithPagination retrieves documents with pagination support
func (p *PostgresDB) GetNewestDocumentsWithPagination(page int, pageSize int) ([]Document, int, error) {
	// Calculate offset
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM documents`
	err := p.db.QueryRow(countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated documents
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents ORDER BY ingress_time DESC LIMIT $1 OFFSET $2`

	rows, err := p.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	docs, err := scanDocuments(rows)
	if err != nil {
		return nil, 0, err
	}

	return docs, totalCount, nil
}

// SearchDocuments performs full-text search using PostgreSQL's native search capabilities
// Supports both prefix matching and phrase search
func (p *PostgresDB) SearchDocuments(searchTerm string) ([]Document, error) {
	// Convert search term to tsquery format
	// For prefix search: "test" becomes "test:*"
	// For phrase search: "test document" becomes "test <-> document"

	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents
	          WHERE full_text_search @@ to_tsquery('english', $1)
	          ORDER BY ts_rank(full_text_search, to_tsquery('english', $1)) DESC`

	// Format the search term for PostgreSQL full-text search
	// Add prefix matching support with :*
	formattedTerm := formatSearchTerm(searchTerm)

	rows, err := p.db.Query(query, formattedTerm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

// formatSearchTerm converts a search term into PostgreSQL tsquery format
func formatSearchTerm(term string) string {
	// Remove special characters that would break tsquery
	term = strings.TrimSpace(term)
	if term == "" {
		return ""
	}

	// Check if it's a phrase (contains spaces)
	if strings.Contains(term, " ") {
		// Phrase search: split into words and join with <->
		words := strings.Fields(term)
		for i := range words {
			words[i] = strings.ToLower(words[i]) + ":*"
		}
		return strings.Join(words, " <-> ")
	}

	// Single word: add prefix matching
	return strings.ToLower(term) + ":*"
}
