-- Rollback full-text search additions

-- Drop the trigger
DROP TRIGGER IF EXISTS trigger_update_full_text_search ON documents;

-- Drop the function
DROP FUNCTION IF EXISTS update_full_text_search();

-- Drop the index
DROP INDEX IF EXISTS idx_documents_full_text_search;

-- Drop the column
ALTER TABLE documents DROP COLUMN IF EXISTS full_text_search;
