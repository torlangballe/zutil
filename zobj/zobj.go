package zobj

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

// type ObjectType string
type EventType int

type Baser interface {
	GetObjStructBasePtr() any // returns pointer to embedded struct that is actually registered in zobj, so that it can be used to get the struct table for it.
}

type column struct {
	field               string
	name                string
	kind                zreflect.TypeKind
	referencesToTable   string
	referencesCanBeNull bool
	uniqueGroup         string
}

// type link struct {
// 	ObjType ObjectType
// 	IsCount bool
// 	IsLoad  bool
// }

type StructTable struct {
	typeName string
	Table    string
	columns  []column
	// refsObjects    map[string]string // fname to table
	idFieldIndex   int
	userFieldIndex int
	nameFieldIndex int
	toJSON         map[string]bool
	created        bool
	isBaseType     bool // isBaseType means other types use it's table. Forces json column to be added for sub-type's fields.
	IsSubType      bool // is a StructTable that uses another StructTable with isBaseType as it's SQL table.
	EventHandlers  zmap.LockMap[EventType, func(et EventType, objPtr any)]
}

type SelectAsk struct {
	ForUserID int64
	TypeName  string
	Table     string
	Where     string
	// MakeLinkCountItems bool // if link type has n rows referencing object, add n empty elements to it. TODO: Work for non-slice
}

type InsertAsk struct {
	ForUserID    int64
	TypeName     string
	Table        string
	ObjectAsJSON []byte
}

type UpdateAsk struct {
	ForUserID    int64
	TypeName     string
	Table        string
	ObjectAsJSON []byte
	Where        string
}

type NameOwner interface {
	Name() string
}

const (
	InsertEvent EventType = iota + 1
	UpdateEvent
	SelectEvent
	DeleteEvent
	WakeUpEvent
	// ObjectTypeNone ObjectType = ""
)

var (
	NotFound = errors.New("not found")
)

func (st *StructTable) columForField(field string) *column {
	for i, c := range st.columns {
		if c.field == field {
			return &st.columns[i]
		}
	}
	return nil
}

func getInt64FromRVal(rval reflect.Value) (int64, error) {
	if rval.Kind() == reflect.String {
		return strconv.ParseInt(rval.String(), 10, 64)
	}
	return rval.Int(), nil
}

func GetIDInStruct(structPtr any, name string) (int64, error) {
	var n int64
	err := errors.New("id not found: " + name)
	zreflect.ForEachField(structPtr, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		tagKV, skip := each.TagKeyValuesForKey("zobj")
		if skip {
			return true
		}
		_, fi := zstr.KeyValuesFindForKey(tagKV, name)
		if fi == -1 {
			return true
		}
		n, err = getInt64FromRVal(each.ReflectValue)
		return false
	})
	return n, err
}

func GetNameInStruct(structPtr any) (string, bool) {
	no, _ := structPtr.(NameOwner)
	if no != nil {
		return no.Name(), true
	}
	var out string
	var got bool
	zreflect.ForEachField(structPtr, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		if each.StructField.Name == "Name" && each.ReflectValue.Kind() == reflect.String {
			out = each.ReflectValue.String()
			got = true
			return false
		}
		tagKV, skip := each.TagKeyValuesForKey("zobj")
		if skip {
			return true
		}
		_, fi := zstr.KeyValuesFindForKey(tagKV, "name")
		if fi == -1 {
			return true
		}
		got = true
		out = each.ReflectValue.String()
		return false
	})
	return out, got
}

func SetIDInStruct(structPtr any, n int64, name string) error {
	var set bool
	zreflect.ForEachField(structPtr, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		tagKV, skip := each.TagKeyValuesForKey("zobj")
		if skip {
			return true
		}
		_, fi := zstr.KeyValuesFindForKey(tagKV, name)
		if fi == -1 {
			return true
		}
		if each.ReflectValue.Kind() == reflect.String {
			each.ReflectValue.SetString(strconv.FormatInt(n, 10))
		} else {
			each.ReflectValue.SetInt(n)
		}
		set = true
		return false
	})
	if !set {
		return errors.New("not found")
	}
	return nil
}

var ObjRegister zreflect.TypeRegistrar[*StructTable]

func TypeNameToTableName(stype string) string {
	pkg, name := zstr.SplitInTwo(stype, ".")
	lowerName := strings.ToLower(name)
	lowerNameAndS := lowerName + "s"
	if lowerNameAndS == pkg {
		return lowerNameAndS
	}
	return pkg + "_" + lowerName
}

