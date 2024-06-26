package core

import (
	. "otpgo/util"

	"fmt"

	dc "github.com/LittleToonCat/dcparser-go"
	lua "github.com/yuin/gopher-lua"
)

// DC wrappers for Lua

func UnpackDataToLuaValue(unpacker dc.DCPacker, L *lua.LState) lua.LValue {
	var value lua.LValue
	switch unpacker.Get_pack_type() {
	case dc.PT_invalid:
		value = lua.LNil
	case dc.PT_double:
		value = lua.LNumber(unpacker.Unpack_double().(float64))
	case dc.PT_int:
		value = lua.LNumber(unpacker.Unpack_int().(int))
	case dc.PT_uint:
		value = lua.LNumber(unpacker.Unpack_uint().(uint))
	case dc.PT_int64:
		value = NewLuaInt64(L, unpacker.Unpack_int64().(int64))
	case dc.PT_uint64:
		value = NewLuaUint64(L, unpacker.Unpack_uint64().(uint64))
	case dc.PT_string:
		fallthrough
	case dc.PT_blob:
		value = lua.LString(unpacker.Unpack_string().(string))
	default:
		// If we reached here, that means it is a list
		// of nested fields (e.g. an array type, an atomic field, a
		// class parameter, or a switch case).
		//
		// We'll have to create a table for these types.
		table := L.NewTable()
		unpacker.Push()
		for unpacker.More_nested_fields() {
			table.Append(UnpackDataToLuaValue(unpacker, L))
		}
		unpacker.Pop()
		value = table
	}

	return value
}

func PackLuaValue(packer dc.DCPacker, value lua.LValue) {
	switch packer.Get_pack_type() {
	case dc.PT_invalid:
	case dc.PT_double:
		fallthrough
	case dc.PT_int:
		fallthrough
	case dc.PT_uint:
		fallthrough
	case dc.PT_int64:
		fallthrough
	case dc.PT_uint64:
		switch value.Type() {
		case lua.LTNumber:
			packer.Pack_double(float64(value.(lua.LNumber)))
		case lua.LTUserData:
			if int64, ok := value.(*lua.LUserData).Value.(int64); ok {
				packer.Pack_int64(int64)
			} else if uint64, ok := value.(*lua.LUserData).Value.(uint64); ok {
				packer.Pack_uint64(uint64)
			}
		}
	case dc.PT_string:
		fallthrough
	case dc.PT_blob:
		if LString, ok := value.(lua.LString); ok {
			packer.Pack_string(string(LString))
		}
	default:
		if table, ok := value.(*lua.LTable); ok {
			packer.Push()
			table.ForEach(func(_, l2 lua.LValue) {
				PackLuaValue(packer, l2)
			})
			packer.Pop()
		}
	}
}

const luaDCFileType = "dcfile"

func RegisterDCFileType(L *lua.LState) {
	mt := L.NewTypeMetatable(luaDCFileType)
	L.SetGlobal(luaDCFileType, mt)
	// Methods
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), DCFileMethods))
}

const luaDCClassType = "dcclass"

func RegisterDCClassType(L *lua.LState) {
	mt := L.NewTypeMetatable(luaDCClassType)
	L.SetGlobal(luaDCClassType, mt)
	// Methods
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), DCClassMethods))
	L.SetField(mt, "__tostring", L.NewFunction(LuaClassToString))
}

const luaDCFieldType = "dcfield"

func RegisterDCFieldType(L *lua.LState) {
	mt := L.NewTypeMetatable(luaDCFieldType)
	L.SetGlobal(luaDCFieldType, mt)
	// Methods
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), DCFieldMethods))
	L.SetField(mt, "__tostring", L.NewFunction(LuaFieldToString))
}

func RegisterLuaDCTypes(L *lua.LState) {
	RegisterDCFileType(L)
	RegisterDCClassType(L)
	RegisterDCFieldType(L)
}

