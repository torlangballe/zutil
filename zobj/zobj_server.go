//go:build server

package zobj

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zslices"
	"github.com/torlangballe/zutil/zsql"
	"github.com/torlangballe/zutil/zstr"
)

var (
	insertTriggers = map[string]func(typeName string, structure any, id int64){}
)

func InitServer(executor znamedfuncs.Executioner) {
	executor.Register(ZOBJCalls{})
}

func RegisterTableForSQL(structure any) string {
	st := RegisterTable(structure)
	if !st.created && !st.IsSubType {
		createTableForStruct(st)
	}
	return st.typeName
}

func checkForUserByUser(forUserID, byUserID int64) error {
	return nil
}

func createTableForStruct(st *StructTable) {
	squery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", st.Table)
	uniqueGroups := map[string][]string{}
	for i, c := range st.columns {
		if i != 0 {
			squery += ","
		}
		squery += "\n" + c.name + " "
		if c.name == "id" {
			squery += "SERIAL PRIMARY KEY, typename TEXT NOT NULL"
			continue
		}
		switch c.kind {
		case zreflect.KindString:
			squery += "TEXT NOT NULL DEFAULT ''"
		case zreflect.KindInt:
			squery += "INT"
			if c.referencesToTable != "" && !c.referencesCanBeNull {
				squery += " NOT NULL DEFAULT 0"
			}
		case zreflect.KindFloat:
			squery += "REAL NOT NULL DEFAULT 0"
		case zreflect.KindBool:
			squery += "BOOLEAN NOT NULL DEFAULT false"
		}
		if c.referencesToTable != "" {
			squery += fmt.Sprintf(" REFERENCES %s (id)", c.referencesToTable)
			if c.referencesCanBeNull {
				squery += " ON DELETE SET NULL"
			} else {
				squery += " ON DELETE CASCADE"
			}
			// zlog.Info("createTableForStruct ref:", st.typeName, c.name, c.referencesToTable)
		}
		if c.uniqueGroup != "" {
			list := uniqueGroups[c.uniqueGroup]
			list = append(list, c.name)
			uniqueGroups[c.uniqueGroup] = list
		}
	}
	if st.isBaseType || len(st.toJSON) > 0 {
		squery += ",\njsondata jsonb NOT NULL DEFAULT '{}'"
	}
	for _, cols := range uniqueGroups {
		sort.Strings(cols)
		sname := "unique_" + strings.Join(cols, "_")
		scols := strings.Join(cols, ",")
		squery += ",\n"
		squery += fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)", sname, scols)
	}
	squery += "\n)\n"
	_, err := zsql.Main.DB.Exec(squery)
	zlog.AssertNotError(err, squery)
	st.created = true
}

func SelectSingle[S any](s *S, forUserID, byUserID int64, where string) error {
	var slice []S
	err := Select(&slice, forUserID, byUserID, where)
	if err != nil {
		return err
	}
	if len(slice) == 0 {
		return sql.ErrNoRows
	}
	*s = slice[0]
	return nil
}

func SelectToInterfaceForBase[BASE any, I any](slice *[]I, forUserID, byUserID int64, where string) error {
	var b BASE
	rtype, st, err := LookupStructTableFromStruct(b)

	if zlog.OnError(err, reflect.TypeOf(b)) {
		return err
	}
	w := MakeUserIDWhere(forUserID, where)
	err = selectIntoSlice(slice, st, rtype, w, forUserID, byUserID, true)
	if zlog.OnError(err, st.Table, rtype, w) {
		return err
	}
	return nil
}

func Select[S any](slice *[]S, forUserID, byUserID int64, where string) error {
	var s S
	rtype, st, err := LookupStructTableFromStruct(s)
	if zlog.OnError(err, reflect.TypeOf(s)) {
		return err
	}
	w := MakeUserIDWhere(forUserID, where)
	err = selectIntoSlice(slice, st, rtype, w, forUserID, byUserID, false)
	return err
}

func MakeUserIDWhere(userID int64, where string) string {
	if where != "" {
		where = " AND (" + where + ")"
	}
	return fmt.Sprintf("userid=%d%s", userID, where)
}

