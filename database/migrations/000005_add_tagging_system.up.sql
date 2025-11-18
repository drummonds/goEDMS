-- Add tagging system with dimensions
-- Migration 000005: Tagging System

-- ============================================================================
-- TAGS - Free-form tagging
-- ============================================================================

-- Tags table - stores all available tags
CREATE TABLE IF NOT EXISTS tags (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    color TEXT DEFAULT '#3498db', -- Hex color for UI display
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Document-Tag relationship (many-to-many)
CREATE TABLE IF NOT EXISTS document_tags (
    document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (document_id, tag_id)
);

-- Indexes for fast tag lookups
CREATE INDEX IF NOT EXISTS idx_document_tags_document ON document_tags(document_id);
CREATE INDEX IF NOT EXISTS idx_document_tags_tag ON document_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);

-- ============================================================================
-- DIMENSIONS - Structured metadata with constraints
-- ============================================================================

-- Dimensions table - defines available dimension types
CREATE TABLE IF NOT EXISTS dimensions (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE, -- e.g., "person", "location", "year", "importance", "retention"
    display_name TEXT NOT NULL, -- e.g., "Person", "Location"
    description TEXT,
    dimension_type TEXT NOT NULL DEFAULT 'single', -- 'single' or 'multiple'
    is_required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Dimension values table - stores allowed values for each dimension
CREATE TABLE IF NOT EXISTS dimension_values (
    id SERIAL PRIMARY KEY,
    dimension_id INTEGER NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    color TEXT DEFAULT '#95a5a6', -- Hex color for UI display
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(dimension_id, value)
);

-- Document dimensions - stores dimension values for each document
CREATE TABLE IF NOT EXISTS document_dimensions (
    id SERIAL PRIMARY KEY,
    document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    dimension_id INTEGER NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
    dimension_value_id INTEGER NOT NULL REFERENCES dimension_values(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(document_id, dimension_id) -- One value per dimension per document
);

-- Indexes for fast dimension lookups
CREATE INDEX IF NOT EXISTS idx_document_dimensions_document ON document_dimensions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_dimensions_dimension ON document_dimensions(dimension_id);
CREATE INDEX IF NOT EXISTS idx_document_dimensions_value ON document_dimensions(dimension_value_id);
CREATE INDEX IF NOT EXISTS idx_dimension_values_dimension ON dimension_values(dimension_id);

-- ============================================================================
-- TRIGGERS - Auto-update timestamps
-- ============================================================================

DROP TRIGGER IF EXISTS update_tags_timestamp ON tags;
CREATE TRIGGER update_tags_timestamp
    BEFORE UPDATE ON tags
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_dimensions_timestamp ON dimensions;
CREATE TRIGGER update_dimensions_timestamp
    BEFORE UPDATE ON dimensions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_dimension_values_timestamp ON dimension_values;
CREATE TRIGGER update_dimension_values_timestamp
    BEFORE UPDATE ON dimension_values
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_document_dimensions_timestamp ON document_dimensions;
CREATE TRIGGER update_document_dimensions_timestamp
    BEFORE UPDATE ON document_dimensions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- SEED DATA - Predefined dimensions and values
-- ============================================================================

-- Insert predefined dimensions
INSERT INTO dimensions (name, display_name, description, dimension_type, is_required) VALUES
    ('person', 'Person', 'Document owner or related person', 'single', false),
    ('location', 'Location', 'Physical or organizational location', 'single', false),
    ('year', 'Year', 'Document year for archival purposes', 'single', false),
    ('importance', 'Importance', 'Document importance level', 'single', false),
    ('retention', 'Retention', 'How long to keep this document', 'single', false)
ON CONFLICT (name) DO NOTHING;

-- Insert Person dimension values
INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
SELECT d.id, v.value, v.display_name, v.description, v.color, v.sort_order
FROM dimensions d
CROSS JOIN (VALUES
    ('husband', 'Husband', 'Documents belonging to husband', '#3498db', 1),
    ('wife', 'Wife', 'Documents belonging to wife', '#e91e63', 2),
    ('child1', 'Child 1', 'Documents for first child', '#9c27b0', 3),
    ('child2', 'Child 2', 'Documents for second child', '#673ab7', 4),
    ('child3', 'Child 3', 'Documents for third child', '#2196f3', 5),
    ('family', 'Family', 'Family documents (all members)', '#4caf50', 6),
    ('business', 'Business', 'Business-related documents', '#ff9800', 7),
    ('other', 'Other', 'Other person-related documents', '#9e9e9e', 99)
) AS v(value, display_name, description, color, sort_order)
WHERE d.name = 'person'
ON CONFLICT (dimension_id, value) DO NOTHING;

-- Insert Location dimension values
INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
SELECT d.id, v.value, v.display_name, v.description, v.color, v.sort_order
FROM dimensions d
CROSS JOIN (VALUES
    ('home', 'Home', 'Home-related documents', '#4caf50', 1),
    ('office', 'Office', 'Office/workplace documents', '#2196f3', 2),
    ('bank', 'Bank', 'Banking and financial documents', '#ff9800', 3),
    ('medical', 'Medical', 'Medical and health documents', '#f44336', 4),
    ('legal', 'Legal', 'Legal documents', '#9c27b0', 5),
    ('insurance', 'Insurance', 'Insurance-related documents', '#00bcd4', 6),
    ('tax', 'Tax', 'Tax-related documents', '#795548', 7),
    ('education', 'Education', 'Education-related documents', '#607d8b', 8),
    ('other', 'Other', 'Other locations', '#9e9e9e', 99)
) AS v(value, display_name, description, color, sort_order)
WHERE d.name = 'location'
ON CONFLICT (dimension_id, value) DO NOTHING;

-- Insert Importance dimension values
INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
SELECT d.id, v.value, v.display_name, v.description, v.color, v.sort_order
FROM dimensions d
CROSS JOIN (VALUES
    ('low', 'Low', 'Low importance - can be discarded easily', '#9e9e9e', 1),
    ('medium', 'Medium', 'Medium importance - keep for reference', '#3498db', 2),
    ('high', 'High', 'High importance - important to keep', '#ff9800', 3),
    ('critical', 'Critical', 'Critical - must keep and protect', '#f44336', 4)
) AS v(value, display_name, description, color, sort_order)
WHERE d.name = 'importance'
ON CONFLICT (dimension_id, value) DO NOTHING;

-- Insert Retention dimension values
INSERT INTO dimension_values (dimension_id, value, display_name, description, color, sort_order)
SELECT d.id, v.value, v.display_name, v.description, v.color, v.sort_order
FROM dimensions d
CROSS JOIN (VALUES
    ('temporary', 'Temporary', 'Delete after processing (< 1 year)', '#9e9e9e', 1),
    ('keep_1_year', '1 Year', 'Keep for 1 year', '#03a9f4', 2),
    ('keep_3_years', '3 Years', 'Keep for 3 years', '#2196f3', 3),
    ('keep_7_years', '7 Years', 'Keep for 7 years (tax records)', '#ff9800', 4),
    ('keep_10_years', '10 Years', 'Keep for 10 years', '#ff5722', 5),
    ('keep_permanent', 'Permanent', 'Keep permanently (birth certificates, etc.)', '#4caf50', 6)
) AS v(value, display_name, description, color, sort_order)
WHERE d.name = 'retention'
ON CONFLICT (dimension_id, value) DO NOTHING;
