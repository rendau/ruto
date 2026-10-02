package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestIsUsernameConflict(t *testing.T) {
	conflict := &pgconn.PgError{Code: pgUniqueViolationCode, ConstraintName: usernameUniqueIndex}

	assert.True(t, isUsernameConflict(conflict))
	assert.True(t, isUsernameConflict(fmt.Errorf("fail to exec: %w", conflict)))

	assert.False(t, isUsernameConflict(&pgconn.PgError{Code: pgUniqueViolationCode, ConstraintName: "usr_pkey"}))
	assert.False(t, isUsernameConflict(&pgconn.PgError{Code: "23503", ConstraintName: usernameUniqueIndex}))
	assert.False(t, isUsernameConflict(errors.New("boom")))
	assert.False(t, isUsernameConflict(nil))
}