func selectIntoSlice(slicePtr any, st *StructTable, rtype reflect.Type, where string, forUserID, byUserID int64, toInterface bool) error {
	var jsonCol zdict.Dict
	var results []any
	var cols []string

	var typeName string
	cols = append(cols, "typename")
	results = append(results, &typeName)
	// zlog.Info("selectIntoSlice", reflect.TypeOf(slicePtr))
	err := checkForUserByUser(forUserID, byUserID)
	if err != nil {
		return err
	}
	objRValPtr := reflect.New(rtype)
	nullIntMap := map[int]*int64{}
	nullIntResultIndexMap := map[int]int{}
	zreflect.ForEachField(objRValPtr.Interface(), zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		c := st.columForField(each.StructField.Name)
		if c == nil {
			return true
		}
		cols = append(cols, c.name)
		if c.referencesCanBeNull {
			zlog.Assert(each.ReflectValue.Kind() == reflect.Int64)
			nullIntResultIndexMap[each.FieldIndex] = len(results)
			results = append(results, &sql.NullInt64{})
			// zlog.Info("selectIntoSlice nullable ref:", each.FieldIndex, each.StructField.Name, c.name, c.referencesToTable)
			nullIntMap[each.FieldIndex] = each.ReflectValue.Addr().Interface().(*int64)
			return true
		}
		receive := each.ReflectValue.Addr().Interface()
		results = append(results, receive)
		return true
	})
	if len(st.toJSON) > 0 {
		cols = append(cols, "jsondata")
		results = append(results, &jsonCol)
	}
	scols := strings.Join(cols, ",")
	query := fmt.Sprintf("SELECT %s FROM %s", scols, st.Table)
	if where != "" {
		query += " WHERE " + where
	}
	row, err := zsql.Main.DB.Query(query)
	// zlog.Info("selectIntoSlice:", query, err)
	if err != nil {
		return zlog.Error(err, query, len(results), query, zdebug.CallingStackString())
	}
	for row.Next() {
		err = row.Scan(results...)
		if err != nil {
			return zlog.Error(err, "scan")
		}
		for i, intPtr := range nullIntMap {
			// field := zreflect.FieldForIndex(objRValPtr.Interface(), zreflect.FlattenIfAnonymous, i)
			// zlog.Info("selectIntoSlice nullable ref val?:", i, intPtr != nil, field.ReflectValue.Type())
			resultIndex := nullIntResultIndexMap[i]
			v := results[resultIndex].(*sql.NullInt64)
			if v.Valid {
				// zlog.Info("selectIntoSlice nullable ref value?:", i, zlog.Full(v))
				*intPtr = v.Int64
			}
		}
		var oval reflect.Value
		if typeName != st.typeName {
			subRow, _, got := ObjRegister.NewForType(typeName)
			if !got {
				return zlog.NewError("type not found:", typeName)
			}
			bSub, _ := subRow.(Baser)
			bRow, _ := objRValPtr.Interface().(Baser)
			// zlog.Info("selectIntoSlice subType:", rtype, typeName, st.typeName, reflect.TypeOf(subRow), reflect.TypeOf(objRValPtr.Interface()), bSub != nil, objRValPtr.Interface() != nil)
			if bSub != nil && bRow != nil && bRow.GetObjStructBasePtr() == objRValPtr.Interface() {
				// zlog.Info("selectIntoSlice scan1:", objRValPtr.Elem().Type(), reflect.ValueOf(bSub.GetObjStructBasePtr()).Elem().Type())
				// zlog.Info("selectIntoSlice scan:", zlog.Full(objRValPtr.Elem().Interface()))
				reflect.ValueOf(bSub.GetObjStructBasePtr()).Elem().Set(objRValPtr.Elem()) // copy
				oval = reflect.ValueOf(subRow)
			}
		}
		if !oval.IsValid() {
			oval = zreflect.CopyRVal(objRValPtr)
		}
		if len(st.toJSON) > 0 {
			zreflect.MapToStruct(jsonCol, oval.Interface())
			// zlog.Info("selectIntoSlice jsonCol:", oval.Type(), jsonCol)
		}
		// zlog.Info("selectIntoSlice scan2:", st.Table, zlog.Full(oval.Interface()))
		err := checkUserIDInStruct(objRValPtr.Interface(), forUserID, byUserID)
		if err != nil {
			zlog.OnError(err, "checkUserIDInStruct for selectIntoSlice")
			return err
		}
		add := oval
		if !toInterface {
			add = add.Elem()
		}
		zslices.AddAtEnd(slicePtr, add.Interface())
		objRValPtr.Elem().SetZero()
	}
	// for i := range reflect.ValueOf(slicePtr).Elem().Len() {
	// 	v := reflect.ValueOf(slicePtr).Elem().Index(i)
	// 	zlog.Info("selectIntoSliceX:", i, v.Type(), zlog.Full(v.Interface()))
	// }
	return nil
}

