package zgeo

/*

func scanFloat64(data []byte, littleEndian bool) float64 {
	var v uint64

	if littleEndian {
		for i := 7; i >= 0; i-- {
			v <<= 8
			v |= uint64(data[i])
		}
	} else {
		for i := 0; i < 8; i++ {
			v <<= 8
			v |= uint64(data[i])
		}
	}

	return math.Float64frombits(v)
}

func scanPrefix(data []byte) (bool, uint32, error) {
	if len(data) < 6 {
		return false, 0, errors.New("Not WKB4")
	}
	if data[0] == 0 {
		return false, scanUint32(data[1:5], false), nil
	}
	if data[0] == 1 {
		return true, scanUint32(data[1:5], true), nil
	}
	zlog.Info("scanPrefix data[0]:", data[0])
	return false, 0, errors.New("Not WKB5")
}

func scanUint32(data []byte, littleEndian bool) uint32 {
	var v uint32

	if littleEndian {
		for i := 3; i >= 0; i-- {
			v <<= 8
			v |= uint32(data[i])
		}
	} else {
		for i := 0; i < 4; i++ {
			v <<= 8
			v |= uint32(data[i])
		}
	}

	return v
}

func (p *Pos) Scan(val interface{}) error {
	b, ok := val.([]byte)
	if !ok {
		return errors.New("GisGeoPoint unsupported data type")
	}
	data, err := hex.DecodeString(string(b))
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if len(data) != 21 {
		// the length of a point type in WKB
		return errors.New(fmt.Sprintln("GisGeoPoint unsupported data length", len(data)))
	}

	littleEndian, typeCode, err := scanPrefix(data)
	if err != nil {
		return err
	}

	if typeCode != 1 {
		return errors.New("GisGeoPoint incorrect type for Point (not 1)")
	}

	p.X = scanFloat64(data[5:13], littleEndian)
	p.Y = scanFloat64(data[13:21], littleEndian)

	return nil
}

// Value implements the driver Valuer interface and will return the string representation of the GisGeoPoint struct by calling the String() method
func (p *Pos) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	str := p.String()
	//	zlog.Info("GisGeoPoint Value:", str)
	return []byte(str), nil
}
*/
