package database

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"

	config "github.com/deranjer/goEDMS/config"
	"github.com/oklog/ulid/v2"
)

// DBInterface defines database operations for future PostgreSQL support
type DBInterface interface {
	Close() error
	SaveDocument(doc *Document) error
	GetDocumentByID(id int) (*Document, error)
	GetDocumentByULID(ulid string) (*Document, error)
	GetDocumentByPath(path string) (*Document, error)
	GetDocumentByHash(hash string) (*Document, error)
	GetNewestDocuments(limit int) ([]Document, error)
	GetAllDocuments() ([]Document, error)
	GetDocumentsByFolder(folder string) ([]Document, error)
	DeleteDocument(ulid string) error
	UpdateDocumentURL(ulid string, url string) error
	UpdateDocumentFolder(ulid string, folder string) error
	SaveConfig(config *config.ServerConfig) error
	GetConfig() (*config.ServerConfig, error)
}

// SQLiteDB implements DBInterface for SQLite
type SQLiteDB struct {
	db *sql.DB
}

// SetupSQLiteDatabase initializes SQLite database with migrations
func SetupSQLiteDatabase() (*SQLiteDB, error) {
	// Create databases directory if it doesn't exist
	dbPath := "databases/goEDMS.db"

	// Open SQLite database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys and other optimizations
	if _, err := db.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA temp_store = MEMORY;
		PRAGMA cache_size = -64000;
	`); err != nil {
		return nil, fmt.Errorf("failed to set pragmas: %w", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

func runMigrations(db *sql.DB) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	migrationsPath, err := filepath.Abs("database/migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"sqlite3",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	Logger.Info("Database migrations completed successfully")
	return nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	return s.db.Close()
}

// SaveDocument saves or updates a document
func (s *SQLiteDB) SaveDocument(doc *Document) error {
	query := `
		INSERT INTO documents (name, path, ingress_time, folder, hash, ulid, document_type, full_text, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			ingress_time = excluded.ingress_time,
			folder = excluded.folder,
			hash = excluded.hash,
			ulid = excluded.ulid,
			document_type = excluded.document_type,
			full_text = excluded.full_text,
			url = excluded.url,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`

	err := s.db.QueryRow(query,
		doc.Name, doc.Path, doc.IngressTime, doc.Folder, doc.Hash,
		doc.ULID.String(), doc.DocumentType, doc.FullText, doc.URL,
	).Scan(&doc.StormID)

	return err
}

// GetDocumentByID retrieves a document by ID
func (s *SQLiteDB) GetDocumentByID(id int) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE id = ?`

	doc := &Document{}
	var ulidStr string

	err := s.db.QueryRow(query, id).Scan(
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
func (s *SQLiteDB) GetDocumentByULID(ulidStr string) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE ulid = ?`

	doc := &Document{}
	var docUlidStr string

	err := s.db.QueryRow(query, ulidStr).Scan(
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
func (s *SQLiteDB) GetDocumentByPath(path string) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE path = ?`

	doc := &Document{}
	var ulidStr string

	err := s.db.QueryRow(query, path).Scan(
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
func (s *SQLiteDB) GetDocumentByHash(hash string) (*Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE hash = ?`

	doc := &Document{}
	var ulidStr string

	err := s.db.QueryRow(query, hash).Scan(
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

// GetNewestDocuments retrieves the newest documents
func (s *SQLiteDB) GetNewestDocuments(limit int) ([]Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents ORDER BY ingress_time DESC LIMIT ?`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

// GetAllDocuments retrieves all documents
func (s *SQLiteDB) GetAllDocuments() ([]Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents ORDER BY id`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

// GetDocumentsByFolder retrieves documents in a specific folder
func (s *SQLiteDB) GetDocumentsByFolder(folder string) ([]Document, error) {
	query := `SELECT id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url
	          FROM documents WHERE folder = ?`

	rows, err := s.db.Query(query, folder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

// DeleteDocument deletes a document by ULID
func (s *SQLiteDB) DeleteDocument(ulidStr string) error {
	query := `DELETE FROM documents WHERE ulid = ?`
	_, err := s.db.Exec(query, ulidStr)
	return err
}

// UpdateDocumentURL updates the URL field of a document
func (s *SQLiteDB) UpdateDocumentURL(ulidStr string, url string) error {
	query := `UPDATE documents SET url = ?, updated_at = CURRENT_TIMESTAMP WHERE ulid = ?`
	_, err := s.db.Exec(query, url, ulidStr)
	return err
}

// UpdateDocumentFolder updates the Folder field of a document
func (s *SQLiteDB) UpdateDocumentFolder(ulidStr string, folder string) error {
	query := `UPDATE documents SET folder = ?, updated_at = CURRENT_TIMESTAMP WHERE ulid = ?`
	_, err := s.db.Exec(query, folder, ulidStr)
	return err
}

// SaveConfig saves server configuration
func (s *SQLiteDB) SaveConfig(cfg *config.ServerConfig) error {
	query := `
		UPDATE server_config SET
			listen_addr_ip = ?,
			listen_addr_port = ?,
			ingress_path = ?,
			ingress_delete = ?,
			ingress_move_folder = ?,
			ingress_preserve = ?,
			document_path = ?,
			new_document_folder = ?,
			new_document_folder_rel = ?,
			web_ui_pass = ?,
			client_username = ?,
			client_password = ?,
			pushbullet_token = ?,
			tesseract_path = ?,
			use_reverse_proxy = ?,
			base_url = ?,
			ingress_interval = ?,
			new_document_number = ?,
			server_api_url = ?
		WHERE id = 1
	`

	_, err := s.db.Exec(query,
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
func (s *SQLiteDB) GetConfig() (*config.ServerConfig, error) {
	query := `
		SELECT listen_addr_ip, listen_addr_port, ingress_path, ingress_delete,
		       ingress_move_folder, ingress_preserve, document_path, new_document_folder,
		       new_document_folder_rel, web_ui_pass, client_username, client_password,
		       pushbullet_token, tesseract_path, use_reverse_proxy, base_url,
		       ingress_interval, new_document_number, server_api_url
		FROM server_config WHERE id = 1
	`

	cfg := &config.ServerConfig{}
	err := s.db.QueryRow(query).Scan(
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

// Helper function to scan multiple documents from rows
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
