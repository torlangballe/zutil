package zmap

import "fmt"

func StringInterfaceMapJoin(m map[string]interface{}, equal, sep string) string {
	str := ""
	for k, v := range m {
		if str != "" {
			str += sep
		}
		str += fmt.Sprint(k, equal, v)
	}
	return str
}

func CopyStringInterfaceMap(m map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range m {
		sub, is := v.(map[string]interface{})
		if is {
			out[k] = CopyStringInterfaceMap(sub)
		} else {
			out[k] = v
		}
	}
	return out
}

func GetStringFromStrInterfaceMap(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

func GetFloat64FromStrInterfaceMap(m map[string]interface{}, key string) float64 { // returns 0 if nothing to get
	if f64, got := m[key].(float64); got {
		return f64
	}
	if f32, got := m[key].(float32); got {
		return float64(f32)
	}
	if i, got := m[key].(int); got {
		return float64(i)
	}
	if i32, got := m[key].(int32); got {
		return float64(i32)
	}
	if i64, got := m[key].(int64); got {
		return float64(i64)
	}
	return 0
}
