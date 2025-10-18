-- Drop triggers
DROP TRIGGER IF EXISTS update_server_config_timestamp;
DROP TRIGGER IF EXISTS update_documents_timestamp;

-- Drop indexes
DROP INDEX IF EXISTS idx_documents_ingress_time;
DROP INDEX IF EXISTS idx_documents_folder;
DROP INDEX IF EXISTS idx_documents_ulid;
DROP INDEX IF EXISTS idx_documents_hash;

-- Drop tables
DROP TABLE IF EXISTS server_config;
DROP TABLE IF EXISTS documents;