func RegisterTable(structure any) *StructTable {
	st := &StructTable{
		idFieldIndex:   -1,
		userFieldIndex: -1,
		nameFieldIndex: -1,
		toJSON:         map[string]bool{},
	}
	base := structure
	baser, isSubType := structure.(Baser)
	if isSubType {
		base = baser.GetObjStructBasePtr()
		if reflect.TypeOf(base) == reflect.TypeOf(structure) {
			isSubType = false
		}
	}
	// zlog.Info("RegisterTable1:", reflect.TypeOf(structure), reflect.TypeOf(base), isSubType) // , zdebug.CallingStackString()
	if reflect.TypeOf(structure).Kind() == reflect.Pointer {
		structure = reflect.ValueOf(structure).Elem().Interface()
	}
	st.typeName = ObjRegister.Register(structure, st)
	zreflect.ForEachField(structure, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		if each.StructField.Type.Kind() == reflect.Func {
			return true
		}
		tags, skip := each.TagKeyValuesForKey("zobj")
		if skip {
			return true
		}
		fname := each.StructField.Name
		if isSubType && !each.IsAnonymous {
			// zlog.Info("RegisterTable json", fname, reflect.TypeOf(structure))
			st.toJSON[fname] = true
			return true
		}
		// var addLink link
		var add column
		// zuiTagMap, _ := each.TagKeyValuesForKey("zui")
		for _, kv := range tags {
			switch kv.Key {
			case "base":
				// zlog.Info("RegisterTable base", fname, reflect.TypeOf(structure))
				if each.FieldIndex == st.idFieldIndex {
					st.isBaseType = true
				} else {
					zlog.Error("base field must be id field:", fname)
				}
			case "id":
				// zlog.Info("RegisterTable id", fname, reflect.TypeOf(structure))
				st.idFieldIndex = each.FieldIndex
				add = column{field: fname, name: "id", kind: zreflect.KindInt}
			case "userid":
				st.userFieldIndex = each.FieldIndex
				add = column{field: fname, name: "userid", kind: zreflect.KindInt}
				add.referencesToTable = "zusers"
			case "name":
				st.nameFieldIndex = each.FieldIndex
				add = column{field: fname, name: "name", kind: zreflect.KindString}
			case "null":
				add.referencesCanBeNull = true
			case "ref":
				add.referencesToTable = kv.Value
				break
			case "row": //, "refto":
				zkind := zreflect.KindFromReflectKindAndType(each.ReflectValue.Kind(), each.ReflectValue.Type())
				if zkind == zreflect.KindStruct || zkind == zreflect.KindMap || zkind == zreflect.KindSlice || zkind == zreflect.KindArray || zkind == zreflect.KindInterface {
					st.toJSON[fname] = true
				} else {
					add = column{
						field: fname,
						name:  strings.ToLower(fname),
						kind:  zkind,
					}
				}
			case "unique":
				str := kv.Value
				if str == "" {
					str = "1"
				}
				add.uniqueGroup = str
				break
			default:
				st.toJSON[fname] = true
			}
		}
		if add.name != "" {
			st.columns = append(st.columns, add)
		}
		return true
	})
	st.IsSubType = isSubType
	zlog.Assert(st.idFieldIndex != -1, reflect.TypeOf(structure), st.idFieldIndex)

	if isSubType {
		btype := reflect.TypeOf(base)
		typeName := zreflect.MakeTypeNameWithPackage(btype)
		st.Table = TypeNameToTableName(typeName)
	} else {
		st.Table = TypeNameToTableName(st.typeName)
	}
	return st
}

func LookupStructTableFromStruct(s any) (reflect.Type, *StructTable, error) {
	rtype := reflect.TypeOf(s)
	if rtype.Kind() == reflect.Pointer {
		rtype = rtype.Elem()
	}
	if rtype.Kind() == reflect.Slice {
		rtype = rtype.Elem()
	}
	if rtype.Kind() == reflect.Pointer {
		rtype = rtype.Elem()
	}
	typeName := rtype.String()
	_, st, got := ObjRegister.Lookup(typeName)
	if !got {
		return nil, nil, zlog.NewError("LookupStructTableFromStruct: Type not registered: '"+typeName+"'", zdebug.CallingStackString())
	}
	return rtype, st, nil
}

func LookupStructTable(typeName string) (reflect.Type, *StructTable, error) {
	rtype, info, got := ObjRegister.Lookup(typeName)
	if !got {
		return rtype, info, zlog.NewError("LookupStructTable: Type not registered: '"+typeName+"'", zdebug.CallingStackString())
	}
	return rtype, info, nil
}

func (st *StructTable) RegisterHandler(et EventType, handler func(et EventType, objPtr any)) {
	st.EventHandlers.Set(et, handler)
}

func DebugListRegTables() {
	ObjRegister.ForEach(func(key string, st *StructTable) bool {
		// zlog.Info("RegTable:", key, st.typeName, st.Table, st.isSubType, st.isBaseType, st.toJSON)
		return true
	})
}
