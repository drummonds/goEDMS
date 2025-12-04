-- Migration 000006: Unify Tags and Dimensions
-- Adds tag_group to tags table and migrates dimension values to grouped tags

-- ============================================================================
-- STEP 1: Add new columns to tags table
-- ============================================================================

-- Add tag_group column (nullable - null means free tag, value means grouped)
ALTER TABLE tags ADD COLUMN IF NOT EXISTS tag_group TEXT;

-- Add sort_order for ordering within groups
ALTER TABLE tags ADD COLUMN IF NOT EXISTS sort_order INTEGER DEFAULT 0;

-- Create index for group lookups
CREATE INDEX IF NOT EXISTS idx_tags_group ON tags(tag_group);

-- ============================================================================
-- STEP 2: Migrate dimension values to grouped tags
-- ============================================================================

-- Insert dimension values as grouped tags (only if they don't already exist)
-- Use a subquery to get distinct dimension values and avoid duplicate name conflicts
INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
SELECT
    sub.name,
    sub.color,
    sub.description,
    sub.tag_group,
    sub.sort_order,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM (
    SELECT DISTINCT ON (dv.display_name)
        dv.display_name as name,
        dv.color,
        COALESCE(dv.description, '') as description,
        d.display_name as tag_group,
        dv.sort_order
    FROM dimension_values dv
    JOIN dimensions d ON dv.dimension_id = d.id
    ORDER BY dv.display_name, dv.sort_order
) sub
WHERE NOT EXISTS (SELECT 1 FROM tags WHERE name = sub.name);

-- ============================================================================
-- STEP 3: Migrate document_dimensions to document_tags
-- ============================================================================

-- Insert document dimension assignments as document tags
INSERT INTO document_tags (document_id, tag_id, created_at)
SELECT
    dd.document_id,
    t.id as tag_id,
    dd.created_at
FROM document_dimensions dd
JOIN dimension_values dv ON dd.dimension_value_id = dv.id
JOIN dimensions d ON dd.dimension_id = d.id
JOIN tags t ON t.name = dv.display_name AND t.tag_group = d.display_name
ON CONFLICT (document_id, tag_id) DO NOTHING;

-- ============================================================================
-- STEP 4: Create constraint function for one-per-group
-- ============================================================================

-- Function to enforce one tag per group per document
CREATE OR REPLACE FUNCTION enforce_one_tag_per_group()
RETURNS TRIGGER AS $$
DECLARE
    v_tag_group TEXT;
BEGIN
    -- Get the group of the tag being inserted
    SELECT tag_group INTO v_tag_group FROM tags WHERE id = NEW.tag_id;

    -- If tag has a group, remove other tags from the same group for this document
    IF v_tag_group IS NOT NULL THEN
        DELETE FROM document_tags
        WHERE document_id = NEW.document_id
        AND tag_id IN (SELECT id FROM tags WHERE tag_group = v_tag_group)
        AND tag_id != NEW.tag_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to enforce one-per-group
DROP TRIGGER IF EXISTS enforce_one_tag_per_group_trigger ON document_tags;
CREATE TRIGGER enforce_one_tag_per_group_trigger
    AFTER INSERT ON document_tags
    FOR EACH ROW
    EXECUTE FUNCTION enforce_one_tag_per_group();

-- ============================================================================
-- NOTE: Dimension tables are kept for backwards compatibility
-- They can be removed in a future migration after confirming all data migrated
-- ============================================================================
