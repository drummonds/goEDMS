package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/drummonds/godocs/config"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/schema"
)

// BunDB implements Repository using Bun ORM
type BunDB struct {
	db     *bun.DB
	dbType string
}

// NewRepository initializes the database based on configuration
func NewRepository(config config.ServerConfig) *BunDB {
	// databases dir used by sqlite and ephemeral so might as well make for all
	_, err := os.Stat("databases")
	if err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir("databases", os.ModePerm)
			if err != nil {
				Logger.Error("Unable to create folder for databases", "error", err)
				os.Exit(1)
			}
		}
	}

	var (
		db      *bun.DB
		sqlDB   *sql.DB
		dialect schema.Dialect
	)

	dbType := config.DatabaseType
	if dbType == "ephemeral" {
		Logger.Info("Starting ephemeral PostgreSQL database for development")
		_, err := SetupEphemeralPostgresDatabase()
		if err != nil {
			Logger.Error("Failed to setup ephemeral database", "error", err)
			os.Exit(1)
		}
		// Run migrations
		Logger.Info("Running database migrations...")
		if err := runMigrations(context.Background(), db); err != nil {
			Logger.Error("failed to ping database", "error", err)
			os.Exit(1)
		}
		Logger.Info("Database migrations completed successfully")

		result := new(BunDB)
		// result.db = ephemeralDB
		return result
	}
	switch dbType {
	case "postgres", "cockroachdb":
		Logger.Info("Initializing postgres database with Bun ORM...", "type", dbType)
		// Build the connection string for postgres/cockroachdb
		userpw := config.DatabaseUser
		if config.DatabasePassword != "" {
			userpw += fmt.Sprintf(":%s", config.DatabasePassword)
		}
		// eg postgres://user:password@localhost:5432/dbname?sslmode=disable
		connectionString := fmt.Sprintf("%s://%s@%s:%s/%s?sslmode=%s",
			config.DatabaseType, userpw, config.DatabaseHost, config.DatabasePort, config.DatabaseDbname, config.DatabaseSslmode)
		Logger.Info("Bun connection strings", "connectionString", connectionString)
		sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(connectionString)))
		// Test connection
		if err := sqlDB.Ping(); err != nil {
			Logger.Error("failed to ping database", "error", err)
			os.Exit(1)
		}

		dialect = pgdialect.New()

	case "sqlite":
		Logger.Info("Initializing sqlite database with Bun ORM...", "type", dbType)
		// Build the connection string for postgres/cockroachdb
		// eg "file:test.db?cache=shared&mode=rwc"
		dbName := config.DatabaseDbname
		if dbName == "" {
			dbName = "godocs"
		}
		// connectionString := "file:databases/test.sqlite:?cache=shared&mode=rwc"
		// connectionString := "file::memory:?cache=shared&mode=rwc"
		connectionString := fmt.Sprintf("file:%s?cache=shared&mode=rwc",
			config.DatabaseDbname)
		Logger.Info("Bun connection strings", "connectionString", connectionString)
		sqlDB, err = sql.Open(sqliteshim.ShimName, connectionString)

		dialect = sqlitedialect.New()

	default:
		Logger.Error("Unknown database type", "type", dbType)
		Logger.Info("Supported database types: ephemeral, postgres, cockroachdb, sqlite")
		os.Exit(1)
	}

	db = bun.NewDB(sqlDB, dialect)
	// Option to turn on verbose logging just returns failures otherwise
	db.AddQueryHook(bundebug.NewQueryHook((bundebug.WithVerbose(false))))
	Logger.Info("Connected to database successfully", "type", dbType)

	// Run migrations
	Logger.Info("Running database migrations...")
	if err := runMigrations(context.Background(), db); err != nil {
		Logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	Logger.Info("Database migrations completed successfully")

	result := new(BunDB)
	result.db = db
	result.dbType = dbType
	return result
}

