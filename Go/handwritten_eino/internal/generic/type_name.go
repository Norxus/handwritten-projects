package generic

import (
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

var (
	regOfAnonymousFunc = regexp.MustCompile(`^func[0-9]+`)
	regOfNumber        = regexp.MustCompile(`^\d+$`)
)

// 拿到类型的名称，如果是函数，需要进行更加精细化处理
func ParseTypeName(val reflect.Value) string {
	typ := val.Type()

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.Func {
		// 取运行时函数全名
		funcName := runtime.FuncForPC(val.Pointer()).Name()
		// 取最后一个点后面的部分作为函数名
		idx := strings.LastIndex(funcName, ".")
		if idx < 0 {
			if funcName != "" {
				return funcName
			}
			return ""
		}

		name := funcName[idx+1:]
		// 如果是匿名函数，返回空字符串
		if regOfAnonymousFunc.MatchString(name) {
			return ""
		}

		// 如果是数字，返回空字符串
		if regOfNumber.MatchString(name) {
			return ""
		}

		return name
	}

	return typ.Name()
}
