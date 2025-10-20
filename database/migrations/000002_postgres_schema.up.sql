-- PostgreSQL-compatible schema
-- This migration will be applied when using PostgreSQL/CockroachDB

-- Create documents table
CREATE TABLE IF NOT EXISTS documents (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    ingress_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    folder TEXT NOT NULL,
    hash TEXT NOT NULL,
    ulid TEXT NOT NULL UNIQUE,
    document_type TEXT NOT NULL,
    full_text TEXT,
    url TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(hash);
CREATE INDEX IF NOT EXISTS idx_documents_ulid ON documents(ulid);
CREATE INDEX IF NOT EXISTS idx_documents_folder ON documents(folder);
CREATE INDEX IF NOT EXISTS idx_documents_ingress_time ON documents(ingress_time DESC);

-- Create server_config table
CREATE TABLE IF NOT EXISTS server_config (
    id INTEGER PRIMARY KEY CHECK (id = 1), -- Only allow one row
    listen_addr_ip TEXT DEFAULT '',
    listen_addr_port TEXT NOT NULL DEFAULT '8000',
    ingress_path TEXT NOT NULL DEFAULT '',
    ingress_delete BOOLEAN NOT NULL DEFAULT false,
    ingress_move_folder TEXT NOT NULL DEFAULT '',
    ingress_preserve BOOLEAN NOT NULL DEFAULT true,
    document_path TEXT NOT NULL DEFAULT '',
    new_document_folder TEXT DEFAULT '',
    new_document_folder_rel TEXT DEFAULT '',
    web_ui_pass BOOLEAN NOT NULL DEFAULT false,
    client_username TEXT DEFAULT '',
    client_password TEXT DEFAULT '',
    pushbullet_token TEXT DEFAULT '',
    tesseract_path TEXT DEFAULT '',
    use_reverse_proxy BOOLEAN NOT NULL DEFAULT false,
    base_url TEXT DEFAULT '',
    ingress_interval INTEGER NOT NULL DEFAULT 10,
    new_document_number INTEGER NOT NULL DEFAULT 5,
    server_api_url TEXT DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default config row (will use default values)
INSERT INTO server_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to update updated_at timestamp
DROP TRIGGER IF EXISTS update_documents_timestamp ON documents;
CREATE TRIGGER update_documents_timestamp
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_server_config_timestamp ON server_config;
CREATE TRIGGER update_server_config_timestamp
    BEFORE UPDATE ON server_config
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
