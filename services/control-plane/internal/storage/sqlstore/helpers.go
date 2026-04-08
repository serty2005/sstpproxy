package sqlstore

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"control-plane/internal/domain"
)

func normalizeWriteErr(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "unique") {
		return domain.ErrAlreadyExists
	}
	return err
}

func expectAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func stringPtr(valid sql.NullString) *string {
	if !valid.Valid {
		return nil
	}
	value := valid.String
	return &value
}

func timePtr(valid sql.NullTime) *time.Time {
	if !valid.Valid {
		return nil
	}
	value := valid.Time
	return &value
}

func int64Ptr(valid sql.NullInt64) *int64 {
	if !valid.Valid {
		return nil
	}
	value := valid.Int64
	return &value
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
