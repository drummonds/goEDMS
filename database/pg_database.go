package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/drummonds/go-postgres"
	"github.com/drummonds/godocs/config"
	_ "github.com/lib/pq"
	"github.com/oklog/ulid/v2"
)

// formatSearchTerm converts a search term into PostgreSQL tsquery format
func formatSearchTerm(term string) string {
	term = strings.TrimSpace(term)
	if term == "" {
		return ""
	}

	if strings.Contains(term, " ") {
		words := strings.Fields(term)
		for i := range words {
			words[i] = strings.ToLower(words[i]) + ":*"
		}
		return strings.Join(words, " <-> ")
	}

	return strings.ToLower(term) + ":*"
}

// PGDB implements Repository using database/sql with raw PostgreSQL SQL.
// For production it connects via lib/pq; for dev/testing via go-postgres ("pglike")
// which transparently translates PostgreSQL SQL to SQLite.
type PGDB struct {
	db       *sql.DB
	isPglike bool // true when using go-postgres/SQLite backend
}

// NewRepository initializes the database based on configuration
func NewRepository(cfg config.ServerConfig) *PGDB {
	// databases dir used by sqlite so might as well make for all
	if _, err := os.Stat("databases"); os.IsNotExist(err) {
		if err := os.Mkdir("databases", os.ModePerm); err != nil {
			Logger.Error("Unable to create folder for databases", "error", err)
			os.Exit(1)
		}
	}

	var (
		sqlDB    *sql.DB
		isPglike bool
		err      error
	)

	dbType := cfg.DatabaseType
	switch dbType {
	case "postgres", "cockroachdb":
		Logger.Info("Initializing postgres database...", "type", dbType)
		userpw := cfg.DatabaseUser
		if cfg.DatabasePassword != "" {
			userpw += fmt.Sprintf(":%s", cfg.DatabasePassword)
		}
		connectionString := fmt.Sprintf("%s://%s@%s:%s/%s?sslmode=%s",
			cfg.DatabaseType, userpw, cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseDbname, cfg.DatabaseSslmode)
		Logger.Info("Connection string", "connectionString", connectionString)
		sqlDB, err = sql.Open("postgres", connectionString)
		if err != nil {
			Logger.Error("Failed to open database", "error", err)
			os.Exit(1)
		}

	case "pglike":
		Logger.Info("Initializing go-postgres (pglike) database...", "type", dbType)
		dbName := cfg.DatabaseDbname
		if dbName == "" {
			dbName = "databases/godocs.db"
		}
		sqlDB, err = sql.Open("pglike", dbName)
		if err != nil {
			Logger.Error("Failed to open pglike database", "error", err)
			os.Exit(1)
		}
		isPglike = true

	case "ephemeral":
		Logger.Info("Starting ephemeral PostgreSQL database for development")
		ephDB, err := SetupEphemeralPostgresDatabase()
		if err != nil {
			Logger.Error("Failed to setup ephemeral database", "error", err)
			os.Exit(1)
		}
		return ephDB

	default:
		Logger.Error("Unknown database type", "type", dbType)
		Logger.Info("Supported database types: ephemeral, postgres, cockroachdb, pglike")
		os.Exit(1)
	}

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		Logger.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}
	Logger.Info("Connected to database successfully", "type", dbType)

	// Run migrations
	Logger.Info("Running database migrations...")
	if err := runMigrations(context.Background(), sqlDB, isPglike); err != nil {
		Logger.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}
	Logger.Info("Database migrations completed successfully")

	return &PGDB{db: sqlDB, isPglike: isPglike}
}

// Close closes the database connection
func (p *PGDB) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// scanDocument scans a single row into a Document, parsing the ULID string.
// Uses timeScanner to handle both time.Time (PostgreSQL) and string (pglike/SQLite) timestamps.
func scanDocument(row interface{ Scan(dest ...any) error }) (*Document, error) {
	doc := &Document{}
	var ulidStr string
	var ingressTime timeScanner
	var docDate, createdDate, updatedDate nullTimeScanner
	err := row.Scan(
		&doc.ID, &doc.Name, &doc.Path, &ingressTime,
		&doc.Folder, &doc.Hash, &ulidStr, &doc.DocumentType,
		&doc.FullText, &doc.URL, &docDate,
		&createdDate, &updatedDate, &doc.Author, &doc.SourceURL, &doc.Source,
	)
	if err != nil {
		return nil, err
	}
	doc.IngressTime = ingressTime.Time
	doc.DocumentDate = docDate.Time
	doc.CreatedDate = createdDate.Time
	doc.UpdatedDate = updatedDate.Time
	doc.ULID, err = ulid.Parse(ulidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ULID: %w", err)
	}
	return doc, nil
}

