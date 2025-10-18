-- Create documents table
CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    ingress_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    folder TEXT NOT NULL,
    hash TEXT NOT NULL,
    ulid TEXT NOT NULL UNIQUE,
    document_type TEXT NOT NULL,
    full_text TEXT,
    url TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
    ingress_delete BOOLEAN NOT NULL DEFAULT 0,
    ingress_move_folder TEXT NOT NULL DEFAULT '',
    ingress_preserve BOOLEAN NOT NULL DEFAULT 1,
    document_path TEXT NOT NULL DEFAULT '',
    new_document_folder TEXT DEFAULT '',
    new_document_folder_rel TEXT DEFAULT '',
    web_ui_pass BOOLEAN NOT NULL DEFAULT 0,
    client_username TEXT DEFAULT '',
    client_password TEXT DEFAULT '',
    pushbullet_token TEXT DEFAULT '',
    tesseract_path TEXT DEFAULT '',
    use_reverse_proxy BOOLEAN NOT NULL DEFAULT 0,
    base_url TEXT DEFAULT '',
    ingress_interval INTEGER NOT NULL DEFAULT 10,
    new_document_number INTEGER NOT NULL DEFAULT 5,
    server_api_url TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default config row (will use default values)
INSERT OR IGNORE INTO server_config (id) VALUES (1);

-- Create trigger to update updated_at timestamp
CREATE TRIGGER IF NOT EXISTS update_documents_timestamp
AFTER UPDATE ON documents
BEGIN
    UPDATE documents SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_server_config_timestamp
AFTER UPDATE ON server_config
BEGIN
    UPDATE server_config SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
