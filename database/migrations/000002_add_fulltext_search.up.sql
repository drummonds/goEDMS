-- Add PostgreSQL full-text search support
-- This replaces the Bleve search index with native PostgreSQL capabilities

-- Add a tsvector column for full-text search
ALTER TABLE documents ADD COLUMN IF NOT EXISTS full_text_search tsvector;

-- Create a GIN index for fast full-text searching
CREATE INDEX IF NOT EXISTS idx_documents_full_text_search ON documents USING GIN(full_text_search);

-- Create a function to automatically update the search vector when full_text changes
CREATE OR REPLACE FUNCTION update_full_text_search()
RETURNS TRIGGER AS $$
BEGIN
    NEW.full_text_search = to_tsvector('english', COALESCE(NEW.full_text, '') || ' ' || COALESCE(NEW.name, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create a trigger to update search vector on insert/update
DROP TRIGGER IF EXISTS trigger_update_full_text_search ON documents;
CREATE TRIGGER trigger_update_full_text_search
    BEFORE INSERT OR UPDATE OF full_text, name ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_full_text_search();

-- Update existing documents to populate the search vector
UPDATE documents SET full_text_search = to_tsvector('english', COALESCE(full_text, '') || ' ' || COALESCE(name, ''));
