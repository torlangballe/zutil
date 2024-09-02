package zbits

import (
	"strings"
)

type NamedBit uint64
type NamedBitMap map[string]uint64

func (n NamedBit) ToString(m NamedBitMap) string {
	var str string
	has := NamedBitMap{}
outer:
	for name, mask := range m {
		if mask == 0 {
			continue
		}
		if uint64(n)&mask == mask {
			for hn, hm := range has {
				if uint64(hm)&mask != 0 {
					if mask > hm {
						delete(has, hn)
					} else {
						continue outer
					}
				}
			}
			has[name] = mask
		}
	}
	for name := range has {
		if str != "" {
			str += "|"
		}
		str += name
	}
	return str
}

func NamedFromString(str string, m NamedBitMap) NamedBit {
	var u uint64
	for _, part := range strings.Split(str, "|") {
		u |= m[part]
	}
	return NamedBit(u)
}

func (n *NamedBit) FromJSON(bytes []byte, m NamedBitMap) error {
	str := strings.Trim(string(bytes), `"`)
	*n = NamedFromString(str, m)
	return nil
}

func (n NamedBit) ToJSON(m NamedBitMap) ([]byte, error) {
	str := `"` + n.ToString(m) + `"`
	return []byte(str), nil
}

func (n *NamedBit) ChangeBit(mask NamedBit, on bool) {
	if on {
		*n |= mask
	} else {
		*n &= ^mask
	}
}
