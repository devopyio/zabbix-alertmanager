package zabbixutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Converts value to kind. Panics if it can't be done.
type Converter func(value interface{}, kind reflect.Kind) interface{}

// Converter: requires value to be exactly of specified kind.
func NoConvert(value interface{}, kind reflect.Kind) interface{} {
	switch kind {
	case reflect.Bool:
		return value.(bool)

	case reflect.Int:
		return int64(value.(int))
	case reflect.Int8:
		return int64(value.(int8))
	case reflect.Int16:
		return int64(value.(int16))
	case reflect.Int32:
		return int64(value.(int32))
	case reflect.Int64:
		return value.(int64)

	case reflect.Uint:
		return uint64(value.(uint))
	case reflect.Uint8:
		return uint64(value.(uint8))
	case reflect.Uint16:
		return uint64(value.(uint16))
	case reflect.Uint32:
		return uint64(value.(uint32))
	case reflect.Uint64:
		return value.(uint64)
	case reflect.Uintptr:
		return uint64(value.(uintptr))

	case reflect.Float32:
		return float64(value.(float32))
	case reflect.Float64:
		return value.(float64)

	case reflect.String:
		return value.(string)
	}

	panic(fmt.Errorf("NoConvert: can't convert %#v to %s", value, kind))
}

// Converter: uses strconv.Parse* functions.
func Strconv(value interface{}, kind reflect.Kind) interface{} {
	err := fmt.Errorf("Strconv: can't convert %#v to %s", value, kind)
	s := fmt.Sprint(value)

	switch kind {
	case reflect.Bool:
		res, err := strconv.ParseBool(s)
		if err != nil {
			panic(err)
		}
		return res

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		res, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			panic(err)
		}
		return res

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		res, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			panic(err)
		}
		return res

	case reflect.Float32, reflect.Float64:
		res, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic(err)
		}
		return res

	case reflect.String:
		return s
	}

	panic(err)
}

// Converts a struct to map.
// First argument is a pointer to struct.
// Second argument is a not-nil map which will be modified.
// Only exported struct fields are used. Pointers will be followed, nils will be present.
// Tag may be used to change mapping between struct field and map key.
// Currently supports bool, ints, uints, floats, strings and pointer to them.
// Panics in case of error.
func StructToMap(StructPointer interface{}, Map map[string]interface{}, tag string) {
	structPointerType := reflect.TypeOf(StructPointer)
	if structPointerType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("StructToMap: expected pointer to struct as first argument, got %s", structPointerType.Kind()))
	}

	structType := structPointerType.Elem()
	if structType.Kind() != reflect.Struct {
		panic(fmt.Errorf("StructToMap: expected pointer to struct as first argument, got pointer to %s", structType.Kind()))
	}

	s := reflect.ValueOf(StructPointer).Elem()

	var name string
	for i := 0; i < structType.NumField(); i++ {
		stf := structType.Field(i)
		if stf.PkgPath != "" {
			continue
		}

		name = ""
		if tag != "" {
			name = strings.Split(stf.Tag.Get(tag), ",")[0]
			if name == "-" {
				continue
			}
		}
		if name == "" {
			name = stf.Name
		}

		f := s.Field(i)
		if f.Kind() == reflect.Ptr {
			if f.IsNil() {
				Map[name] = nil
				continue
			}
			f = f.Elem()
		}
		Map[name] = f.Interface()
	}
}

// Converts a struct to map. Uses StructToMap().
// First argument is a struct.
// Second argument is a not-nil map which will be modified.
// Only exported struct fields are used. Pointers will be followed, nils will be present.
// Tag may be used to change mapping between struct field and map key.
// Currently supports bool, ints, uints, floats, strings and pointer to them.
// Panics in case of error.
func StructValueToMap(Struct interface{}, Map map[string]interface{}, tag string) {
	structType := reflect.TypeOf(Struct)
	if structType.Kind() != reflect.Struct {
		panic(fmt.Errorf("StructValueToMap: expected struct as first argument, got %s", structType.Kind()))
	}

	v := reflect.New(reflect.TypeOf(Struct))
	v.Elem().Set(reflect.ValueOf(Struct))
	StructToMap(v.Interface(), Map, tag)
}

