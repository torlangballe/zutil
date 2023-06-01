package zbits

import (
	"strings"
)

type NamedBit uint64
type NamedBitMap map[string]uint64

func (n NamedBit) ToString(m NamedBitMap) string {
	var str string
	for i := uint64(0); i < 64; i++ {
		mask := NamedBit(1 << i)
		if n&mask == 0 {
			continue
		}
		for s, j := range m {
			// zlog.Info("BitAdd?:", s, mask, j)
			if j == uint64(mask) {
				str += s + "|"
				break
			}
		}
	}
	if len(str) != 0 {
		str = str[:len(str)-1]
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
