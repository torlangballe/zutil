package zreflect

/*
type Item struct {
	Kind            TypeKind
	BitSize         int
	TypeName        string
	FieldName       string
	Tag             string
	IsAnonymous     bool
	IsSlice         bool
	IsPointer       bool
	Address         any
	Package         string
	Value           reflect.Value
	Interface       any
	SimpleInterface any
	Children        []Item
}

type Options struct {
	UnnestAnonymous        bool
	Recursive              bool
	MakeSliceElementIfNone bool
}

func iterate(level int, fieldName, typeName, tagName string, isAnonymous bool, val reflect.Value, options Options) (item Item, err error) {
	item.FieldName = fieldName
	vtype := val.Type()
	if typeName == "" {
		typeName = vtype.Name()
	}
	item.TypeName = typeName
	item.Package = vtype.PkgPath()
	if !val.CanInterface() {
		fmt.Println("zreflect.iterate: can't interface to:", fieldName)
		return
	}
	if !val.IsValid() {
		err = errors.New("marshalValue: val.IsValid() failed")
		return
	}
	switch val.Kind() {
	case reflect.Interface:
		item.Interface = val.Interface()
		item.Kind = KindInterface

	case reflect.Ptr:
		t := vtype
		var v reflect.Value
		if val.IsNil() {
			t = vtype.Elem()
			v = reflect.Zero(t)
		} else {
			v = reflect.Indirect(val)
		}
		pItem, perr := iterate(level, fieldName, t.Name(), tagName, isAnonymous, v, options)
		err = perr
		item.Interface = val.Interface()
		item.IsPointer = true
		item.Kind = pItem.Kind
		item.Children = pItem.Children

	case reflect.Slice:
		t := reflect.TypeOf(val.Interface()).Elem()
		v := reflect.Zero(t)
		length := val.Len()
		if !v.CanAddr() {
			v = reflect.New(t)
		}
		item, err = iterate(level, fieldName, t.Name(), tagName, isAnonymous, v, options)
		if length == 0 && !options.MakeSliceElementIfNone {
			item.Children = item.Children[:0]
		}
		item.Value = val
		item.IsSlice = true
		item.Kind = KindSlice
		item.Interface = val.Interface()
		item.SimpleInterface = val.Interface()
		if options.Recursive { // !MakeSliceElementIfNone, wont happen if length is 0 anyway
			for i := 0; i < length; i++ {
				v := val.Index(i)
				sliceItem, serr := iterate(level+1, "", t.Name(), "", isAnonymous, v, options)
				if serr != nil {
					fmt.Println(serr, "slice item iterate")
					continue
				}
				item.Children = append(item.Children, sliceItem)
			}
		}
		return

	case reflect.Array:
		return

	case reflect.Struct:
		switch vtype {
		case TimeType:
			item.Kind = KindTime
			item.Interface = val.Interface()
			item.SimpleInterface = val.Interface()
		default:
			item.Interface = val.Interface()
			item.SimpleInterface = val.Interface()
			item.Kind = KindStruct
			n := vtype.NumField()
			// fmt.Printf("struct: %s %+v\n", fieldName, val.Interface())
			if level > 0 && !options.Recursive && (!options.UnnestAnonymous || !isAnonymous) { // always go into first level
				break
			}
			for i := 0; i < n; i++ {
				f := vtype.Field(i)
				fval := val.Field(i)
				if !fval.CanInterface() {
					continue
				}
				tag := string(f.Tag) // http://golang.org/pkg/reflect/#StructTag
				tname := fval.Type().Name()
				fname := f.Name
				l := level
				if !f.Anonymous {
					l++
				}
				c, e := iterate(l, fname, tname, tag, f.Anonymous, fval, options)
				if fval.CanAddr() {
					c.Address = fval.Addr().Interface()
				}
				if e != nil {
					err = e
				} else {
					if c.Value.CanAddr() {
						c.Address = c.Value.Addr().Interface()
					}
					if options.UnnestAnonymous && f.Anonymous {
						item.Children = append(item.Children, c.Children...)
					} else {
						item.Children = append(item.Children, c)
					}
				}
			}
		}

	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8:
		item.Kind = KindInt
		item.SimpleInterface = val.Int()
		item.Interface = val.Interface()

	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		item.Kind = KindInt
		item.SimpleInterface = val.Uint()
		item.Interface = val.Interface()

	case reflect.String:
		item.Kind = KindString
		item.Interface = val.Interface()

	case reflect.Bool:
		item.Kind = KindBool
		item.SimpleInterface = val.Bool()
		item.Interface = val.Interface()

	case reflect.Float32, reflect.Float64:
		item.Kind = KindFloat
		item.SimpleInterface = val.Float()
		item.Interface = val.Interface()

	case reflect.Map:
		item.Kind = KindMap
		item.Interface = val.Interface()
		item.SimpleInterface = val.Interface()

	case reflect.Func:
		item.Kind = KindFunc
		item.SimpleInterface = val.Interface()
		item.Interface = val.Interface()

	default:
		fmt.Println("marshal.marshalValue: Carefull, unknown type!", val.Kind())
		item.Kind = KindUndef
	}
	switch val.Kind() {
	case reflect.Int, reflect.Uint:
		item.BitSize = zint.SizeOfInt
	case reflect.Int16, reflect.Uint16:
		item.BitSize = 16
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		item.BitSize = 32
	case reflect.Int64, reflect.Uint64, reflect.Float64:
		item.BitSize = 64
	case reflect.Int8, reflect.Uint8:
		item.BitSize = 8
	}
	//item.FieldName = fieldName
	item.IsAnonymous = isAnonymous
	item.Tag = tagName
	item.Value = val
	return
}

func IterateStruct(istruct any, options Options) (item Item, err error) {
	rval := reflect.ValueOf(istruct)
	if !rval.IsValid() { //|| rval.IsZero() { //  && rval.Kind() != reflect.StructKind
		fmt.Println("IterateStruct: not valid", rval.IsValid(), rval.IsZero(), rval.Type(), rval.Kind())
		return
	}
	if rval.Kind() != reflect.Ptr {
		panic(zstr.Spaced("not pointer:", rval.Kind(), rval.Type(), rval))
	}
	return iterate(0, "", "", "", false, rval.Elem(), options)
}
*/
