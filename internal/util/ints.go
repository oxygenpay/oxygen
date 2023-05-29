package util

func Pow64(a, b int64) int64 {
	if b < 0 {
		return 0
	}

	result := int64(1)
	for i := int64(0); i < b; i++ {
		result *= a
	}

	return result
}
