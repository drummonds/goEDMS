# Database Configuration Guide

goEDMS now supports multiple database backends to provide flexibility for different deployment scenarios.

## Supported Databases

- **SQLite** (default) - Simple, embedded database, great for single-user deployments
- **PostgreSQL** - Production-grade relational database with embedded option
- **CockroachDB** - Distributed SQL database for high-availability deployments

## Configuration

Database settings are configured in `config/serverConfig.toml`:

```toml
[database]
    # Database type: "sqlite", "postgres", or "cockroachdb"
    Type = "sqlite"
    
    # PostgreSQL/CockroachDB connection string (leave empty to use embedded PostgreSQL)
    ConnectionString = ""
```

## SQLite (Default)

**Best for:** Single-user installations, development, testing

**Configuration:**
```toml
[database]
    Type = "sqlite"
    ConnectionString = ""
```

**Features:**
- No external dependencies
- Automatic setup
- File-based database stored in `databases/goEDMS.db`
- Zero configuration required

## PostgreSQL

**Best for:** Multi-user deployments, production environments, need for advanced SQL features

### Option 1: Embedded PostgreSQL (Recommended for Easy Setup)

**Configuration:**
```toml
[database]
    Type = "postgres"
    ConnectionString = ""  # Leave empty for embedded mode
```

**Features:**
- **Persistent data storage on disk** - Your data is saved between restarts
- Automatically downloads and runs PostgreSQL 17
- No manual installation required
- Runs on port 5433 (to avoid conflicts with system PostgreSQL)
- Data stored persistently in `databases/postgres_data/`
- Binaries cached in `databases/postgres_cache/` (reused across restarts)
- Supports pre-downloaded binaries for air-gapped/offline environments

**Data Persistence:**
The embedded PostgreSQL uses persistent storage. This means:
- ✅ Data survives application restarts
- ✅ Data survives system reboots
- ✅ You can stop and start goEDMS without losing data
- ✅ Works exactly like a traditional PostgreSQL installation

**Directory Structure:**
```
databases/
├── postgres_data/       # Your actual database data (persistent)
├── postgres_cache/      # Downloaded PostgreSQL binaries (cached)
└── postgres_binaries/   # Optional: Pre-downloaded binaries for offline use
```

**Requirements:**
- Sufficient disk space (~100MB for PostgreSQL binaries + your data)
- Available port 5433

**Using Pre-Downloaded Binaries (Dev/Offline Mode):**

For development or air-gapped systems, you can pre-download PostgreSQL binaries:

1. Download the appropriate binary for your system from the embedded-postgres releases
2. Extract to `databases/postgres_binaries/`
3. goEDMS will automatically detect and use these binaries instead of downloading

This is useful for:
- Offline/air-gapped environments
- Faster startup during development
- Consistent binary versions across deployments
- CI/CD pipelines with cached dependencies

### Option 2: External PostgreSQL Server

**Configuration:**
```toml
[database]
    Type = "postgres"
    ConnectionString = "host=localhost port=5432 user=goedms password=yourpassword dbname=goedms sslmode=disable"
```

**Setup Steps:**

1. Install PostgreSQL on your system or server
2. Create a database and user:
   ```sql
   CREATE DATABASE goedms;
   CREATE USER goedms WITH PASSWORD 'yourpassword';
   GRANT ALL PRIVILEGES ON DATABASE goedms TO goedms;
   ```
3. Update the connection string in `serverConfig.toml`
4. Start goEDMS - migrations will run automatically

**Connection String Format:**
```
host=<hostname> port=<port> user=<username> password=<password> dbname=<database> sslmode=<disable|require>
```

**Parameters:**
- `host` - Database server hostname (e.g., localhost, 192.168.1.100)
- `port` - PostgreSQL port (default: 5432)
- `user` - Database username
- `password` - Database password
- `dbname` - Database name
- `sslmode` - SSL mode (disable, require, verify-ca, verify-full)

## CockroachDB

**Best for:** Distributed deployments, high availability, horizontal scalability

**Configuration:**
```toml
[database]
    Type = "cockroachdb"
    ConnectionString = "host=localhost port=26257 user=root dbname=goedms sslmode=require"
```

**Setup Steps:**

1. Install and start CockroachDB cluster
2. Create database:
   ```sql
   CREATE DATABASE goedms;
   ```
3. Update connection string in `serverConfig.toml`
4. Start goEDMS - migrations will run automatically

**Notes:**
- CockroachDB uses PostgreSQL wire protocol
- Default port is 26257 (not 5432)
- SSL is typically required for CockroachDB connections
- Supports geo-distributed deployments

## Migration Between Databases

⚠️ **Important:** Databases are not automatically migrated between types. If you switch database types, you'll start with a fresh database.

To migrate data between databases:

1. Export documents from the old database
2. Change database configuration
3. Restart goEDMS with new database
4. Re-ingest documents or manually migrate data

