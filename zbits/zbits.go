package zbits

import (
	"encoding/binary"
	"strings"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zmap"
)

func CopyBit(to *int64, from int64, mask int64) {
	ChangeBits(to, mask, from&mask != 0)
}

func ChangeBits[N zint.Integer](all *N, mask N, on bool) {
	if on {
		*all |= mask
	} else {
		*all &= ^mask
	}
}

func IntFromBits(data []byte, bits, startFromEnd int) int {
	up := zint.Max(0, startFromEnd-bits)
	mask := uint16((1<<bits - 1) << up)
	full := binary.BigEndian.Uint16(data)
	// fmt.Printf("GetIntFromBits: mask:%b full:%b bits:%d startfe:%d shift:%d and:%b %x %x\n", mask, full, bits, startFromEnd, up, full&mask, data[0], data[1])
	return int((full & mask) >> up)
}

func BoolFromBit(data byte, startFromEnd int) bool {
	return (data & (1 << startFromEnd)) != 0
}

func StringsToBits[N zint.Integer](str string, m map[N]string) N {
	var out N
	for _, s := range strings.Split(str, "|") {
		out |= zmap.KeyForValue(m, s)
	}
	return out
}

func BitsToStrings[N zint.Integer](n N, m map[N]string) string {
	var out string
	for bit, str := range m {
		if bit == 0 {
			continue
		}
		for b := range m {
			if b != bit && b > bit && b&bit == bit {
				continue // we don't output masks that have bigger composite masks
			}
		}
		if n&bit == bit {
			if out != "" {
				out += "|"
			}
			out += str
		}
	}
	return out
}
