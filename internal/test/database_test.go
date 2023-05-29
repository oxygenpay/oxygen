package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDB(t *testing.T) {
	// ARRANGE
	// Given a db with migrated schema
	db := NewDB(context.Background())

	// ACT 1
	// Query db
	testQuery := `select 
		case when count(0) > 5 then 'HAS_TABLES' else 'NO_TABLES' end as test_check
		from information_schema.tables where table_schema='public';`

	row := db.connection.QueryRow(db.Context(), testQuery)

	// Get results
	var result string
	err := row.Scan(&result)

	// Close db connection
	// Sidenote: if you call .Teardown() *before row.Scan() it will dead block
	// because rows.Scan() internally calls connection.Release()
	db.TearDown()

	// ASSERT
	assert.NoError(t, err)
	assert.Equal(t, result, "HAS_TABLES")
}
