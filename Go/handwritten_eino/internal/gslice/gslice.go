package gslice

// 根据 slice 中的值，通过 f 映射为 map
func ToMap[T, V any, K comparable](s []T, f func(T) (K, V)) map[K]V {
	m := make(map[K]V, len(s))
	for _, e := range s {
		k, v := f(e)
		m[k] = v
	}
	return m
}
