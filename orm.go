package ora

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

var tbls = make(map[string]*tbl)

// Schema may optionally be specified to prefix a table name in the sql
// generated by the ora.Ins, ora.Upd, ora.Del, and ora.Sel methods.
var Schema string = ""

// ResType represents a result type returned by the ora.Sel method.
type ResType int

const (
	// SliceOfPtr indicates a slice of struct pointers will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	SliceOfPtr ResType = iota

	// SliceOfVal indicates a slice of structs will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	SliceOfVal

	// MapOfPtrPk indicates a map of struct pointers will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"pk"`.
	MapOfPtrPk

	// MapOfPtrFk1 indicates a map of struct pointers will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk1"`.
	MapOfPtrFk1

	// MapOfPtrFk2 indicates a map of struct pointers will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk2"`.
	MapOfPtrFk2

	// MapOfPtrFk3 indicates a map of struct pointers will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk3"`.
	MapOfPtrFk3

	// MapOfPtrFk4 indicates a map of struct pointers will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk4"`.
	MapOfPtrFk4

	// MapOfValPk indicates a map of structs will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"pk"`.
	MapOfValPk

	// MapOfValFk1 indicates a map of structs will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk1"`.
	MapOfValFk1

	// MapOfValFk2 indicates a map of structs will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk2"`.
	MapOfValFk2

	// MapOfValFk3 indicates a map of structs will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk3"`.
	MapOfValFk3

	// MapOfValFk4 indicates a map of structs will be returned by the ora.Sel method.
	// The struct type is specified to ora.Sel by the user.
	// The map key is determined by a struct field tagged with `db:"fk4"`.
	MapOfValFk4
)

// Represents attributes marked on a struct field `db` tag.
// Available tags are `db:"column_name,id,pk,fk1,fk2,fk3,fk4,-"`
type tag int

const (
	id  tag = 1 << iota
	pk  tag = 1 << iota
	fk1 tag = 1 << iota
	fk2 tag = 1 << iota
	fk3 tag = 1 << iota
	fk4 tag = 1 << iota
)

type tbl struct {
	name string
	cols []col
	typ  reflect.Type
	attr tag
}
type col struct {
	fieldIdx int
	name     string
	gct      GoColumnType
	attr     tag
}

// Ins inserts a struct into an Oracle table returning a possible error.
//
// Specify a struct, or struct pointer to parameter 'v' and an open Ses to
// parameter 'ses'.
//
// Optional struct field tags `db:"column_name,id,-"` may be specified to
// control how the sql INSERT statement is generated.
//
// By default, Ins generates and executes a sql INSERT statement based on the
// struct name and all exported field names. A struct name is used for the table
// name and a field name is used for a column name. Prior to calling Ins, you
// may specify an alternative table name to ora.AddTbl. An alternative column
// name may be specified to the field tag `db:"column_name"`. Specifying the
// `db:"-"` tag will remove a field from the INSERT statement.
//
// The optional `db:"id"` field tag may combined with the `db:"pk"` tag. A field
// tagged with `db:"pk,id"` indicates a field is a primary key backed by an
// Oracle identity sequence. `db:"pk,id"` may be tagged to one field per struct.
// When `db:"pk,id"` is tagged to a field Ins generates a RETURNING clause to
// recevie a db generated identity value. The `db:"id"` tag is not required and
// Ins will insert a struct to a table without returning an identity value.
//
// Set ora.Schema to specify an optional table name prefix.
func Ins(v interface{}, ses *Ses) (err error) {
	_drv.insMu.Lock()
	defer _drv.insMu.Unlock()
	defer func() {
		if value := recover(); value != nil {
			err = errR(value)
		}
	}()
	log(_drv.cfg.Log.Ins)
	tbl, err := tblGet(v)
	if err != nil {
		return errE(err)
	}
	// enable inserting to tables with `db:"id"` and without `db:"id"`
	// case 1: insert all columns/fields when no `db:"id"`
	// case 2: insert non-id columns when `db:"id"` present; capture id in
	//		   returning clause expect id field at last index.
	rv, err := finalValue(v)
	if err != nil {
		return errE(err)
	}
	params := make([]interface{}, len(tbl.cols))
	buf := new(bytes.Buffer)
	buf.WriteString("INSERT INTO ")
	if Schema != "" {
		buf.WriteString(Schema)
		buf.WriteString(".")
	}
	buf.WriteString(tbl.name)
	buf.WriteString(" (")
	colLen := len(tbl.cols)
	if tbl.attr&id != 0 {
		colLen--
	}
	for n := 0; n < colLen; n++ {
		col := tbl.cols[n]
		buf.WriteString(col.name)
		if n < colLen-1 {
			buf.WriteString(", ")
		} else {
			buf.WriteString(") VALUES (")
		}
		params[n] = rv.Field(col.fieldIdx).Interface() // build params for insert
	}
	for n := 1; n <= colLen; n++ { // use starting value of 1 for consistent bind param naming with Oracle
		buf.WriteString(fmt.Sprintf(":%v", n))
		if n < colLen {
			buf.WriteString(", ")
		} else {
			buf.WriteString(")")
		}
	}
	if tbl.attr&id != 0 { // add RETURNING clause for id
		last := len(tbl.cols) - 1
		lastCol := tbl.cols[last] // expect id positioned at last index
		buf.WriteString(" RETURNING ")
		buf.WriteString(lastCol.name)
		buf.WriteString(" INTO :RET_VAL")
		fv := rv.Field(lastCol.fieldIdx)
		if fv.Kind() == reflect.Ptr { // ensure last field is ptr to capture id from db
			params[last] = fv.Interface()
		} else {
			params[last] = fv.Addr().Interface()
		}
	}
	stmt, err := ses.Prep(buf.String())
	if err != nil {
		return errE(err)
	}
	defer stmt.Close()
	_, err = stmt.Exe(params...)
	if err != nil {
		return errE(err)
	}
	return nil
}