func checkUserIDInStruct(objPtr any, forUserID, byUserID int64) error {
	if byUserID == 0 && forUserID == 0 {
		return nil
	}
	userID, err := GetIDInStruct(objPtr, "userid")
	if err != nil {
		return err
	}
	var insert bool
	if userID == 0 {
		insert = true
		userID = forUserID
	}
	if userID == 0 {
		return zlog.NewError("no user id for insert")
	}
	err = checkForUserByUser(userID, byUserID)
	if err != nil {
		return err
	}
	if insert {
		err = SetIDInStruct(objPtr, userID, "userid")
		if err != nil {
			return err
		}
	}
	return nil
}

// Upsert uses zsql.UpsertRow to upsert the struct *S
// conflictCol is the column name or multi-column constraint name to check for conflicts.
// If a conflict is found, the row will be updated instead of inserted.
func Upsert[S any](s *S, conflictCol, idCol, where string, skipColumns []string, forUserID int64) (int64, error) {
	_, st, err := LookupStructTableFromStruct(s)
	if zlog.OnError(err, reflect.TypeOf(s)) {
		return 0, err
	}
	SetIDInStruct(s, forUserID, "userid") //  no error in case no userid
	id, err := zsql.UpsertRow(s, st.Table, conflictCol, skipColumns, idCol, where)
	if err != nil {
		return 0, err
	}
	SetIDInStruct(s, id, "id") //  no error in case no userid
	return id, nil
}

func Insert(objPtr any, forUserID, byUserID int64) (int64, error) {
	rtype, st, err := LookupStructTableFromStruct(objPtr)
	if zlog.OnError(err, reflect.TypeOf(objPtr)) {
		return 0, err
	}
	return insertFromStruct(objPtr, st, rtype, forUserID, byUserID)
}

func insertFromStruct(objPtr any, st *StructTable, rtype reflect.Type, forUserID, byUserID int64) (int64, error) {
	var cols, vals string
	var params []any
	var id int64
	var jsonCol = zdict.Dict{}

	err := checkUserIDInStruct(objPtr, forUserID, byUserID)
	if zlog.OnError(err, forUserID, byUserID) {
		return 0, err
	}
	i := 1
	zreflect.ForEachField(objPtr, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		col := st.columForField(each.StructField.Name)
		if col == nil {
			if st.toJSON[each.StructField.Name] {
				// zlog.Info("Json:", each.StructField.Name, each.ReflectValue.Interface())
				jsonCol[each.StructField.Name] = each.ReflectValue.Interface()
			}
			return true
		}
		if col.name == "id" {
			return true
		}
		// zlog.Info("insertFromStruct:", st.typeName, col.name, col.referencesToTable, col.referencesCanBeNull, each.ReflectValue.IsZero(), each.ReflectValue.Interface())
		if col.referencesToTable != "" && col.referencesCanBeNull && each.ReflectValue.IsZero() {
			return true
		}
		// if col.name == "userid" {
		// 	return true
		// }
		// zlog.Info("Json?:", each.StructField.Name, st.columns)
		if cols != "" {
			cols += ","
			vals += ","
		}
		cols += col.name
		vals += fmt.Sprintf("$%d", i)
		params = append(params, each.ReflectValue.Interface())
		i++
		return true
	})
	cols += ", typename"
	vals += fmt.Sprintf(",$%d", i)
	params = append(params, st.typeName)
	i++
	if len(st.toJSON) > 0 {
		vals += fmt.Sprintf(",$%d", i)
		cols += ",jsondata"
		data, err := json.Marshal(jsonCol)
		// zlog.Info("zobj.InsertFromStruct Data:", string(data), err, jsonCol, st.toJSON)
		if zlog.OnError(err) {
			return 0, err
		}
		// sval := zsql.QuoteString(string(data))
		params = append(params, data)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id", st.Table, cols, vals)
	row := zsql.Main.DB.QueryRow(query, params...)
	err = row.Scan(&id)
	zlog.Info("Insert", query, err, params)
	if err != nil {
		return 0, zlog.Error(err, query)
	}
	f := st.EventHandlers.Index(InsertEvent)
	if f != nil {
		f(InsertEvent, objPtr)
	}
	return id, nil
}

