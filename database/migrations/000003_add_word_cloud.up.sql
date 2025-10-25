-- Add word cloud functionality
-- This table stores pre-calculated word frequencies for fast word cloud generation

-- Create word_frequencies table
CREATE TABLE IF NOT EXISTS word_frequencies (
    word TEXT PRIMARY KEY,
    frequency INTEGER NOT NULL DEFAULT 1,
    last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index for sorting by frequency
CREATE INDEX IF NOT EXISTS idx_word_frequencies_frequency ON word_frequencies(frequency DESC);

-- Create index for last_updated
CREATE INDEX IF NOT EXISTS idx_word_frequencies_updated ON word_frequencies(last_updated DESC);

-- Create metadata table to track when word cloud was last calculated
CREATE TABLE IF NOT EXISTS word_cloud_metadata (
    id INTEGER PRIMARY KEY CHECK (id = 1), -- Only allow one row
    last_full_calculation TIMESTAMP,
    total_documents_processed INTEGER DEFAULT 0,
    total_words_indexed INTEGER DEFAULT 0,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default metadata row
INSERT INTO word_cloud_metadata (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

-- Function to clean and normalize words for word cloud
-- This removes common stop words and normalizes the text
CREATE OR REPLACE FUNCTION clean_word_for_cloud(word TEXT)
RETURNS TEXT AS $$
BEGIN
    -- Convert to lowercase and trim
    word := lower(trim(word));

    -- Remove if it's a stop word (common English words)
    -- Simplified list - you can expand this
    IF word IN ('the', 'a', 'an', 'and', 'or', 'but', 'in', 'on', 'at', 'to',
                'for', 'of', 'as', 'by', 'is', 'was', 'are', 'were', 'be',
                'this', 'that', 'with', 'from', 'they', 'we', 'you', 'it',
                'have', 'has', 'had', 'will', 'would', 'could', 'should',
                'can', 'may', 'must', 'shall', 'their', 'there', 'here') THEN
        RETURN NULL;
    END IF;

    -- Remove if too short (less than 3 characters)
    IF length(word) < 3 THEN
        RETURN NULL;
    END IF;

    -- Remove if it's a number
    IF word ~ '^\d+$' THEN
        RETURN NULL;
    END IF;

    RETURN word;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
