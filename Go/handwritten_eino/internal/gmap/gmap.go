package gmap

func Concat[K comparable, V any](ms ...map[K]V) map[K]V {
	if len(ms) == 0 {
		return make(map[K]V)
	}
	if len(ms) == 1 {
		return cloneWithoutNilCheck(ms[0])
	}

	// 因为是合并 map，所以需要取最大值
	var maxLen int
	for _, m := range ms {
		if len(m) > maxLen {
			maxLen = len(m)
		}
	}

	ret := make(map[K]V, maxLen)
	if maxLen == 0 {
		return ret
	}

	for _, m := range ms {
		for k, v := range m {
			ret[k] = v
		}
	}

	return ret
}

// 通过函数 f 映射 map m 中的每个键值对，返回一个新的 map r
func Map[K1, K2 comparable, V1, V2 any](m map[K1]V1, f func(K1, V1) (K2, V2)) map[K2]V2 {
	r := make(map[K2]V2, len(m))
	for k, v := range m {
		k2, v2 := f(k, v)
		r[k2] = v2
	}
	return r
}

func Values[K comparable, V any](m map[K]V) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

func Clone[K comparable, V any, M ~map[K]V](m M) M {
	r := make(M, len(m))
	for k, v := range m {
		r[k] = v
	}
	return r
}

// ~map[K]V = “接受所有底层类型是 map[K]V 的 map 类型”
func cloneWithoutNilCheck[K comparable, V any, M ~map[K]V](m M) M {
	r := make(M, len(m))
	for k, v := range m {
		r[k] = v
	}
	return r
}
