package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func (i *IntegrationTest) AssertTableRows(t *testing.T, table string, expected int64) {
	var counter int64
	err := i.Database.Conn().QueryRow(i.Context, fmt.Sprintf("select count(0) from %s;", table)).Scan(&counter)

	assert.NoError(t, err, "unable to count rows at table %s", table)
	assert.Equal(t, expected, counter)
}

func (i *IntegrationTest) AssertTableRowsByMerchant(t *testing.T, merchantID int64, table string, expected int64) {
	var counter int64
	sql := fmt.Sprintf("select count(0) from %s where merchant_id = %d", table, merchantID)

	err := i.Database.Conn().QueryRow(i.Context, sql).Scan(&counter)

	assert.NoError(t, err, "unable to count rows at table %s", table)
	assert.Equal(t, expected, counter)
}
