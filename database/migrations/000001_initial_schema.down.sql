-- Drop triggers
DROP TRIGGER IF EXISTS update_documents_timestamp ON documents;
DROP TRIGGER IF EXISTS update_server_config_timestamp ON server_config;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS server_config;