// Converts a slice of structs to a slice of maps. Uses StructValueToMap().
// First argument is a slice of structs.
// Second argument is a pointer to (possibly nil) slice of maps which will be set.
func StructsToMaps(Structs interface{}, Maps *[]map[string]interface{}, tag string) {
	sliceType := reflect.TypeOf(Structs)
	if sliceType.Kind() != reflect.Slice {
		panic(fmt.Errorf("Expected slice of structs as first argument, got %s", sliceType.Kind()))
	}

	structType := sliceType.Elem()
	if structType.Kind() != reflect.Struct {
		panic(fmt.Errorf("Expected slice of structs as first argument, got slice of %s", structType.Kind()))
	}

	structs := reflect.ValueOf(Structs)
	l := structs.Len()
	maps := reflect.MakeSlice(reflect.TypeOf([]map[string]interface{}{}), 0, l)

	for i := 0; i < l; i++ {
		m := make(map[string]interface{})
		StructValueToMap(structs.Index(i).Interface(), m, tag)
		maps = reflect.Append(maps, reflect.ValueOf(m))
	}

	reflect.ValueOf(Maps).Elem().Set(maps)
}

// Converts a map to struct using converter function.
// First argument is a map.
// Second argument is a not-nil pointer to struct which will be modified.
// Only exported struct fields are set. Omitted or extra values in map are ignored. Pointers will be set.
// Tag may be used to change mapping between struct field and map key.
// Currently supports bool, ints, uints, floats, strings and pointer to them.
// Panics in case of error.
func MapToStruct(Map map[string]interface{}, StructPointer interface{}, converter Converter, tag string) {
	structPointerType := reflect.TypeOf(StructPointer)
	if structPointerType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("MapToStruct: expected pointer to struct as second argument, got %s", structPointerType.Kind()))
	}

	structType := structPointerType.Elem()
	if structType.Kind() != reflect.Struct {
		panic(fmt.Errorf("MapToStruct: expected pointer to struct as second argument, got pointer to %s", structType.Kind()))
	}
	s := reflect.ValueOf(StructPointer).Elem()

	var name string
	defer func() {
		e := recover()
		if e == nil {
			return
		}

		panic(fmt.Errorf("MapToStruct, field %s: %s", name, e))
	}()

	for i := 0; i < structType.NumField(); i++ {
		f := s.Field(i)
		if !f.CanSet() {
			continue
		}

		stf := structType.Field(i)
		name = ""
		if tag != "" {
			name = strings.Split(stf.Tag.Get(tag), ",")[0]
			if name == "-" {
				continue
			}
		}
		if name == "" {
			name = stf.Name
		}
		v, ok := Map[name]
		if !ok {
			continue
		}

		var fp reflect.Value
		kind := f.Kind()
		if kind == reflect.Ptr {
			t := f.Type().Elem()
			kind = t.Kind()
			fp = reflect.New(t)
			f = fp.Elem()
		}

		switch kind {
		case reflect.Bool:
			f.SetBool(converter(v, kind).(bool))

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(converter(v, kind).(int64))

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			f.SetUint(converter(v, kind).(uint64))

		case reflect.Float32, reflect.Float64:
			f.SetFloat(converter(v, kind).(float64))

		case reflect.String:
			f.SetString(converter(v, kind).(string))

		default:
			// not implemented
		}

		if fp.IsValid() {
			s.Field(i).Set(fp)
		}
	}

	return
}

// Converts a slice of maps to a slice of structs. Uses MapToStruct().
// First argument is a slice of maps.
// Second argument is a pointer to (possibly nil) slice of structs which will be set.
func MapsToStructs(Maps []map[string]interface{}, SlicePointer interface{}, converter Converter, tag string) {
	slicePointerType := reflect.TypeOf(SlicePointer)
	if slicePointerType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("MapsToStructs: expected pointer to slice of structs as second argument, got %s", slicePointerType.Kind()))
	}

	sliceType := slicePointerType.Elem()
	if sliceType.Kind() != reflect.Slice {
		panic(fmt.Errorf("MapsToStructs: expected pointer to slice of structs as second argument, got pointer to %s", sliceType.Kind()))
	}

	structType := sliceType.Elem()
	if structType.Kind() != reflect.Struct {
		panic(fmt.Errorf("MapsToStructs: expected pointer to slice of structs as second argument, got pointer to slice of %s", structType.Kind()))
	}

	slice := reflect.MakeSlice(sliceType, 0, len(Maps))
	for _, m := range Maps {
		s := reflect.New(structType)
		MapToStruct(m, s.Interface(), converter, tag)
		slice = reflect.Append(slice, s.Elem())
	}
	reflect.ValueOf(SlicePointer).Elem().Set(slice)
}

// Variant of MapsToStructs() with relaxed signature.
func MapsToStructs2(Maps []interface{}, SlicePointer interface{}, converter Converter, tag string) {
	m := make([]map[string]interface{}, len(Maps))
	for index, i := range Maps {
		m[index] = i.(map[string]interface{})
	}
	MapsToStructs(m, SlicePointer, converter, tag)
}
