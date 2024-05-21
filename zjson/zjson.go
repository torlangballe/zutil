package zjson

import (
	"github.com/torlangballe/zutil/zlog"
)

func MarshalEnum[S comparable](from S, m map[string]S) ([]byte, error) {
	for k, v := range m {
		if v == from {
			return []byte(k), nil
		}
	}
	return nil, zlog.Error("No value:", from)
}

func UnmarshalEnum[S comparable](to *S, data []byte, m map[string]S) error {
	key := string(data)
	v, got := m[key]
	if !got {
		return zlog.Error("No value for key:", key)
	}
	*to = v
	return nil
}
