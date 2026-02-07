package database

import (
	"fmt"
	"strings"
	"time"
)

// timeScanner implements sql.Scanner to handle timestamps from both
// PostgreSQL (returns time.Time) and go-postgres/SQLite (returns string).
// This is a workaround for go-postgres not coercing timestamps to time.Time.
type timeScanner struct {
	Time  time.Time
	Valid bool
}

func (ts *timeScanner) Scan(src any) error {
	if src == nil {
		ts.Valid = false
		return nil
	}
	ts.Valid = true
	switch v := src.(type) {
	case time.Time:
		ts.Time = v
	case string:
		// Strip Go's monotonic clock suffix (e.g. " m=+0.012028838")
		if idx := strings.Index(v, " m="); idx != -1 {
			v = strings.TrimSpace(v[:idx])
		}
		// Try common formats
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999 -0700 MST", // Go time.Time.String() format
			"2006-01-02 15:04:05.999999999 +0000 UTC",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05",
			"2006-01-02",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				ts.Time = t
				return nil
			}
		}
		return fmt.Errorf("timeScanner: cannot parse %q as time", v)
	default:
		return fmt.Errorf("timeScanner: unsupported type %T", src)
	}
	return nil
}

// nullTimeScanner is like timeScanner but for nullable time columns (*time.Time)
type nullTimeScanner struct {
	Time  *time.Time
	Valid bool
}

func (nts *nullTimeScanner) Scan(src any) error {
	if src == nil {
		nts.Time = nil
		nts.Valid = false
		return nil
	}
	var ts timeScanner
	if err := ts.Scan(src); err != nil {
		return err
	}
	nts.Time = &ts.Time
	nts.Valid = true
	return nil
}