// Upd updates a struct to an Oracle table returning a possible error.
//
// Specify a struct, or struct pointer to parameter 'v' and an open Ses to
// parameter 'ses'.
//
// Upd requires one struct field tagged with `db:"pk"`. The field tagged with
// `db:"pk"` is used in a sql WHERE clause. Optional struct field tags
// `db:"column_name,-"` may be specified to control how the sql UPDATE statement
// is generated.
//
// By default, Upd generates and executes a sql UPDATE statement based on the
// struct name and all exported field names. A struct name is used for the table
// name and a field name is used for a column name. Prior to calling Upd, you may
// specify an alternative table name to ora.AddTbl. An alternative column name may
// be specified to the field tag `db:"column_name"`. Specifying the `db:"-"`
// tag will remove a field from the UPDATE statement.
//
// Set ora.Schema to specify an optional table name prefix.
func Upd(v interface{}, ses *Ses) (err error) {
	_drv.updMu.Lock()
	defer _drv.updMu.Unlock()
	defer func() {
		if value := recover(); value != nil {
			err = errR(value)
		}
	}()
	log(_drv.cfg.Log.Upd)
	tbl, err := tblGet(v)
	if err != nil {
		return errE(err)
	}
	rv, err := finalValue(v)
	if err != nil {
		return errE(err)
	}
	// enable updating to tables with pk only
	pairs := make([]interface{}, len(tbl.cols)*2)
	for n, col := range tbl.cols {
		p := n * 2
		pairs[p] = col.name
		pairs[p+1] = rv.Field(col.fieldIdx).Interface()
	}
	tblName := ""
	if Schema != "" {
		tblName = Schema + "." + tbl.name
	} else {
		tblName = tbl.name
	}
	err = ses.Upd(tblName, pairs...) // expects last pair is pk
	if err != nil {
		return errE(err)
	}
	return nil
}

// Del deletes a struct from an Oracle table returning a possible error.
//
// Specify a struct, or struct pointer to parameter 'v' and an open Ses to
// parameter 'ses'.
//
// Del requires one struct field tagged with `db:"pk"`. The field tagged with
// `db:"pk"` is used in a sql WHERE clause.
//
// By default, Del generates and executes a sql DELETE statement based on the
// struct name and one exported field name tagged with `db:"pk"`. A struct name
// is used for the table name and a field name is used for a column name. Prior
// to calling Del, you may specify an alternative table name to ora.AddTbl. An
// alternative column name may be specified to the field tag `db:"column_name"`.
//
// Set ora.Schema to specify an optional table name prefix.
func Del(v interface{}, ses *Ses) (err error) {
	_drv.delMu.Lock()
	defer _drv.delMu.Unlock()
	defer func() {
		if value := recover(); value != nil {
			err = errR(value)
		}
	}()
	log(_drv.cfg.Log.Del)
	tbl, err := tblGet(v)
	if err != nil {
		return errE(err)
	}
	rv, err := finalValue(v)
	if err != nil {
		return errE(err)
	}
	// enable deleting from tables with pk only
	lastCol := tbl.cols[len(tbl.cols)-1] // expect pk positioned at last index
	var buf bytes.Buffer
	buf.WriteString("DELETE FROM ")
	if Schema != "" {
		buf.WriteString(Schema)
		buf.WriteString(".")
	}
	buf.WriteString(tbl.name)
	buf.WriteString(" WHERE ")
	buf.WriteString(lastCol.name)
	buf.WriteString(" = :WHERE_VAL")
	_, err = ses.PrepAndExe(buf.String(), rv.Field(lastCol.fieldIdx).Interface())
	if err != nil {
		return errE(err)
	}
	return nil
}

