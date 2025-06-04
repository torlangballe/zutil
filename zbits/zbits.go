package zbits

import (
	"encoding/binary"

	"github.com/torlangballe/zutil/zint"
)

func CopyBit(to *int64, from int64, mask int64) {
	ChangeBit(to, mask, from&mask != 0)
}

func ChangeBit(all *int64, mask int64, on bool) {
	if on {
		*all |= mask
	} else {
		*all &= ^mask
	}
}

func GetIntFromBits(data []byte, bits, startFromEnd int) int {
	up := zint.Max(0, startFromEnd-bits)
	mask := uint16((1<<bits - 1) << up)
	full := binary.BigEndian.Uint16(data)
	// fmt.Printf("GetIntFromBits: mask:%b full:%b bits:%d startfe:%d shift:%d and:%b %x %x\n", mask, full, bits, startFromEnd, up, full&mask, data[0], data[1])
	return int((full & mask) >> up)
}

func GetBoolFromBit(data byte, startFromEnd int) bool {
	return (data & (1 << startFromEnd)) != 0
}
