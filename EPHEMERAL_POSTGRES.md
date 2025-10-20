# Ephemeral PostgreSQL Setup

This document describes the ephemeral PostgreSQL implementation in goEDMS, based on the approach from [Michael Stapelberg's blog post](https://michael.stapelberg.ch/posts/2024-11-19-testing-with-go-and-postgresql-ephemeral-dbs/).

## Overview

goEDMS now supports ephemeral PostgreSQL databases for development and testing. This approach replaces the previous `embedded-postgres` library with `postgrestest`, providing:

- **Faster startup times**: ~400ms vs several seconds
- **Better test isolation**: Each test gets a clean database
- **No persistent data**: Databases are automatically cleaned up
- **Simplified development**: No need to manage PostgreSQL installations

## How It Works

When you run goEDMS with PostgreSQL configured but no connection string provided, it automatically:

1. Starts a temporary PostgreSQL instance using `postgrestest`
2. Creates a new database for the application
3. Runs migrations to set up the schema
4. Cleans up everything when the application exits

## Configuration

### Using Ephemeral PostgreSQL (Development)

In `config/serverConfig.toml`:

```toml
[database]
    Type = "postgres"
    ConnectionString = ""  # Empty = use ephemeral PostgreSQL
```

### Using External PostgreSQL (Production)

```toml
[database]
    Type = "postgres"
    ConnectionString = "host=localhost port=5432 user=goedms password=goedms dbname=goedms sslmode=disable"
```

### Using SQLite (Simple Deployments)

```toml
[database]
    Type = "sqlite"
    ConnectionString = ""
```

## Running the Application

```bash
# With ephemeral PostgreSQL (configured as above)
go run .

# The application will:
# 1. Start an ephemeral PostgreSQL server
# 2. Create and migrate the database
# 3. Start the web server on port 8000
# 4. Clean up PostgreSQL when you stop the application
```

## Testing

### Unit Tests

The database package includes tests for ephemeral PostgreSQL:

```bash
cd database
go test -v -run TestEphemeralPostgres
```

### Integration Tests

Tests can use the `SetupEphemeralPostgresDatabase()` function directly:

```go
func TestWithDatabase(t *testing.T) {
    ephemeralDB, err := database.SetupEphemeralPostgresDatabase()
    if err != nil {
        t.Fatal(err)
    }
    defer ephemeralDB.Close()

    // Use ephemeralDB.PostgresDB for database operations
}
```

### Shared Test Database

For faster test suites, you can share a single PostgreSQL instance across tests:

```bash
# Set PGURL environment variable to share a PostgreSQL instance
export PGURL="postgres://postgres:@/postgres?host=/tmp/postgrestest123&sslmode=disable"
go test ./...
```

## Implementation Details

### Key Files

- `database/ephemeral_postgres.go`: Ephemeral PostgreSQL setup
- `database/postgres_database.go`: PostgreSQL database interface
- `database/migrations/`: SQL migration files

### Migration Strategy

The system uses numbered migrations:
- `000001_*.sql`: SQLite schema (skipped for PostgreSQL)
- `000002_*.sql`: PostgreSQL schema

When using PostgreSQL, the system automatically skips the SQLite migration and applies only the PostgreSQL-specific schema.

### Dependencies

```go
require (
    github.com/stapelberg/postgrestest v0.0.0-20250114201530-c4d5c90e782b
    github.com/lib/pq v1.10.10-0.20241116184759-b7ffbd3b47da
    // ... other dependencies
)
```

## Troubleshooting

### Application Hangs on Startup

If the application hangs during startup:

1. Check if PostgreSQL binaries are available:
   ```bash
   ls /usr/lib/postgresql/*/bin/pg_ctl
   ```

2. Clear the bleve search index if corrupted:
   ```bash
   rm -rf databases/simpleEDMSIndex.bleve
   ```

3. Check the logs:
   ```bash
   tail -f goedms.log
   ```

### Port Already in Use

The application automatically tries alternative ports if 8000 is in use. Check the console output for the actual port being used.

### Migration Errors

If you see migration errors, ensure:
- The migration files exist in `database/migrations/`
- The PostgreSQL migration (version 2) is being applied, not the SQLite one

## Performance

Typical startup times with ephemeral PostgreSQL:
- PostgreSQL server start: ~400ms
- Database creation: ~20ms
- Migration execution: ~10ms
- Total overhead: ~450ms

This is significantly faster than embedded-postgres which could take 5-10 seconds to start.

## Benefits Over embedded-postgres

1. **Faster startup**: Sub-second vs multiple seconds
2. **Better isolation**: Each run gets a clean database
3. **No filesystem persistence**: No cleanup needed
4. **Simpler codebase**: Less configuration and error handling
5. **Standard PostgreSQL**: Uses system PostgreSQL binaries

## Future Improvements

- Add support for PostgreSQL connection pooling
- Implement parallel test execution with shared PostgreSQL instance
- Add Docker Compose configuration for production-like testing
- Support for PostgreSQL extensions in ephemeral mode
