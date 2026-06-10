// Package pgconv holds small conversions between pgx/pgtype values and plain Go
// types used by the domain (string UUIDs, UTC time.Time). Keeping these in one
// place avoids duplicating them across feature stores.
package pgconv

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Timestamptz wraps a time.Time as a valid pgtype.Timestamptz in UTC.
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// UUIDString renders a pgtype.UUID as its canonical textual form, or "" if null.
func UUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ParseUUID parses a canonical UUID string into a pgtype.UUID.
func ParseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, err
	}
	return u, nil
}