// Update updates the struct to DB.
func Update(objPtr any, forUserID, byUserID int64, where string) error {
	rtype, st, err := LookupStructTableFromStruct(objPtr)
	if zlog.OnError(err, reflect.TypeOf(objPtr)) {
		return err
	}
	return updateFromStruct(objPtr, st, rtype, forUserID, byUserID, where)
}

func Delete[S any](ids []int64, byUserID int64, where string) error {
	var s S
	_, st, err := LookupStructTableFromStruct(&s)
	if zlog.OnError(err, reflect.TypeOf(s)) {
		return err
	}
	sids := zint.Join64(ids, ",")
	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s) AND userid=%d", st.Table, sids, byUserID)
	if where != "" {
		query += " AND (" + where + ")"
	}
	_, err = zsql.Main.DB.Exec(query)
	if err != nil {
		return zlog.Error(err, query)
	}
	f := st.EventHandlers.Index(DeleteEvent)
	if f != nil {
		f(DeleteEvent, &s)
	}
	return nil
}

// updateFromStruct updates the struct to DB.
// If where is empty, id=<current id> is used.
func updateFromStruct(objPtr any, st *StructTable, rtype reflect.Type, forUserID, byUserID int64, where string) error {
	var set string
	var params []any
	var jsonCol = zdict.Dict{}

	if where == "" {
		id, err := GetIDInStruct(objPtr, "id")
		if err != nil {
			return err
		}
		where = fmt.Sprintf("id=%d", id)
	}
	err := checkUserIDInStruct(objPtr, forUserID, byUserID)
	if zlog.OnError(err, forUserID, byUserID) {
		return err
	}
	i := 1
	zreflect.ForEachField(objPtr, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		col := st.columForField(each.StructField.Name)
		if col == nil {
			if st.toJSON[each.StructField.Name] {
				// zlog.Info("Json:", each.StructField.Name)
				jsonCol[each.StructField.Name] = each.ReflectValue.Interface()
			}
			return true
		}
		if col.name == "id" {
			return true
		}
		if col.referencesToTable != "" && col.referencesCanBeNull && each.ReflectValue.IsZero() {
			return true
		}
		if i != 1 {
			set += ", "
		}
		// zlog.Info("updateFromStruct:", set, col.name, each.ReflectValue.Interface())
		set += fmt.Sprintf("%s=$%d", col.name, i)
		params = append(params, each.ReflectValue.Interface())
		i++
		return true
	})
	if len(st.toJSON) > 0 {
		set += fmt.Sprintf(", jsondata=$%d", i)
		data, err := json.Marshal(jsonCol)
		// zlog.Info("Data:", string(data), err, jsonCol, st.toJSON)
		if zlog.OnError(err) {
			return err
		}
		// sval := zsql.QuoteString(string(data))
		params = append(params, data)
	}
	if where != "" {
		where = "WHERE " + where
	}
	query := fmt.Sprintf("UPDATE %s SET %s %s", st.Table, set, where)
	_, err = zsql.Main.DB.Exec(query, params...)
	if err != nil {
		return zlog.Error(err, query)
	}
	f := st.EventHandlers.Index(UpdateEvent)
	if f != nil {
		f(UpdateEvent, objPtr)
	}
	return nil
}

func IDsOfNamedObjects[S any](names []string, userID int64) ([]int64, error) {
	var s S
	// objType := ObjectTypeForStruct(s)
	_, st, err := LookupStructTableFromStruct(&s)
	if zlog.OnError(err, reflect.TypeOf(s)) {
		return nil, err
	}
	where := fmt.Sprintf("userid=%d", userID)
	nameIDs, err := zsql.Main.SelectIDOfNames(st.Table, "name", "id", names, where)
	if err != nil {
		return nil, err
	}
	return nameIDs, nil
}

func NameForID(table string, id, userID int64) (string, error) {
	var name string
	query := fmt.Sprintf("SELECT name FROM %s WHERE id=$1 AND userid=%d", table, userID)
	row := zsql.Main.DB.QueryRow(query, id)
	err := row.Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}

func GetNamesAndIDsForAny(table string, userID int64, where string) ([]zstr.StrInt64, error) {
	var nids []zstr.StrInt64
	var nid zstr.StrInt64
	query := fmt.Sprintf("SELECT name, id FROM %s WHERE userid=$1", table)
	if where != "" {
		query += " AND (" + where + ")"
	}
	query += " ORDER BY name ASC"
	rows, err := zsql.Main.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&nid.Str, &nid.Int)
		if err != nil {
			return nil, err
		}
		nids = append(nids, nid)
	}
	return nids, nil
}
