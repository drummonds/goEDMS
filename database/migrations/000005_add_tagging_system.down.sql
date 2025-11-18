-- Rollback tagging system
-- Migration 000005 DOWN: Remove tagging system

-- Drop triggers
DROP TRIGGER IF EXISTS update_document_dimensions_timestamp ON document_dimensions;
DROP TRIGGER IF EXISTS update_dimension_values_timestamp ON dimension_values;
DROP TRIGGER IF EXISTS update_dimensions_timestamp ON dimensions;
DROP TRIGGER IF EXISTS update_tags_timestamp ON tags;

-- Drop tables (order matters due to foreign keys)
DROP TABLE IF EXISTS document_dimensions;
DROP TABLE IF EXISTS dimension_values;
DROP TABLE IF EXISTS dimensions;
DROP TABLE IF EXISTS document_tags;
DROP TABLE IF EXISTS tags;
