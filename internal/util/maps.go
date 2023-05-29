package util

func Keys[T comparable, V any](items map[T]V) []T {
	keys := make([]T, 0)

	for k := range items {
		keys = append(keys, k)
	}

	return keys
}

func ToStringMap[T ~string](items []T) []string {
	result := make([]string, len(items))
	for i := range items {
		result[i] = string(items[i])
	}

	return result
}
