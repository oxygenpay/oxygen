package repository

import (
	"database/sql"
	"math/big"
	"time"

	"github.com/jackc/pgtype"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
)

func PointerStringToNullable(s *string) sql.NullString {
	var v string
	if s != nil {
		v = *s
	}

	return sql.NullString{String: v, Valid: s != nil}
}

func StringToNullable(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func NullableStringToPointer(s sql.NullString) *string {
	if s.Valid {
		return &s.String
	}

	return nil
}

func NullTimeToPointer(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}

	return &t.Time
}

func TimeToNullable(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

func Int64ToNullable(i int64) sql.NullInt64 {
	return sql.NullInt64{Int64: i, Valid: true}
}

func PointerInt64ToNullable(i *int64) sql.NullInt64 {
	var v int64
	if i != nil {
		v = *i
	}

	return sql.NullInt64{Int64: v, Valid: i != nil}
}

func NullableInt64ToPointer(i sql.NullInt64) *int64 {
	if i.Valid {
		return &i.Int64
	}

	return nil
}

func BigIntToNumeric(bigInt *big.Int) pgtype.Numeric {
	return pgtype.Numeric{
		Int:    new(big.Int).Set(bigInt),
		Status: pgtype.Present,
	}
}

func NumericToBigInt(num pgtype.Numeric) (*big.Int, error) {
	if num.Status != pgtype.Present {
		return nil, errors.New("numeric is nil")
	}

	bigInt := big.NewInt(0).Set(num.Int)

	if num.Exp > 0 {
		mul := &big.Int{}
		mul.Exp(big.NewInt(10), big.NewInt(int64(num.Exp)), nil)
		bigInt.Mul(bigInt, mul)
	}

	return bigInt, nil
}

func NumericToMoney(num pgtype.Numeric, moneyType money.Type, ticker string, decimals int64) (money.Money, error) {
	bigInt, err := NumericToBigInt(num)
	if err != nil {
		return money.Money{}, err
	}

	return money.NewFromBigInt(moneyType, ticker, bigInt, decimals)
}

func NumericToCrypto(num pgtype.Numeric, currency money.CryptoCurrency) (money.Money, error) {
	bigInt, err := NumericToBigInt(num)
	if err != nil {
		return money.Money{}, err
	}

	return currency.MakeAmountFromBigInt(bigInt)
}

func MoneyToNumeric(m money.Money) pgtype.Numeric {
	bigInt, _ := m.BigInt()
	return BigIntToNumeric(bigInt)
}

func MoneyToNegNumeric(m money.Money) pgtype.Numeric {
	bigInt, _ := m.BigInt()
	return BigIntToNumeric(big.NewInt(0).Neg(bigInt))
}
