package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// WordFrequency represents a word and its frequency count
type WordFrequency struct {
	Word      string    `json:"word"`
	Frequency int       `json:"frequency"`
	Updated   time.Time `json:"updated"`
}

// WordCloudMetadata tracks word cloud calculation status
type WordCloudMetadata struct {
	LastCalculation      time.Time `json:"lastCalculation"`
	TotalDocsProcessed   int       `json:"totalDocsProcessed"`
	TotalWordsIndexed    int       `json:"totalWordsIndexed"`
	Version              int       `json:"version"`
}

// Stop words to filter out (common English words that don't add value)
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "as": true, "by": true, "is": true,
	"was": true, "are": true, "were": true, "be": true, "this": true,
	"that": true, "with": true, "from": true, "they": true, "we": true,
	"you": true, "it": true, "have": true, "has": true, "had": true,
	"will": true, "would": true, "could": true, "should": true, "can": true,
	"may": true, "must": true, "shall": true, "their": true, "there": true,
	"here": true, "what": true, "where": true, "when": true, "who": true,
	"which": true, "how": true, "all": true, "each": true, "every": true,
	"both": true, "few": true, "more": true, "most": true, "other": true,
	"some": true, "such": true, "than": true, "too": true, "very": true,
}

// WordTokenizer handles text processing for word cloud
type WordTokenizer struct {
	wordRegex *regexp.Regexp
}

// NewWordTokenizer creates a new word tokenizer
func NewWordTokenizer() *WordTokenizer {
	return &WordTokenizer{
		// Match words with letters and optional hyphens/apostrophes
		wordRegex: regexp.MustCompile(`\b[a-zA-Z][a-zA-Z'-]*[a-zA-Z]\b|\b[a-zA-Z]+\b`),
	}
}

// TokenizeAndCount extracts words from text and counts frequencies
func (wt *WordTokenizer) TokenizeAndCount(text string) map[string]int {
	frequencies := make(map[string]int)

	// Convert to lowercase
	text = strings.ToLower(text)

	// Find all words
	words := wt.wordRegex.FindAllString(text, -1)

	for _, word := range words {
		// Skip if too short
		if len(word) < 3 {
			continue
		}

		// Skip if it's a stop word
		if stopWords[word] {
			continue
		}

		// Skip if it's purely numeric
		if regexp.MustCompile(`^\d+$`).MatchString(word) {
			continue
		}

		frequencies[word]++
	}

	return frequencies
}

// UpdateWordFrequencies updates word frequencies after document ingestion
// This should be called incrementally as documents are added
func (p *PostgresDB) UpdateWordFrequencies(docID string) error {
	// Get the document
	doc, err := p.GetDocumentByULID(docID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Tokenize the document's full text and name
	tokenizer := NewWordTokenizer()
	combinedText := doc.FullText + " " + doc.Name
	frequencies := tokenizer.TokenizeAndCount(combinedText)

	// Update word frequencies in database
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for word, count := range frequencies {
		query := `
			INSERT INTO word_frequencies (word, frequency, last_updated)
			VALUES ($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (word) DO UPDATE SET
				frequency = word_frequencies.frequency + EXCLUDED.frequency,
				last_updated = CURRENT_TIMESTAMP
		`
		_, err := tx.Exec(query, word, count)
		if err != nil {
			return fmt.Errorf("failed to update word frequency: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RecalculateAllWordFrequencies performs a full recalculation of word frequencies
// This should be called during database cleaning or on-demand
func (p *PostgresDB) RecalculateAllWordFrequencies() error {
	Logger.Info("Starting full word cloud recalculation")

	// Clear existing frequencies
	_, err := p.db.Exec("TRUNCATE TABLE word_frequencies")
	if err != nil {
		return fmt.Errorf("failed to clear word frequencies: %w", err)
	}

	// Get all documents
	docs, err := p.GetAllDocuments()
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
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Use prepared statement for efficiency
	stmt, err := tx.Prepare(`
		INSERT INTO word_frequencies (word, frequency, last_updated)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for word, count := range globalFrequencies {
		_, err := stmt.Exec(word, count)
		if err != nil {
			return fmt.Errorf("failed to insert word frequency: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update metadata
	updateMetadata := `
		UPDATE word_cloud_metadata SET
			last_full_calculation = CURRENT_TIMESTAMP,
			total_documents_processed = $1,
			total_words_indexed = $2,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`
	_, err = p.db.Exec(updateMetadata, len(docs), len(globalFrequencies))
	if err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	Logger.Info("Word cloud recalculation completed", "docs", len(docs), "words", len(globalFrequencies))
	return nil
}

// GetTopWords retrieves the top N most frequent words
func (p *PostgresDB) GetTopWords(limit int) ([]WordFrequency, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT word, frequency, last_updated
		FROM word_frequencies
		ORDER BY frequency DESC, word ASC
		LIMIT $1
	`

	rows, err := p.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top words: %w", err)
	}
	defer rows.Close()

	// Initialize as empty slice so JSON marshals to [] instead of null
	words := make([]WordFrequency, 0)
	for rows.Next() {
		var wf WordFrequency
		err := rows.Scan(&wf.Word, &wf.Frequency, &wf.Updated)
		if err != nil {
			return nil, fmt.Errorf("failed to scan word frequency: %w", err)
		}
		words = append(words, wf)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return words, nil
}

// GetWordCloudMetadata retrieves metadata about the word cloud
func (p *PostgresDB) GetWordCloudMetadata() (*WordCloudMetadata, error) {
	query := `
		SELECT last_full_calculation, total_documents_processed,
		       total_words_indexed, version
		FROM word_cloud_metadata
		WHERE id = 1
	`

	var meta WordCloudMetadata
	var lastCalc sql.NullTime

	err := p.db.QueryRow(query).Scan(
		&lastCalc,
		&meta.TotalDocsProcessed,
		&meta.TotalWordsIndexed,
		&meta.Version,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	if lastCalc.Valid {
		meta.LastCalculation = lastCalc.Time
	}

	return &meta, nil
}