func NewLuaDCFile(L *lua.LState, dcFile dc.DCFile) *lua.LUserData {
	ud := L.NewUserData()
	ud.Value = dcFile
	L.SetMetatable(ud, L.GetTypeMetatable(luaDCFileType))
	return ud
}

func CheckDCFile(L *lua.LState, n int) dc.DCFile {
	ud := L.CheckUserData(n)
	if dc, ok := ud.Value.(dc.DCFile); ok {
		return dc
	}
	L.ArgError(n, "DCFile expected")
	return nil
}

func NewLuaDCClass(L *lua.LState, dclass dc.DCClass) *lua.LUserData {
	ud := L.NewUserData()
	ud.Value = dclass
	L.SetMetatable(ud, L.GetTypeMetatable(luaDCClassType))
	return ud
}

func CheckDCClass(L *lua.LState, n int) dc.DCClass {
	ud := L.CheckUserData(n)
	if dc, ok := ud.Value.(dc.DCClass); ok {
		return dc
	}
	L.ArgError(n, "DCClass expected")
	return nil
}

func NewLuaDCField(L *lua.LState, dcField dc.DCField) *lua.LUserData {
	ud := L.NewUserData()
	ud.Value = dcField
	L.SetMetatable(ud, L.GetTypeMetatable(luaDCFieldType))
	return ud
}

func CheckDCField(L *lua.LState, n int) dc.DCField {
	ud := L.CheckUserData(n)
	if dc, ok := ud.Value.(dc.DCField); ok {
		return dc
	}
	L.ArgError(n, "DCField expected")
	return nil
}

var DCFileMethods = map[string]lua.LGFunction{
	"getNumClasses": LuaGetNumClasses,
	"getClass": LuaGetClass,
	"getClassByName": LuaGetClassByName,
	"getFieldByIndex": LuaFileGetFieldByIndex,
}

func LuaGetNumClasses(L *lua.LState) int {
	dcFile := CheckDCFile(L, 1)
	L.Push(lua.LNumber(dcFile.Get_num_classes()))
	return 1
}

func LuaGetClass(L *lua.LState) int {
	dcFile := CheckDCFile(L, 1)
	cls := L.CheckInt(2)

	dclass := dcFile.Get_class(cls)

	if (dclass == dc.SwigcptrDCClass(0)) {
		L.ArgError(2, fmt.Sprintf("Could not find class with index %d", cls))
		return 0
	}

	L.Push(NewLuaDCClass(L, dclass))
	return 1
}

func LuaGetClassByName(L *lua.LState) int {
	dcFile := CheckDCFile(L, 1)
	cls := L.CheckString(2)

	dclass := dcFile.Get_class_by_name(cls)

	if dclass == dc.SwigcptrDCClass(0) {
		L.ArgError(2, fmt.Sprintf("Could not find class with name \"%s\"", cls))
		return 0
	}

	L.Push(NewLuaDCClass(L, dclass))
	return 1
}

func LuaFileGetFieldByIndex(L *lua.LState) int {
	dcFile := CheckDCFile(L, 1)
	index := L.CheckInt(2)

	dcField := dcFile.Get_field_by_index(index)
	if dcField == dc.SwigcptrDCField(0) {
		L.ArgError(2, fmt.Sprintf("Could not find field with index %d", index))
		return 0
	}

	L.Push(NewLuaDCField(L, dcField))
	return 1
}

var DCClassMethods = map[string]lua.LGFunction {
	"getName": LuaGetClassName,
	"getNumber": LuaGetClassNumber,
	"getNumParents": LuaGetNumParents,
	"getParent": LuaGetParent,
	"getNumFields": LuaGetNumFields,
	"getField": LuaGetField,
	"getFieldByIndex": LuaClassGetFieldByIndex,
	"getFieldByName": LuaGetFieldByName,
}