// scanDocumentRows scans multiple rows into Documents
func scanDocumentRows(rows *sql.Rows) ([]Document, error) {
	var docs []Document
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, *doc)
	}
	return docs, rows.Err()
}

const docColumns = `id, name, path, ingress_time, folder, hash, ulid, document_type, full_text, url, document_date, created_date, updated_date, author, source_url, source`

const docColumnsAliased = `d.id, d.name, d.path, d.ingress_time, d.folder, d.hash, d.ulid, d.document_type, d.full_text, d.url, d.document_date, d.created_date, d.updated_date, d.author, d.source_url, d.source`

// SaveDocument saves or updates a document
func (p *PGDB) SaveDocument(doc *Document) error {
	ctx := context.Background()

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO documents (name, path, ingress_time, folder, hash, ulid, document_type, full_text, url, document_date,
			created_date, updated_date, author, source_url, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (path) DO UPDATE SET
			name = EXCLUDED.name,
			ingress_time = EXCLUDED.ingress_time,
			folder = EXCLUDED.folder,
			hash = EXCLUDED.hash,
			ulid = EXCLUDED.ulid,
			document_type = EXCLUDED.document_type,
			full_text = EXCLUDED.full_text,
			url = EXCLUDED.url,
			document_date = EXCLUDED.document_date,
			created_date = EXCLUDED.created_date,
			updated_date = EXCLUDED.updated_date,
			author = EXCLUDED.author,
			source_url = EXCLUDED.source_url,
			source = EXCLUDED.source,
			updated_at = CURRENT_TIMESTAMP
	`, doc.Name, doc.Path, doc.IngressTime, doc.Folder, doc.Hash,
		doc.ULID.String(), doc.DocumentType, doc.FullText, doc.URL, doc.DocumentDate,
		doc.CreatedDate, doc.UpdatedDate, doc.Author, doc.SourceURL, doc.Source)
	if err != nil {
		return err
	}

	// Fetch the auto-generated ID
	return p.db.QueryRowContext(ctx,
		`SELECT id FROM documents WHERE path = $1`, doc.Path).Scan(&doc.ID)
}

// GetDocumentByID retrieves a document by ID
func (p *PGDB) GetDocumentByID(id int) (*Document, error) {
	return scanDocument(p.db.QueryRowContext(context.Background(),
		`SELECT `+docColumns+` FROM documents WHERE id = $1`, id))
}

// GetDocumentByULID retrieves a document by ULID
func (p *PGDB) GetDocumentByULID(ulidStr string) (*Document, error) {
	return scanDocument(p.db.QueryRowContext(context.Background(),
		`SELECT `+docColumns+` FROM documents WHERE ulid = $1`, ulidStr))
}

// GetDocumentByPath retrieves a document by file path
func (p *PGDB) GetDocumentByPath(path string) (*Document, error) {
	return scanDocument(p.db.QueryRowContext(context.Background(),
		`SELECT `+docColumns+` FROM documents WHERE path = $1`, path))
}

// GetDocumentByHash retrieves a document by hash
func (p *PGDB) GetDocumentByHash(hash string) (*Document, error) {
	doc, err := scanDocument(p.db.QueryRowContext(context.Background(),
		`SELECT `+docColumns+` FROM documents WHERE hash = $1`, hash))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return doc, err
}

// GetNewestDocuments retrieves the newest documents
func (p *PGDB) GetNewestDocuments(limit int) ([]Document, error) {
	rows, err := p.db.QueryContext(context.Background(),
		`SELECT `+docColumns+` FROM documents ORDER BY ingress_time DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

// GetNewestDocumentsWithPagination retrieves documents with pagination support
func (p *PGDB) GetNewestDocumentsWithPagination(page int, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	var totalCount int
	if err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	rows, err := p.db.QueryContext(ctx,
		`SELECT `+docColumns+` FROM documents ORDER BY ingress_time DESC LIMIT $1 OFFSET $2`,
		pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	docs, err := scanDocumentRows(rows)
	return docs, totalCount, err
}

// GetAllDocuments retrieves all documents
func (p *PGDB) GetAllDocuments() ([]Document, error) {
	rows, err := p.db.QueryContext(context.Background(),
		`SELECT `+docColumns+` FROM documents ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

// GetDocumentsByFolder retrieves documents in a specific folder
func (p *PGDB) GetDocumentsByFolder(folder string) ([]Document, error) {
	rows, err := p.db.QueryContext(context.Background(),
		`SELECT `+docColumns+` FROM documents WHERE folder = $1`, folder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

// DeleteDocument deletes a document by ULID
func (p *PGDB) DeleteDocument(ulidStr string) error {
	_, err := p.db.ExecContext(context.Background(),
		`DELETE FROM documents WHERE ulid = $1`, ulidStr)
	return err
}

// UpdateDocumentURL updates the URL field of a document
func (p *PGDB) UpdateDocumentURL(ulidStr string, url string) error {
	_, err := p.db.ExecContext(context.Background(),
		`UPDATE documents SET url = $1, updated_at = CURRENT_TIMESTAMP WHERE ulid = $2`, url, ulidStr)
	return err
}

// UpdateDocumentPath updates the path and folder fields of a document
func (p *PGDB) UpdateDocumentPath(ulidStr string, path string, folder string) error {
	_, err := p.db.ExecContext(context.Background(),
		`UPDATE documents SET path = $1, folder = $2, updated_at = CURRENT_TIMESTAMP WHERE ulid = $3`,
		path, folder, ulidStr)
	return err
}

// UpdateDocumentFolder updates the Folder field of a document
func (p *PGDB) UpdateDocumentFolder(ulidStr string, folder string) error {
	_, err := p.db.ExecContext(context.Background(),
		`UPDATE documents SET folder = $1, updated_at = CURRENT_TIMESTAMP WHERE ulid = $2`, folder, ulidStr)
	return err
}

// UpdateDocumentDate updates the document_date field of a document
func (p *PGDB) UpdateDocumentDate(ulidStr string, date *time.Time) error {
	_, err := p.db.ExecContext(context.Background(),
		`UPDATE documents SET document_date = $1, updated_at = CURRENT_TIMESTAMP WHERE ulid = $2`, date, ulidStr)
	return err
}

// UpdateDocumentFullText updates the full_text field of a document
func (p *PGDB) UpdateDocumentFullText(ulidStr string, text string) error {
	_, err := p.db.ExecContext(context.Background(),
		`UPDATE documents SET full_text = $1, updated_at = CURRENT_TIMESTAMP WHERE ulid = $2`, text, ulidStr)
	return err
}

// UpdateDocumentMetadata updates multiple metadata fields in a single query.
// Only non-nil fields in the update struct are applied.
func (p *PGDB) UpdateDocumentMetadata(ulidStr string, meta DocumentMetadataUpdate) error {
	var setClauses []string
	var args []interface{}
	argIdx := 1

	if meta.CreatedDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("created_date = $%d", argIdx))
		args = append(args, *meta.CreatedDate)
		argIdx++
	}
	if meta.UpdatedDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("updated_date = $%d", argIdx))
		args = append(args, *meta.UpdatedDate)
		argIdx++
	}
	if meta.Author != nil {
		setClauses = append(setClauses, fmt.Sprintf("author = $%d", argIdx))
		args = append(args, *meta.Author)
		argIdx++
	}
	if meta.SourceURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("source_url = $%d", argIdx))
		args = append(args, *meta.SourceURL)
		argIdx++
	}
	if meta.Source != nil {
		setClauses = append(setClauses, fmt.Sprintf("source = $%d", argIdx))
		args = append(args, *meta.Source)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil // nothing to update
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE documents SET %s WHERE ulid = $%d",
		strings.Join(setClauses, ", "), argIdx)
	args = append(args, ulidStr)

	_, err := p.db.ExecContext(context.Background(), query, args...)
	return err
}

// SaveConfig saves server configuration
func (p *PGDB) SaveConfig(cfg *config.ServerConfig) error {
	_, err := p.db.ExecContext(context.Background(), `
		UPDATE server_config SET
			listen_addr_ip = $1, listen_addr_port = $2, ingress_path = $3,
			ingress_delete = $4, ingress_move_folder = $5, ingress_preserve = $6,
			document_path = $7, new_document_folder = $8, new_document_folder_rel = $9,
			web_ui_pass = $10, client_username = $11, client_password = $12,
			pushbullet_token = $13, tesseract_path = $14, use_reverse_proxy = $15,
			base_url = $16, ingress_interval = $17,
			new_document_number = $18, server_api_url = $19
		WHERE id = 1`,
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
func (p *PGDB) GetConfig() (*config.ServerConfig, error) {
	cfg := &config.ServerConfig{ID: 1}
	err := p.db.QueryRowContext(context.Background(), `
		SELECT listen_addr_ip, listen_addr_port, ingress_path, ingress_delete,
		       ingress_move_folder, ingress_preserve, document_path, new_document_folder,
		       new_document_folder_rel, web_ui_pass, client_username, client_password,
		       pushbullet_token, tesseract_path, use_reverse_proxy, base_url,
		       ingress_interval, new_document_number, server_api_url
		FROM server_config WHERE id = 1`).Scan(
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
	return cfg, nil
}

// SearchDocuments performs full-text search
func (p *PGDB) SearchDocuments(searchTerm string) ([]Document, error) {
	ctx := context.Background()
	var rows *sql.Rows
	var err error

	if !p.isPglike {
		// PostgreSQL full-text search with tsvector
		formattedTerm := formatSearchTerm(searchTerm)
		rows, err = p.db.QueryContext(ctx, `
			SELECT `+docColumns+`
			FROM documents
			WHERE full_text_search @@ to_tsquery('english', $1)
			ORDER BY ts_rank(full_text_search, to_tsquery('english', $1)) DESC`,
			formattedTerm)
	} else {
		// SQLite/pglike: LIKE fallback
		searchPattern := "%" + searchTerm + "%"
		rows, err = p.db.QueryContext(ctx, `
			SELECT `+docColumns+`
			FROM documents
			WHERE full_text LIKE $1 OR name LIKE $2`,
			searchPattern, searchPattern)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

// ReindexSearchDocuments reindexes all documents for full-text search
func (p *PGDB) ReindexSearchDocuments() (int, error) {
	if p.isPglike {
		return 0, nil // SQLite doesn't need reindexing for LIKE searches
	}

	result, err := p.db.ExecContext(context.Background(), `
		UPDATE documents
		SET full_text_search = to_tsvector('english', COALESCE(full_text, '') || ' ' || COALESCE(name, ''))
		WHERE full_text IS NOT NULL AND full_text != ''`)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	return int(count), err
}

// GetSchemaVersion returns the latest applied migration version
func (p *PGDB) GetSchemaVersion() (string, error) {
	var version string
	err := p.db.QueryRowContext(context.Background(), `
		SELECT version FROM schema_migrations
		ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err == sql.ErrNoRows {
		return "none", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get schema version: %w", err)
	}
	return version, nil
}

// ============================================================================
// JOB METHODS
// ============================================================================

const jobColumns = `id, type, status, progress, current_step, total_steps, message, error, result, created_at, updated_at, started_at, completed_at`

// scanJob scans a single row into a Job, parsing the ULID string.
// Uses timeScanner/nullTimeScanner to handle both time.Time (PostgreSQL) and string (pglike/SQLite) timestamps.
func scanJob(row interface{ Scan(dest ...any) error }) (*Job, error) {
	job := &Job{}
	var idStr string
	var createdAt, updatedAt timeScanner
	var startedAt, completedAt nullTimeScanner
	err := row.Scan(
		&idStr, &job.Type, &job.Status, &job.Progress,
		&job.CurrentStep, &job.TotalSteps, &job.Message,
		&job.Error, &job.Result,
		&createdAt, &updatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	job.CreatedAt = createdAt.Time
	job.UpdatedAt = updatedAt.Time
	job.StartedAt = startedAt.Time
	job.CompletedAt = completedAt.Time
	job.ID, err = ulid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse job ULID: %w", err)
	}
	return job, nil
}

// scanJobRows scans multiple rows into Jobs
func scanJobRows(rows *sql.Rows) ([]Job, error) {
	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

// CreateJob creates a new job in the database
func (p *PGDB) CreateJob(jobType JobType, message string) (*Job, error) {
	ctx := context.Background()
	now := time.Now()
	jobID, err := CalculateUUID(now)
	if err != nil {
		return nil, err
	}

	job := &Job{
		ID:        jobID,
		Type:      jobType,
		Status:    JobStatusPending,
		Message:   message,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = p.db.ExecContext(ctx, `
		INSERT INTO jobs (id, type, status, progress, current_step, total_steps, message, error, result, created_at, updated_at, started_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		job.ID.String(), job.Type, job.Status, job.Progress, job.CurrentStep,
		job.TotalSteps, job.Message, job.Error, job.Result,
		job.CreatedAt, job.UpdatedAt, job.StartedAt, job.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// UpdateJobProgress updates the progress of a job
func (p *PGDB) UpdateJobProgress(jobID ulid.ULID, progress int, currentStep string) error {
	_, err := p.db.ExecContext(context.Background(), `
		UPDATE jobs SET progress = $1, current_step = $2, updated_at = $3
		WHERE id = $4`,
		progress, currentStep, time.Now(), jobID.String())
	return err
}

// UpdateJobStatus updates the status of a job
func (p *PGDB) UpdateJobStatus(jobID ulid.ULID, status JobStatus, message string) error {
	now := time.Now()
	var startedAt, completedAt interface{}

	if status == JobStatusRunning {
		startedAt = now
	}
	if status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled {
		completedAt = now
	}

	_, err := p.db.ExecContext(context.Background(), `
		UPDATE jobs SET status = $1, message = $2, updated_at = $3,
			started_at = COALESCE(started_at, $4), completed_at = $5
		WHERE id = $6`,
		status, message, now, startedAt, completedAt, jobID.String())
	return err
}

// UpdateJobError updates a job with an error
func (p *PGDB) UpdateJobError(jobID ulid.ULID, errorMsg string) error {
	now := time.Now()
	_, err := p.db.ExecContext(context.Background(), `
		UPDATE jobs SET status = $1, error = $2, updated_at = $3, completed_at = $4
		WHERE id = $5`,
		JobStatusFailed, errorMsg, now, now, jobID.String())
	return err
}

// CompleteJob marks a job as completed with optional result data
func (p *PGDB) CompleteJob(jobID ulid.ULID, result string) error {
	now := time.Now()
	_, err := p.db.ExecContext(context.Background(), `
		UPDATE jobs SET status = $1, progress = 100, result = $2, updated_at = $3, completed_at = $4
		WHERE id = $5`,
		JobStatusCompleted, result, now, now, jobID.String())
	return err
}

// GetJob retrieves a job by ID
func (p *PGDB) GetJob(jobID ulid.ULID) (*Job, error) {
	return scanJob(p.db.QueryRowContext(context.Background(),
		`SELECT `+jobColumns+` FROM jobs WHERE id = $1`, jobID.String()))
}

// GetRecentJobs retrieves the most recent jobs with pagination
func (p *PGDB) GetRecentJobs(limit, offset int) ([]Job, error) {
	rows, err := p.db.QueryContext(context.Background(),
		`SELECT `+jobColumns+` FROM jobs ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobRows(rows)
}

// GetActiveJobs retrieves all running or pending jobs
func (p *PGDB) GetActiveJobs() ([]Job, error) {
	rows, err := p.db.QueryContext(context.Background(),
		`SELECT `+jobColumns+` FROM jobs WHERE status IN ($1, $2) ORDER BY created_at DESC`,
		JobStatusPending, JobStatusRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobRows(rows)
}

// DeleteOldJobs deletes completed jobs older than the specified duration
func (p *PGDB) DeleteOldJobs(olderThan time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-olderThan)
	result, err := p.db.ExecContext(context.Background(), `
		DELETE FROM jobs
		WHERE status IN ($1, $2, $3) AND completed_at < $4`,
		JobStatusCompleted, JobStatusFailed, JobStatusCancelled, cutoffTime)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	return int(count), err
}

// ============================================================================
// WORD CLOUD METHODS
// ============================================================================

// GetTopWords retrieves the top N most frequent words
func (p *PGDB) GetTopWords(limit int) ([]WordFrequency, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := p.db.QueryContext(context.Background(), `
		SELECT word, frequency, last_updated
		FROM word_frequencies
		ORDER BY frequency DESC, word ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top words: %w", err)
	}
	defer rows.Close()

	words := make([]WordFrequency, 0)
	for rows.Next() {
		var wf WordFrequency
		var updated timeScanner
		if err := rows.Scan(&wf.Word, &wf.Frequency, &updated); err != nil {
			return nil, fmt.Errorf("failed to scan word frequency: %w", err)
		}
		wf.Updated = updated.Time
		words = append(words, wf)
	}
	return words, rows.Err()
}

// GetWordCloudMetadata retrieves metadata about the word cloud
func (p *PGDB) GetWordCloudMetadata() (*WordCloudMetadata, error) {
	var meta WordCloudMetadata
	var lastCalc nullTimeScanner

	err := p.db.QueryRowContext(context.Background(), `
		SELECT last_full_calculation, total_documents_processed,
		       total_words_indexed, version
		FROM word_cloud_metadata WHERE id = 1`).Scan(
		&lastCalc, &meta.TotalDocsProcessed, &meta.TotalWordsIndexed, &meta.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	if lastCalc.Valid && lastCalc.Time != nil {
		meta.LastCalculation = *lastCalc.Time
	}
	return &meta, nil
}

// RecalculateAllWordFrequencies performs a full recalculation of word frequencies
func (p *PGDB) RecalculateAllWordFrequencies() error {
	ctx := context.Background()
	Logger.Info("Starting full word cloud recalculation")

	// Clear existing frequencies (DELETE FROM works on both PG and SQLite)
	if _, err := p.db.ExecContext(ctx, `DELETE FROM word_frequencies`); err != nil {
		return fmt.Errorf("failed to clear word frequencies: %w", err)
	}

	docs, err := p.GetAllDocuments()
	if err != nil {
		return fmt.Errorf("failed to get documents: %w", err)
	}

	Logger.Info("Processing documents for word cloud", "count", len(docs))

	tokenizer := NewWordTokenizer()
	globalFrequencies := make(map[string]int)

	for _, doc := range docs {
		combinedText := doc.FullText + " " + doc.Name
		for word, count := range tokenizer.TokenizeAndCount(combinedText) {
			globalFrequencies[word] += count
		}
	}

	Logger.Info("Inserting word frequencies", "unique_words", len(globalFrequencies))

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO word_frequencies (word, frequency, last_updated)
		VALUES ($1, $2, CURRENT_TIMESTAMP)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for word, count := range globalFrequencies {
		if _, err := stmt.ExecContext(ctx, word, count); err != nil {
			return fmt.Errorf("failed to insert word frequency: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update metadata
	_, err = p.db.ExecContext(ctx, `
		UPDATE word_cloud_metadata SET
			last_full_calculation = CURRENT_TIMESTAMP,
			total_documents_processed = $1,
			total_words_indexed = $2,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1`, len(docs), len(globalFrequencies))
	if err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	Logger.Info("Word cloud recalculation completed", "docs", len(docs), "words", len(globalFrequencies))
	return nil
}

// UpdateWordFrequencies updates word frequencies after document ingestion
func (p *PGDB) UpdateWordFrequencies(docID string) error {
	ctx := context.Background()

	doc, err := p.GetDocumentByULID(docID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	tokenizer := NewWordTokenizer()
	combinedText := doc.FullText + " " + doc.Name
	frequencies := tokenizer.TokenizeAndCount(combinedText)

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for word, count := range frequencies {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO word_frequencies (word, frequency, last_updated)
			VALUES ($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (word) DO UPDATE SET
				frequency = word_frequencies.frequency + EXCLUDED.frequency,
				last_updated = CURRENT_TIMESTAMP`, word, count)
		if err != nil {
			return fmt.Errorf("failed to update word frequency: %w", err)
		}
	}

	return tx.Commit()
}