// Sel selects structs from an Oracle table returning a specified container of
// structs and a possible error.
//
// Specify a struct, or struct pointer to parameter 'v' to indicate the struct
// return type. Specify a ResType to parameter 'rt' to indicate the container
// return type. Possible container return types include a slice of structs,
// slice of struct pointers, map of structs, and map of struct pointers.
// Specify an open Ses to parameter 'ses'. Optionally specify a where clause to
// parameter 'where' and where parameters to variadic parameter 'whereParams'.
//
// Optional struct field tags `db:"column_name,omit"` may be specified to
// control how the sql SELECT statement is generated. Optional struct field tags
// `db:"pk,fk1,fk2,fk3,fk4"` control how a map return type is generated.
//
// A slice may be returned by specifying one of the 'SliceOf' ResTypes to
// parameter 'rt'. Specify a SliceOfPtr to return a slice of struct pointers.
// Specify a SliceOfVal to return a slice of structs.
//
// A map may be returned by specifying one of the 'MapOf' ResTypes to parameter
// 'rt'. The map key type is based on a struct field type tagged with one of
// `db:"pk"`, `db:"fk1"`, `db:"fk2"`, `db:"fk3"`, or `db:"fk4"` matching
// the specified ResType suffix Pk, Fk1, Fk2, Fk3, or Fk4. The map value type is
// a struct pointer when a 'MapOfPtr' ResType is specified. The map value type
// is a struct when a 'MapOfVal' ResType is specified. For example, tagging a
// uint64 struct field with `db:"pk"` and specifying a MapOfPtrPk generates a
// map with a key type of uint64 and a value type of struct pointer.
//
// ResTypes available to specify to parameter 'rt' are MapOfPtrPk, MapOfPtrFk1,
// MapOfPtrFk2, MapOfPtrFk3, MapOfPtrFk4, MapOfValPk, MapOfValFk1, MapOfValFk2,
// MapOfValFk3, and MapOfValFk4.
//
// Set ora.Schema to specify an optional table name prefix.
func Sel(v interface{}, rt ResType, ses *Ses, where string, whereParams ...interface{}) (result interface{}, err error) {
	_drv.selMu.Lock()
	defer _drv.selMu.Unlock()
	defer func() {
		if value := recover(); value != nil {
			err = errR(value)
		}
	}()
	log(_drv.cfg.Log.Sel)
	tbl, err := tblGet(v)
	if err != nil {
		return nil, errE(err)
	}
	// build SELECT statement, GoColumnTypes
	gcts := make([]GoColumnType, len(tbl.cols))
	buf := new(bytes.Buffer)
	buf.WriteString("SELECT ")
	for n, col := range tbl.cols {
		buf.WriteString(col.name)
		if n != len(tbl.cols)-1 {
			buf.WriteString(", ")
		}
		gcts[n] = col.gct
	}
	buf.WriteString(" FROM ")
	if Schema != "" {
		buf.WriteString(Schema)
		buf.WriteString(".")
	}
	buf.WriteString(tbl.name)
	if where != "" {
		buf.WriteString(" ")
		whereIdx := strings.Index(strings.ToUpper(where), "WHERE")
		if whereIdx < 0 {
			buf.WriteString("WHERE ")
		}
		buf.WriteString(where)
	}
	// prep
	stmt, err := ses.Prep(buf.String(), gcts...)
	defer func() {
		err = stmt.Close()
		if err != nil {
			err = errE(err)
		}
	}()
	if err != nil {
		return nil, errE(err)
	}
	// qry
	rset, err := stmt.Qry(whereParams...)
	if err != nil {
		return nil, errE(err)
	}
	switch rt {
	case SliceOfPtr:
		sliceT := reflect.SliceOf(reflect.New(tbl.typ).Type())
		sliceOfPtrRV := reflect.MakeSlice(sliceT, 0, 0)
		for rset.Next() {
			ptrRV := reflect.New(tbl.typ)
			valRV := ptrRV.Elem()
			for n, col := range tbl.cols {
				f := valRV.Field(col.fieldIdx)
				f.Set(reflect.ValueOf(rset.Row[n]))
			}
			sliceOfPtrRV = reflect.Append(sliceOfPtrRV, ptrRV)
		}
		result = sliceOfPtrRV.Interface()
	case SliceOfVal:
		sliceT := reflect.SliceOf(tbl.typ)
		sliceOfValRV := reflect.MakeSlice(sliceT, 0, 0)
		for rset.Next() {
			valRV := reflect.New(tbl.typ).Elem()
			for n, col := range tbl.cols {
				f := valRV.Field(col.fieldIdx)
				f.Set(reflect.ValueOf(rset.Row[n]))
			}
			sliceOfValRV = reflect.Append(sliceOfValRV, valRV)
		}
		result = sliceOfValRV.Interface()
	case MapOfPtrPk, MapOfPtrFk1, MapOfPtrFk2, MapOfPtrFk3, MapOfPtrFk4:
		// lookup column for map key
		var keyRT reflect.Type
		switch rt {
		case MapOfPtrPk:
			for _, col := range tbl.cols {
				if col.attr&pk != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, fmt.Errorf("Unable to make a map of pk to pointers for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"pk\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfPtrFk1:
			for _, col := range tbl.cols {
				if col.attr&fk1 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, fmt.Errorf("Unable to make a map of fk1 to pointers for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk1\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfPtrFk2:
			for _, col := range tbl.cols {
				if col.attr&fk2 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, fmt.Errorf("Unable to make a map of fk2 to pointers for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk2\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfPtrFk3:
			for _, col := range tbl.cols {
				if col.attr&fk3 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, fmt.Errorf("Unable to make a map of fk3 to pointers for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk3\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfPtrFk4:
			for _, col := range tbl.cols {
				if col.attr&fk4 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, fmt.Errorf("Unable to make a map of fk4 to pointers for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk4\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		}
		mapT := reflect.MapOf(keyRT, reflect.New(tbl.typ).Type())
		mapOfPtrRV := reflect.MakeMap(mapT)
		for rset.Next() {
			var keyRV reflect.Value
			ptrRV := reflect.New(tbl.typ)
			valRV := ptrRV.Elem()
			for n, col := range tbl.cols {
				f := valRV.Field(col.fieldIdx)
				fv := reflect.ValueOf(rset.Row[n])
				f.Set(fv)
				switch rt {
				case MapOfPtrPk:
					if col.attr&pk != 0 { // validation ensures only one field is marked with `pk`
						keyRV = fv
					}
				case MapOfPtrFk1:
					if col.attr&fk1 != 0 { // validation ensures only one field is marked with `fk1`
						keyRV = fv
					}
				case MapOfPtrFk2:
					if col.attr&fk2 != 0 { // validation ensures only one field is marked with `fk2`
						keyRV = fv
					}
				case MapOfPtrFk3:
					if col.attr&fk3 != 0 { // validation ensures only one field is marked with `fk3`
						keyRV = fv
					}
				case MapOfPtrFk4:
					if col.attr&fk4 != 0 { // validation ensures only one field is marked with `fk4`
						keyRV = fv
					}
				}
			}
			mapOfPtrRV.SetMapIndex(keyRV, ptrRV)
		}
		result = mapOfPtrRV.Interface()
	case MapOfValPk, MapOfValFk1, MapOfValFk2, MapOfValFk3, MapOfValFk4:
		// lookup column for map key
		var keyRT reflect.Type
		switch rt {
		case MapOfValPk:
			for _, col := range tbl.cols {
				if col.attr&pk != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, errF("Unable to make a map of pk to values for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"pk\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfValFk1:
			for _, col := range tbl.cols {
				if col.attr&fk1 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, errF("Unable to make a map of fk1 to values for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk1\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfValFk2:
			for _, col := range tbl.cols {
				if col.attr&fk2 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, errF("Unable to make a map of fk2 to values for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk2\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfValFk3:
			for _, col := range tbl.cols {
				if col.attr&fk3 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, errF("Unable to make a map of fk3 to values for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk3\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		case MapOfValFk4:
			for _, col := range tbl.cols {
				if col.attr&fk4 != 0 {
					keyRT = tbl.typ.Field(col.fieldIdx).Type
					break
				}
			}
			if keyRT == nil {
				return nil, errF("Unable to make a map of fk4 to values for struct '%v'. '%v' doesn't have an exported field marked with a `db:\"fk4\"` tag.", tbl.typ.Name(), tbl.typ.Name())
			}
		}
		mapT := reflect.MapOf(keyRT, tbl.typ)
		mapOfValRV := reflect.MakeMap(mapT)
		for rset.Next() {
			var keyRV reflect.Value
			valRV := reflect.New(tbl.typ).Elem()
			for n, col := range tbl.cols {
				f := valRV.Field(col.fieldIdx)
				fv := reflect.ValueOf(rset.Row[n])
				f.Set(fv)
				switch rt {
				case MapOfValPk:
					if col.attr&pk != 0 { // validation ensured only one field is marked with `pk`
						keyRV = fv
					}
				case MapOfValFk1:
					if col.attr&fk1 != 0 { // validation ensured only one field is marked with `fk1`
						keyRV = fv
					}
				case MapOfValFk2:
					if col.attr&fk2 != 0 { // validation ensured only one field is marked with `fk2`
						keyRV = fv
					}
				case MapOfValFk3:
					if col.attr&fk3 != 0 { // validation ensured only one field is marked with `fk3`
						keyRV = fv
					}
				case MapOfValFk4:
					if col.attr&fk4 != 0 { // validation ensured only one field is marked with `fk4`
						keyRV = fv
					}
				}
			}
			mapOfValRV.SetMapIndex(keyRV, valRV)
		}
		result = mapOfValRV.Interface()
	}
	return result, nil
}

// AddTbl maps a table name to a struct type when a struct type name is not
// identitcal to an Oracle table name.
//
// AddTbl is optional and used by the orm-like methods ora.Ins, ora.Upd,
// ora.Del, and ora.Sel.
//
// AddTbl may be called once during the lifetime of the driver.
func AddTbl(v interface{}, tblName string) (err error) {
	_drv.addTblMu.Lock()
	defer _drv.addTblMu.Unlock()
	typ, err := finalType(v)
	if err != nil {
		return errE(err)
	}
	logF(_drv.cfg.Log.AddTbl, "%v to %v", typ.Name(), tblName)
	_, err = tblCreate(typ, strings.ToUpper(tblName))
	if err != nil {
		return errE(err)
	}
	return nil
}

func tblGet(v interface{}) (tbl *tbl, err error) {
	defer func() {
		if value := recover(); value != nil {
			err = errR(value)
		}
	}()
	typ, err := finalType(v)
	if err != nil {
		return nil, err
	}
	tbl, ok := tbls[typ.Name()]
	if !ok {
		tbl, err = tblCreate(typ, "") // create tbl
		if err != nil {
			return nil, err
		}
	}
	return tbl, nil
}

func finalType(v interface{}) (rt reflect.Type, err error) {
	if v == nil {
		return nil, errors.New("Unable to determine type from nil value.")
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface { // get final type
		rv = rv.Elem()
	}
	return rv.Type(), nil
}

func finalValue(v interface{}) (rv reflect.Value, err error) {
	defer func() {
		if value := recover(); value != nil {
			err = errR(value)
		}
	}()
	if v == nil {
		return rv, errors.New("Unable to obtain final value from nil.")
	}
	rv = reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface { // get final value
		rv = rv.Elem()
	}
	return rv, nil
}

func tblCreate(typ reflect.Type, tblName string) (t *tbl, err error) {
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("Expected type of Struct, received type of %v.", typ.Kind())
	}
	t = &tbl{}
	t.typ = typ
	t.cols = make([]col, 0)
	if tblName == "" { // possible user passed in empty string for table name
		tblName = typ.Name()
	}
	t.name = strings.ToUpper(tblName)
Outer:
	for n := 0; n < typ.NumField(); n++ {
		f := typ.Field(n)
		if unicode.IsLower(rune(f.Name[0])) { // skip unexported fields
			continue
		}
		tag := f.Tag.Get("db")
		col := col{fieldIdx: n}
		if tag == "" { // no db tag; use field name
			col.name = f.Name
		} else {
			tagValues := strings.Split(tag, ",")
			for n := range tagValues {
				tagValues[n] = strings.ToLower(strings.Trim(tagValues[n], " "))
			}
			// check for ignore tag `-`
			for _, tagValue := range tagValues {
				if tagValue == "-" {
					continue Outer
				}
			}
			if len(tagValues) == 0 {
				return nil, fmt.Errorf("Struct '%v' field '%v' has `db` tag but no value.", typ.Name(), f.Name)
			} else {
				if tagValues[0] == "" { // may be empty string in case of `db:"id"`
					col.name = f.Name
				} else {
					col.name = tagValues[0]
				}
				// check for single `id`,`pk`,`fk1`,`fk2`,`fk3`,`fk4` field
				idCount := 0
				pkCount := 0
				fk1Count := 0
				fk2Count := 0
				fk3Count := 0
				fk4Count := 0
				for _, tagValue := range tagValues {
					if tagValue == "id" {
						col.attr |= id
						t.attr |= id
						idCount++
					} else if tagValue == "pk" {
						col.attr |= pk
						t.attr |= pk
						pkCount++
					} else if tagValue == "fk1" {
						col.attr |= fk1
						t.attr |= fk1
						fk1Count++
					} else if tagValue == "fk2" {
						col.attr |= fk2
						t.attr |= fk2
						fk2Count++
					} else if tagValue == "fk3" {
						col.attr |= fk3
						t.attr |= fk3
						fk3Count++
					} else if tagValue == "fk4" {
						col.attr |= fk4
						t.attr |= fk4
						fk4Count++
					}
				}
				if idCount > 1 {
					return nil, fmt.Errorf("Struct '%v' has more than one exported field marked with a `db:\"id\"` tag.", typ.Name())
				} else if pkCount > 1 {
					return nil, fmt.Errorf("Struct '%v' has more than one exported field marked with a `db:\"pk\"` tag.", typ.Name())
				} else if fk1Count > 1 {
					return nil, fmt.Errorf("Struct '%v' has more than one exported field marked with a `db:\"fk1\"` tag.", typ.Name())
				} else if fk2Count > 1 {
					return nil, fmt.Errorf("Struct '%v' has more than one exported field marked with a `db:\"fk2\"` tag.", typ.Name())
				} else if fk3Count > 1 {
					return nil, fmt.Errorf("Struct '%v' has more than one exported field marked with a `db:\"fk3\"` tag.", typ.Name())
				} else if fk4Count > 1 {
					return nil, fmt.Errorf("Struct '%v' has more than one exported field marked with a `db:\"fk4\"` tag.", typ.Name())
				}
			}
		}
		col.name = strings.ToUpper(col.name)
		col.gct = gct(f.Type)
		t.cols = append(t.cols, col)
	}
	// place pk field at last index for Ins, Upd
	// Ins optionally uses pk,id for RETURNING clause
	// Upd requires pk at end to specify WHERE clause
	if t.attr&pk != 0 {
		for n, col := range t.cols {
			if col.attr&pk != 0 && n != len(t.cols)-1 {
				t.cols = append(t.cols[:n], t.cols[n+1:]...) // remove id col
				t.cols = append(t.cols, col)                 // append id col
				break
			}
		}
	}
	if len(t.cols) == 0 {
		return nil, fmt.Errorf("Struct '%v' has no db columns.", typ.Name())
	}
	tbls[typ.Name()] = t // store tbl for future lookup
	return t, nil
}

func gct(rt reflect.Type) GoColumnType {
	switch rt.Kind() {
	case reflect.Bool:
		return B
	case reflect.String:
		return S
	case reflect.Array, reflect.Slice:
		name := rt.Elem().Name()
		if name == "uint8" || name == "byte" {
			return Bin
		}
	case reflect.Int64:
		return I64
	case reflect.Int32:
		return I32
	case reflect.Int16:
		return I16
	case reflect.Int8:
		return I8
	case reflect.Uint64:
		return U64
	case reflect.Uint32:
		return U32
	case reflect.Uint16:
		return U16
	case reflect.Uint8:
		return U8
	case reflect.Float64:
		return F64
	case reflect.Float32:
		return F32
	case reflect.Struct:
		name := rt.Name()
		switch rt.PkgPath() {
		case "time":
			if name == "Time" {
				return T
			}
		case "ora":
			switch name {
			case "OraI64":
				return OraI64
			case "OraI32":
				return OraI32
			case "OraI16":
				return OraI16
			case "OraI8":
				return OraI8
			case "OraU64":
				return OraU64
			case "OraU32":
				return OraU32
			case "OraU16":
				return OraU16
			case "OraU8":
				return OraU8
			case "OraF64":
				return OraF64
			case "OraF32":
				return OraF32
			case "OraT":
				return OraT
			case "OraS":
				return OraS
			case "OraB":
				return OraB
			case "OraBin":
				return OraBin
			}
		}
	}
	return D
}