// Close closes the database connection and stops embedded server if running
func (b *BunDB) Close() error {
	if b.db != nil {
		if err := b.db.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SaveDocument saves or updates a document
func (b *BunDB) SaveDocument(doc *Document) error {
	ctx := context.Background()
	bunDoc := FromDocument(doc)

	// Use INSERT ... ON CONFLICT for upsert behavior
	_, err := b.db.NewInsert().
		Model(bunDoc).
		On("CONFLICT (path) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("ingress_time = EXCLUDED.ingress_time").
		Set("folder = EXCLUDED.folder").
		Set("hash = EXCLUDED.hash").
		Set("ulid = EXCLUDED.ulid").
		Set("document_type = EXCLUDED.document_type").
		Set("full_text = EXCLUDED.full_text").
		Set("url = EXCLUDED.url").
		Set("updated_at = CURRENT_TIMESTAMP").
		Returning("id").
		Exec(ctx)

	if err != nil {
		return err
	}

	// Fetch the ID if it was auto-generated
	if bunDoc.ID == 0 {
		err = b.db.NewSelect().
			Model(bunDoc).
			Where("path = ?", bunDoc.Path).
			Scan(ctx)
		if err != nil {
			return err
		}
	}

	doc.StormID = bunDoc.ID
	return nil
}

// GetDocumentByID retrieves a document by ID
func (b *BunDB) GetDocumentByID(id int) (*Document, error) {
	ctx := context.Background()
	bunDoc := new(BunDocument)

	err := b.db.NewSelect().
		Model(bunDoc).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return bunDoc.ToDocument()
}

// GetDocumentByULID retrieves a document by ULID
func (b *BunDB) GetDocumentByULID(ulidStr string) (*Document, error) {
	ctx := context.Background()
	bunDoc := new(BunDocument)

	err := b.db.NewSelect().
		Model(bunDoc).
		Where("ulid = ?", ulidStr).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return bunDoc.ToDocument()
}

// GetDocumentByPath retrieves a document by file path
func (b *BunDB) GetDocumentByPath(path string) (*Document, error) {
	ctx := context.Background()
	bunDoc := new(BunDocument)

	err := b.db.NewSelect().
		Model(bunDoc).
		Where("path = ?", path).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return bunDoc.ToDocument()
}

// GetDocumentByHash retrieves a document by hash
func (b *BunDB) GetDocumentByHash(hash string) (*Document, error) {
	ctx := context.Background()
	bunDoc := new(BunDocument)

	err := b.db.NewSelect().
		Model(bunDoc).
		Where("hash = ?", hash).
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, nil // No duplicate found
	}
	if err != nil {
		return nil, err
	}

	return bunDoc.ToDocument()
}

// GetNewestDocuments retrieves the newest documents
func (b *BunDB) GetNewestDocuments(limit int) ([]Document, error) {
	ctx := context.Background()
	var bunDocs []BunDocument

	err := b.db.NewSelect().
		Model(&bunDocs).
		Order("ingress_time DESC").
		Limit(limit).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return b.bunDocsToDocuments(bunDocs)
}

// GetNewestDocumentsWithPagination retrieves documents with pagination support
func (b *BunDB) GetNewestDocumentsWithPagination(page int, pageSize int) ([]Document, int, error) {
	ctx := context.Background()

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get total count
	totalCount, err := b.db.NewSelect().
		Model((*BunDocument)(nil)).
		Count(ctx)

	if err != nil {
		return nil, 0, err
	}

	// Get paginated documents
	var bunDocs []BunDocument
	err = b.db.NewSelect().
		Model(&bunDocs).
		Order("ingress_time DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(ctx)

	if err != nil {
		return nil, 0, err
	}

	docs, err := b.bunDocsToDocuments(bunDocs)
	return docs, totalCount, err
}

// GetAllDocuments retrieves all documents
func (b *BunDB) GetAllDocuments() ([]Document, error) {
	ctx := context.Background()
	var bunDocs []BunDocument

	err := b.db.NewSelect().
		Model(&bunDocs).
		Order("id").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return b.bunDocsToDocuments(bunDocs)
}

// GetDocumentsByFolder retrieves documents in a specific folder
func (b *BunDB) GetDocumentsByFolder(folder string) ([]Document, error) {
	ctx := context.Background()
	var bunDocs []BunDocument

	err := b.db.NewSelect().
		Model(&bunDocs).
		Where("folder = ?", folder).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return b.bunDocsToDocuments(bunDocs)
}

// DeleteDocument deletes a document by ULID
func (b *BunDB) DeleteDocument(ulidStr string) error {
	ctx := context.Background()

	_, err := b.db.NewDelete().
		Model((*BunDocument)(nil)).
		Where("ulid = ?", ulidStr).
		Exec(ctx)

	return err
}

// UpdateDocumentURL updates the URL field of a document
func (b *BunDB) UpdateDocumentURL(ulidStr string, url string) error {
	ctx := context.Background()

	_, err := b.db.NewUpdate().
		Model((*BunDocument)(nil)).
		Set("url = ?", url).
		Set("updated_at = ?", time.Now()).
		Where("ulid = ?", ulidStr).
		Exec(ctx)

	return err
}

// UpdateDocumentFolder updates the Folder field of a document
func (b *BunDB) UpdateDocumentFolder(ulidStr string, folder string) error {
	ctx := context.Background()

	_, err := b.db.NewUpdate().
		Model((*BunDocument)(nil)).
		Set("folder = ?", folder).
		Set("updated_at = ?", time.Now()).
		Where("ulid = ?", ulidStr).
		Exec(ctx)

	return err
}

// SaveConfig saves server configuration
func (b *BunDB) SaveConfig(cfg *config.ServerConfig) error {
	ctx := context.Background()

	bunConfig := &BunServerConfig{
		ID:                   1,
		ListenAddrIP:         cfg.ListenAddrIP,
		ListenAddrPort:       cfg.ListenAddrPort,
		IngressPath:          cfg.IngressPath,
		IngressDelete:        cfg.IngressDelete,
		IngressMoveFolder:    cfg.IngressMoveFolder,
		IngressPreserve:      cfg.IngressPreserve,
		DocumentPath:         cfg.DocumentPath,
		NewDocumentFolder:    cfg.NewDocumentFolder,
		NewDocumentFolderRel: cfg.NewDocumentFolderRel,
		WebUIPass:            cfg.WebUIPass,
		ClientUsername:       cfg.ClientUsername,
		ClientPassword:       cfg.ClientPassword,
		PushBulletToken:      cfg.PushBulletToken,
		TesseractPath:        cfg.TesseractPath,
		UseReverseProxy:      cfg.UseReverseProxy,
		BaseURL:              cfg.BaseURL,
		IngressInterval:      cfg.IngressInterval,
		NewDocumentNumber:    cfg.FrontEndConfig.NewDocumentNumber,
		ServerAPIURL:         cfg.FrontEndConfig.ServerAPIURL,
	}

	_, err := b.db.NewUpdate().
		Model(bunConfig).
		WherePK().
		Exec(ctx)

	return err
}

// GetConfig retrieves server configuration
func (b *BunDB) GetConfig() (*config.ServerConfig, error) {
	ctx := context.Background()
	bunConfig := &BunServerConfig{ID: 1}

	err := b.db.NewSelect().
		Model(bunConfig).
		WherePK().
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	cfg := &config.ServerConfig{
		StormID:              1,
		ListenAddrIP:         bunConfig.ListenAddrIP,
		ListenAddrPort:       bunConfig.ListenAddrPort,
		IngressPath:          bunConfig.IngressPath,
		IngressDelete:        bunConfig.IngressDelete,
		IngressMoveFolder:    bunConfig.IngressMoveFolder,
		IngressPreserve:      bunConfig.IngressPreserve,
		DocumentPath:         bunConfig.DocumentPath,
		NewDocumentFolder:    bunConfig.NewDocumentFolder,
		NewDocumentFolderRel: bunConfig.NewDocumentFolderRel,
		WebUIPass:            bunConfig.WebUIPass,
		ClientUsername:       bunConfig.ClientUsername,
		ClientPassword:       bunConfig.ClientPassword,
		PushBulletToken:      bunConfig.PushBulletToken,
		TesseractPath:        bunConfig.TesseractPath,
		UseReverseProxy:      bunConfig.UseReverseProxy,
		BaseURL:              bunConfig.BaseURL,
		IngressInterval:      bunConfig.IngressInterval,
	}

	cfg.FrontEndConfig.NewDocumentNumber = bunConfig.NewDocumentNumber
	cfg.FrontEndConfig.ServerAPIURL = bunConfig.ServerAPIURL

	return cfg, nil
}

// SearchDocuments performs full-text search
func (b *BunDB) SearchDocuments(searchTerm string) ([]Document, error) {
	ctx := context.Background()
	var bunDocs []BunDocument

	if b.dbType == "postgres" || b.dbType == "cockroachdb" {
		// Use PostgreSQL full-text search
		formattedTerm := formatSearchTerm(searchTerm)

		err := b.db.NewSelect().
			Model(&bunDocs).
			Where("full_text_search @@ to_tsquery('english', ?)", formattedTerm).
			OrderExpr("ts_rank(full_text_search, to_tsquery('english', ?)) DESC", formattedTerm).
			Scan(ctx)

		if err != nil {
			return nil, err
		}
	} else {
		// SQLite: Use simple LIKE search on full_text and name
		searchPattern := "%" + searchTerm + "%"

		err := b.db.NewSelect().
			Model(&bunDocs).
			Where("full_text LIKE ? OR name LIKE ?", searchPattern, searchPattern).
			Scan(ctx)

		if err != nil {
			return nil, err
		}
	}

	return b.bunDocsToDocuments(bunDocs)
}

// ReindexSearchDocuments reindexes all documents to populate the full_text_search column
func (b *BunDB) ReindexSearchDocuments() (int, error) {
	ctx := context.Background()

	if b.dbType == "postgres" || b.dbType == "cockroachdb" {
		result, err := b.db.NewUpdate().
			// PostgreSQL: Update full_text_search column
			Model((*BunDocument)(nil)).
			Set("full_text_search = to_tsvector('english', COALESCE(full_text, '') || ' ' || COALESCE(name, ''))").
			Where("full_text IS NOT NULL AND full_text != ''").
			Exec(ctx)

		if err != nil {
			return 0, err
		}

		rowsAffected, err := result.RowsAffected()
		return int(rowsAffected), err
	}

	// SQLite doesn't need reindexing for LIKE searches
	return 0, nil
}

// bunDocsToDocuments converts a slice of BunDocument to Document
func (b *BunDB) bunDocsToDocuments(bunDocs []BunDocument) ([]Document, error) {
	docs := make([]Document, 0, len(bunDocs))
	for _, bunDoc := range bunDocs {
		doc, err := bunDoc.ToDocument()
		if err != nil {
			return nil, err
		}
		docs = append(docs, *doc)
	}
	return docs, nil
}

// Job tracking methods
// CreateJob creates a new job in the database
func (b *BunDB) CreateJob(jobType JobType, message string) (*Job, error) {
	ctx := context.Background()
	now := time.Now()
	jobID, err := CalculateUUID(now)
	if err != nil {
		return nil, err
	}

	job := &Job{
		ID:          jobID,
		Type:        jobType,
		Status:      JobStatusPending,
		Progress:    0,
		CurrentStep: "",
		TotalSteps:  0,
		Message:     message,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	bunJob := FromJob(job)

	_, err = b.db.NewInsert().
		Model(bunJob).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	return job, nil
}

// UpdateJobProgress updates the progress of a job
func (b *BunDB) UpdateJobProgress(jobID ulid.ULID, progress int, currentStep string) error {
	ctx := context.Background()

	_, err := b.db.NewUpdate().
		Model((*BunJob)(nil)).
		Set("progress = ?", progress).
		Set("current_step = ?", currentStep).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", jobID.String()).
		Exec(ctx)

	return err
}

// UpdateJobStatus updates the status of a job
func (b *BunDB) UpdateJobStatus(jobID ulid.ULID, status JobStatus, message string) error {
	ctx := context.Background()
	now := time.Now()

	query := b.db.NewUpdate().
		Model((*BunJob)(nil)).
		Set("status = ?", status).
		Set("message = ?", message).
		Set("updated_at = ?", now)

	if status == JobStatusRunning {
		query = query.Set("started_at = COALESCE(started_at, ?)", now)
	}
	if status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled {
		query = query.Set("completed_at = ?", now)
	}

	_, err := query.Where("id = ?", jobID.String()).Exec(ctx)
	return err
}

// UpdateJobError updates a job with an error
func (b *BunDB) UpdateJobError(jobID ulid.ULID, errorMsg string) error {
	ctx := context.Background()
	now := time.Now()

	_, err := b.db.NewUpdate().
		Model((*BunJob)(nil)).
		Set("status = ?", JobStatusFailed).
		Set("error = ?", errorMsg).
		Set("updated_at = ?", now).
		Set("completed_at = ?", now).
		Where("id = ?", jobID.String()).
		Exec(ctx)

	return err
}

// CompleteJob marks a job as completed with optional result data
func (b *BunDB) CompleteJob(jobID ulid.ULID, result string) error {
	ctx := context.Background()
	now := time.Now()

	_, err := b.db.NewUpdate().
		Model((*BunJob)(nil)).
		Set("status = ?", JobStatusCompleted).
		Set("progress = ?", 100).
		Set("result = ?", result).
		Set("updated_at = ?", now).
		Set("completed_at = ?", now).
		Where("id = ?", jobID.String()).
		Exec(ctx)

	return err
}

// GetJob retrieves a job by ID
func (b *BunDB) GetJob(jobID ulid.ULID) (*Job, error) {
	ctx := context.Background()
	bunJob := new(BunJob)

	err := b.db.NewSelect().
		Model(bunJob).
		Where("id = ?", jobID.String()).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return bunJob.ToJob()
}

// GetRecentJobs retrieves the most recent jobs with pagination
func (b *BunDB) GetRecentJobs(limit, offset int) ([]Job, error) {
	ctx := context.Background()
	var bunJobs []BunJob

	err := b.db.NewSelect().
		Model(&bunJobs).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return b.bunJobsToJobs(bunJobs)
}

// GetActiveJobs retrieves all running or pending jobs
func (b *BunDB) GetActiveJobs() ([]Job, error) {
	ctx := context.Background()
	var bunJobs []BunJob

	err := b.db.NewSelect().
		Model(&bunJobs).
		Where("status IN (?)", bun.In([]string{string(JobStatusPending), string(JobStatusRunning)})).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return b.bunJobsToJobs(bunJobs)
}

// DeleteOldJobs deletes completed jobs older than the specified duration
func (b *BunDB) DeleteOldJobs(olderThan time.Duration) (int, error) {
	ctx := context.Background()
	cutoffTime := time.Now().Add(-olderThan)

	result, err := b.db.NewDelete().
		Model((*BunJob)(nil)).
		Where("status IN (?)", bun.In([]string{string(JobStatusCompleted), string(JobStatusFailed), string(JobStatusCancelled)})).
		Where("completed_at < ?", cutoffTime).
		Exec(ctx)

	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	return int(count), err
}

// bunJobsToJobs converts a slice of BunJob to Job
func (b *BunDB) bunJobsToJobs(bunJobs []BunJob) ([]Job, error) {
	jobs := make([]Job, 0, len(bunJobs))
	for _, bunJob := range bunJobs {
		job, err := bunJob.ToJob()
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

// Word cloud methods
// GetTopWords retrieves the top N most frequent words
func (b *BunDB) GetTopWords(limit int) ([]WordFrequency, error) {
	ctx := context.Background()

	if limit <= 0 {
		limit = 100
	}

	var bunWords []BunWordFrequency
	err := b.db.NewSelect().
		Model(&bunWords).
		Order("frequency DESC", "word ASC").
		Limit(limit).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	words := make([]WordFrequency, 0, len(bunWords))
	for _, bw := range bunWords {
		words = append(words, *bw.ToWordFrequency())
	}

	return words, nil
}

// GetWordCloudMetadata retrieves metadata about the word cloud
func (b *BunDB) GetWordCloudMetadata() (*WordCloudMetadata, error) {
	ctx := context.Background()
	bunMeta := &BunWordCloudMetadata{ID: 1}

	err := b.db.NewSelect().
		Model(bunMeta).
		WherePK().
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return bunMeta.ToWordCloudMetadata(), nil
}

// RecalculateAllWordFrequencies performs a full recalculation of word frequencies
func (b *BunDB) RecalculateAllWordFrequencies() error {
	ctx := context.Background()
	Logger.Info("Starting full word cloud recalculation")

	// Clear existing frequencies
	_, err := b.db.NewTruncateTable().Model((*BunWordFrequency)(nil)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear word frequencies: %w", err)
	}

	// Get all documents
	docs, err := b.GetAllDocuments()
	if err != nil {
		return fmt.Errorf("failed to get documents: %w", err)
	}

	Logger.Info("Processing documents for word cloud", "count", len(docs))

	tokenizer := NewWordTokenizer()
	globalFrequencies := make(map[string]int)

	// Process all documents
	for _, doc := range docs {
		combinedText := doc.FullText + " " + doc.Name
		frequencies := tokenizer.TokenizeAndCount(combinedText)

		// Aggregate frequencies
		for word, count := range frequencies {
			globalFrequencies[word] += count
		}
	}

	Logger.Info("Inserting word frequencies", "unique_words", len(globalFrequencies))

	// Batch insert frequencies
	bunWords := make([]BunWordFrequency, 0, len(globalFrequencies))
	for word, count := range globalFrequencies {
		bunWords = append(bunWords, BunWordFrequency{
			Word:        word,
			Frequency:   count,
			LastUpdated: time.Now(),
		})
	}

	if len(bunWords) > 0 {
		_, err = b.db.NewInsert().
			Model(&bunWords).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to insert word frequencies: %w", err)
		}
	}

	// Update metadata
	now := time.Now()
	_, err = b.db.NewUpdate().
		Model(&BunWordCloudMetadata{
			ID:                  1,
			LastFullCalculation: &now,
			TotalDocsProcessed:  len(docs),
			TotalWordsIndexed:   len(globalFrequencies),
			UpdatedAt:           now,
		}).
		Column("last_full_calculation", "total_documents_processed", "total_words_indexed", "updated_at").
		Set("version = version + 1").
		WherePK().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	Logger.Info("Word cloud recalculation completed", "docs", len(docs), "words", len(globalFrequencies))
	return nil
}

// UpdateWordFrequencies updates word frequencies after document ingestion
func (b *BunDB) UpdateWordFrequencies(docID string) error {
	ctx := context.Background()

	// Get the document
	doc, err := b.GetDocumentByULID(docID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Tokenize the document's full text and name
	tokenizer := NewWordTokenizer()
	combinedText := doc.FullText + " " + doc.Name
	frequencies := tokenizer.TokenizeAndCount(combinedText)

	// Update word frequencies in database
	for word, count := range frequencies {
		// Use INSERT ... ON CONFLICT for upsert
		if b.dbType == "postgres" || b.dbType == "cockroachdb" {
			_, err := b.db.NewRaw(`
				INSERT INTO word_frequencies (word, frequency, last_updated)
				VALUES (?, ?, CURRENT_TIMESTAMP)
				ON CONFLICT (word) DO UPDATE SET
					frequency = word_frequencies.frequency + EXCLUDED.frequency,
					last_updated = CURRENT_TIMESTAMP
			`, word, count).Exec(ctx)

			if err != nil {
				return fmt.Errorf("failed to update word frequency: %w", err)
			}
		} else {
			// SQLite uses different syntax
			_, err := b.db.NewRaw(`
				INSERT INTO word_frequencies (word, frequency, last_updated)
				VALUES (?, ?, CURRENT_TIMESTAMP)
				ON CONFLICT (word) DO UPDATE SET
					frequency = frequency + excluded.frequency,
					last_updated = CURRENT_TIMESTAMP
			`, word, count).Exec(ctx)

			if err != nil {
				return fmt.Errorf("failed to update word frequency: %w", err)
			}
		}
	}

	return nil
}

// GetSchemaVersion returns the latest applied migration version
func (b *BunDB) GetSchemaVersion() (string, error) {
	ctx := context.Background()

	type SchemaVersion struct {
		Version string `bun:"version"`
	}

	var version SchemaVersion
	err := b.db.NewSelect().
		TableExpr("bun_schema_migrations").
		Column("version").
		Order("version DESC").
		Limit(1).
		Scan(ctx, &version)

	if err != nil {
		if err == sql.ErrNoRows {
			return "none", nil
		}
		return "", fmt.Errorf("failed to get schema version: %w", err)
	}

	return version.Version, nil
}
