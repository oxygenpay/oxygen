package util

func Set[T comparable](items []T) map[T]struct{} {
	result := make(map[T]struct{})

	for _, i := range items {
		result[i] = struct{}{}
	}

	return result
}

func MapSlice[T, E any](items []T, mapper func(T) E) []E {
	results := make([]E, len(items))

	for i := range items {
		results[i] = mapper(items[i])
	}

	return results
}

func FilterSlice[T any](items []T, filter func(T) bool) []T {
	results := make([]T, 0)

	for i := range items {
		if filter(items[i]) {
			results = append(results, items[i])
		}
	}

	return results
}

func KeyFunc[V any, K comparable](items []V, fn func(V) K) map[K]V {
	results := make(map[K]V)

	for i := range items {
		results[fn(items[i])] = items[i]
	}

	return results
}