## Database Files and Locations

### SQLite
- Database: `databases/goEDMS.db`
- WAL file: `databases/goEDMS.db-wal`
- Shared memory: `databases/goEDMS.db-shm`

### Embedded PostgreSQL

**All data is persistent and stored on disk:**

- Data directory: `databases/postgres_data/` (persistent database files)
  - Base files: Table data, indexes, system catalogs
  - WAL files: Write-ahead logs for crash recovery
  - Logs: `postgresql.log` for debugging
- Binary cache: `databases/postgres_cache/` (downloaded PostgreSQL binaries)
- Optional binaries: `databases/postgres_binaries/` (pre-downloaded for offline use)

**Important Notes:**
- Data persists across restarts (just like external PostgreSQL)
- First startup downloads binaries (~100MB) - subsequent starts use cached binaries
- Database initialization only happens once - reuses existing data on restart
- You can safely stop/start goEDMS without data loss
- Backup `postgres_data/` directory to preserve your data

### External PostgreSQL/CockroachDB
- Managed by external database server
- No local files created by goEDMS

## Troubleshooting

### Embedded PostgreSQL Won't Start

**Error:** `failed to start embedded postgres: timed out waiting for database to become available`

**Solutions:**
1. Check port 5433 is not in use: `lsof -i :5433`
2. Ensure sufficient disk space
3. Check permissions on `databases/` directory
4. Delete `databases/postgres_data` and `databases/postgres_runtime` to force clean reinstall
5. Switch to external PostgreSQL or SQLite

### Connection Refused

**Error:** `connection refused` or `could not connect to server`

**Solutions:**
1. Verify PostgreSQL/CockroachDB is running
2. Check hostname and port in connection string
3. Verify firewall allows connections
4. Check database credentials
5. For SSL issues, try `sslmode=disable` for testing

### Migration Errors

**Error:** `failed to run migrations` or `SQL logic error`

**Solutions:**
1. Check database type matches connection string
2. Ensure database user has sufficient privileges
3. Delete database and start fresh
4. Check logs in `goedms.log` for detailed error

### Performance Issues

For SQLite:
- Consider switching to PostgreSQL for multi-user scenarios
- SQLite is single-writer, PostgreSQL supports concurrent writes

For PostgreSQL:
- Check `databases/postgres_data/postgresql.log` for slow queries
- Consider adding indexes for frequently searched fields
- Tune PostgreSQL configuration for your workload

## Schema Information

goEDMS uses migrations to manage database schema:

- **Migration 000001**: SQLite schema (auto-increment IDs, SQLite-specific syntax)
- **Migration 000002**: PostgreSQL schema (SERIAL IDs, triggers, functions)

Migrations are automatically applied on startup based on the configured database type.

## Performance Considerations

| Feature | SQLite | PostgreSQL | CockroachDB |
|---------|--------|-----------|-------------|
| Single user | ⭐⭐⭐ | ⭐⭐ | ⭐ |
| Multiple users | ⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| Large datasets | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| Setup complexity | ⭐⭐⭐ | ⭐⭐ | ⭐ |
| Resource usage | ⭐⭐⭐ | ⭐⭐ | ⭐ |
| High availability | ❌ | Limited | ⭐⭐⭐ |
| Geo-distribution | ❌ | ❌ | ⭐⭐⭐ |

## Backup and Recovery

### SQLite Backup
```bash
# Stop goEDMS first
cp databases/goEDMS.db databases/goEDMS.db.backup
```

### PostgreSQL Backup (Embedded)

**Option 1: File-based backup (recommended for embedded PostgreSQL)**
```bash
# Stop goEDMS first to ensure data consistency
./goEDMS stop  # or kill the process

# Backup the entire data directory
tar -czf goedms_postgres_backup_$(date +%Y%m%d).tar.gz databases/postgres_data/

# To restore:
# 1. Stop goEDMS
# 2. Remove old data: rm -rf databases/postgres_data
# 3. Extract backup: tar -xzf goedms_postgres_backup_YYYYMMDD.tar.gz
```

**Option 2: PostgreSQL dump (works while goEDMS is running)**
```bash
# Create SQL dump
pg_dump -h localhost -p 5433 -U goedms goedms > goedms_backup.sql

# To restore:
psql -h localhost -p 5433 -U goedms goedms < goedms_backup.sql
```

### PostgreSQL Backup (External)
```bash
pg_dump -h localhost -U goedms goedms > goedms_backup.sql
```

### CockroachDB Backup
```sql
BACKUP DATABASE goedms TO 'nodelocal://1/goedms_backup';
```

## Recommendations

- **Development/Testing:** SQLite (default) - zero configuration
- **Single-user Production:** Embedded PostgreSQL - better performance, no setup
- **Multi-user Production:** External PostgreSQL - full control, tuning, backups
- **Enterprise/Distributed:** CockroachDB - high availability, geo-replication
