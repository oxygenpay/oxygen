package util

import (
	"math/rand"
	"strconv"
	"time"
)

type strings string

const Strings strings = ""

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

//nolint:gosec
var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func (strings) ToFloat64(s string) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return 0
}

func (strings) Nullable(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func (strings) Random(l int) string {
	b := make([]byte, l)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