func LuaClassToString(L *lua.LState) int {
	dclass := CheckDCClass(L, 1)

	if dclass.Is_struct() {
		L.Push(lua.LString(fmt.Sprintf("struct %s", dclass.Get_name())))
	} else {
		L.Push(lua.LString(fmt.Sprintf("dclass %s", dclass.Get_name())))
	}

	return 1
}

func LuaGetClassName(L *lua.LState) int {
	dclass := CheckDCClass(L, 1)

	L.Push(lua.LString(dclass.Get_name()))
	return 1
}

func LuaGetClassNumber(L *lua.LState) int {
	dclass := CheckDCClass(L, 1)

	L.Push(lua.LNumber(dclass.Get_number()))
	return 1
}

func LuaGetNumParents(L *lua.LState) int {
	dclass := CheckDCClass(L, 1)

	L.Push(lua.LNumber(dclass.Get_num_parents()))
	return 1
}

func LuaGetParent(L *lua.LState) int {
	dclass := CheckDCClass(L, 1)
	n := L.CheckInt(2)

	parentClass := dclass.Get_parent(n)
	if (parentClass == dc.SwigcptrDCClass(0)) {
		L.ArgError(2, fmt.Sprintf("Could not find parent class with index %d", n))
		return 0
	}

	L.Push(NewLuaDCClass(L, parentClass))
	return 1
}

func LuaGetNumFields(L *lua.LState) int {
	dclass := CheckDCClass(L, 1)

	L.Push(lua.LNumber(dclass.Get_num_fields()))
	return 1
}

func LuaGetField(L *lua.LState) int {
	dcClass := CheckDCClass(L, 1)
	n := L.CheckInt(2)

	dcField := dcClass.Get_field(n)
	if dcField == dc.SwigcptrDCField(0) {
		L.ArgError(2, fmt.Sprintf("Could not find field %d", n))
		return 0
	}

	L.Push(NewLuaDCField(L, dcField))
	return 1
}

func LuaClassGetFieldByIndex(L *lua.LState) int {
	dcClass := CheckDCClass(L, 1)
	index := L.CheckInt(2)

	dcField := dcClass.Get_field_by_index(index)
	if dcField == dc.SwigcptrDCField(0) {
		L.ArgError(2, fmt.Sprintf("Could not find field with index %d", index))
		return 0
	}

	L.Push(NewLuaDCField(L, dcField))
	return 1
}

func LuaGetFieldByName(L *lua.LState) int {
	dcClass := CheckDCClass(L, 1)
	name := L.CheckString(2)

	dcField := dcClass.Get_field_by_name(name)
	if dcField == dc.SwigcptrDCField(0) {
		L.ArgError(2, fmt.Sprintf("Could not find field with name \"%s\"", name))
		return 0
	}

	L.Push(NewLuaDCField(L, dcField))
	return 1
}

var DCFieldMethods = map[string]lua.LGFunction {
	"getName": LuaGetFieldName,
	"getNumber": LuaGetFieldNumber,
	"getClass": LuaFieldGetClass,
	"hasKeyword": LuaHasKeyword,
}

func LuaFieldToString(L *lua.LState) int {
	dcField := CheckDCField(L, 1)

	L.Push(lua.LString(fmt.Sprintf("DCField %s", dcField.Get_name())))
	return 1
}

func LuaGetFieldName(L *lua.LState) int {
	dcField := CheckDCField(L, 1)

	L.Push(lua.LString(dcField.Get_name()))
	return 1
}

func LuaGetFieldNumber(L *lua.LState) int {
	dcField := CheckDCField(L, 1)

	L.Push(lua.LNumber(dcField.Get_number()))
	return 1
}

func LuaFieldGetClass(L *lua.LState) int {
	dcField := CheckDCField(L, 1)

	L.Push(NewLuaDCClass(L, dcField.Get_class()))
	return 1
}

func LuaHasKeyword(L *lua.LState) int {
	dcField := CheckDCField(L, 1)
	keyword := L.CheckString(2)

	L.Push(lua.LBool(dcField.Has_keyword(keyword)))
	return 1
}
