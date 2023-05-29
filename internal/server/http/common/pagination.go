package common

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

const (
	queryLimitMin     = 1
	queryLimitDefault = 30
	queryLimitMax     = 100
)

const (
	ParamQueryLimit        = "limit"
	ParamQueryCursor       = "cursor"
	ParamQueryReserveOrder = "reverseOrder"
)

var (
	ErrLimitInvalid        = errors.New("invalid limit")
	ErrReverseOrderInvalid = errors.New("invalid reverseOrder")
)

type PaginationQuery struct {
	Limit       int
	Cursor      string
	ReverseSort bool
}

func QueryPagination(c echo.Context) (PaginationQuery, error) {
	limit, err := QueryLimit(c)
	if err != nil {
		return PaginationQuery{}, err
	}

	reverse, err := QueryReverseOrder(c)
	if err != nil {
		return PaginationQuery{}, err
	}

	return PaginationQuery{
		Limit:       limit,
		Cursor:      c.QueryParams().Get(ParamQueryCursor),
		ReverseSort: reverse,
	}, nil
}

func QueryLimit(c echo.Context) (int, error) {
	raw := c.QueryParams().Get(ParamQueryLimit)
	if raw == "" {
		return queryLimitDefault, nil
	}

	i, err := strconv.Atoi(raw)
	if err != nil {
		return 0, ErrLimitInvalid
	}

	limit := i
	if limit == 0 {
		limit = queryLimitDefault
	}

	if limit < queryLimitMin || limit > queryLimitMax {
		return 0, ErrLimitInvalid
	}

	return limit, nil
}

func QueryReverseOrder(c echo.Context) (bool, error) {
	raw := c.QueryParams().Get(ParamQueryReserveOrder)
	if raw == "" {
		return false, nil
	}

	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, ErrReverseOrderInvalid
	}

	return b, nil
}
