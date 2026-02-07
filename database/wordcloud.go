package database

import (
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
	LastCalculation    time.Time `json:"lastCalculation"`
	TotalDocsProcessed int       `json:"totalDocsProcessed"`
	TotalWordsIndexed  int       `json:"totalWordsIndexed"`
	Version            int       `json:"version"`
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
