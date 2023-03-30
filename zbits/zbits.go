package zbits

import (
	"encoding/binary"

	"github.com/torlangballe/zutil/zint"
)

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

/*
func SetBitsFromStruct(index int, data []byte, structure interface{}, handleSlice func(i int, fieldName string, s interface{}) int) int {
	items, err := zreflect.ItterateStruct(structure, zreflect.Options{UnnestAnonymous: true, Recursive: true})
	if err != nil {
		zlog.Error(err)
		return index
	}
	for _, item := range items.Children {
		var n int
		var ignore bool
		for _, part := range zreflect.GetTagAsMap(item.Tag)["zbits"] {
			if part == "ignore" {
				ignore = true
			} else {
				n, _ = strconv.Atoi(part)
				zlog.Assert(n != 0)
			}
		}
		if ignore {
			index += n
			continue
		}
		t := ""
		switch item.Kind {
		case zreflect.KindBool:
			t = "bool"
		case zreflect.KindInt:
			t = "int"
		case zreflect.KindSlice:
			index = handleSlice(index, item.FieldName, item.Address)
		case zreflect.KindStruct:
			index = SetBitsFromStruct(index, data, item.Address, handleSlice)
		}
		zlog.Info("Field:", index, item.FieldName, t, n)
		index += n
	}
	return index
}
*/
