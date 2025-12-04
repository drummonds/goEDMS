-- Rollback Migration 000006: Unify Tags and Dimensions

-- Drop the trigger
DROP TRIGGER IF EXISTS enforce_one_tag_per_group_trigger ON document_tags;

-- Drop the function
DROP FUNCTION IF EXISTS enforce_one_tag_per_group();

-- Remove document_tags entries that came from dimensions
DELETE FROM document_tags
WHERE tag_id IN (SELECT id FROM tags WHERE tag_group IS NOT NULL);

-- Remove tags that were created from dimensions
DELETE FROM tags WHERE tag_group IS NOT NULL;

-- Drop the index
DROP INDEX IF EXISTS idx_tags_group;

-- Remove the columns
ALTER TABLE tags DROP COLUMN IF EXISTS tag_group;
ALTER TABLE tags DROP COLUMN IF EXISTS sort_order;
