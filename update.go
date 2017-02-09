package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
)

// Startxref returns the value of startxref.
func (r *Reader) Startxref() int64 {
	return r.startxref
}

// Obj returns array of obj, or [] if not found
func (r *Reader) Obj(i int) []Value {
	var result []Value
	return recurse("trailer", r.Trailer(), 0, i, result)
}

// Xref returns the xref offset and generation of an obj
func (r *Reader) Xref(obj int) (int64, int) {
	if obj >= len(r.xref) {
		return 0, 65535
	}
	xref := r.xref[obj]
	if xref.ptr.id != uint32(obj) {
		return 0, 65535
	}
	return xref.offset, int(xref.ptr.gen)
}

// Append a value to an array.
func (v Value) Append(value Value) Value {
	x, ok := v.data.(array)
	if !ok {
		return Value{}
	}
	v.data = append(x, value.data)
	return v
}

// Obj returns obj number.
func (v Value) Obj() int {
	return int(v.ptr.id)
}

// SetMapIndex for PDF works like reflect.SetMapIndex
func (v Value) SetMapIndex(key string, value Value) error {
	x, ok := v.data.(dict)
	if !ok {
		strm, ok := v.data.(stream)
		if !ok {
			return errors.New("not a dict")
		}
		x = strm.hdr
	}
	null := Value{}
	if value == null {
		delete(x, name(key))
	} else {
		x[name(key)] = value.data
	}
	return nil
}

// Ustring is like String, but here strings are delimited with () instead of ""
// This makes them usable in PDF files.
// The Value might be home made (not from NewReader(), so check null value.
func (v Value) Ustring() string {
	null := Value{}
	if v == null {
		return ""
	}
	return uobjfmt(v.data)
}

// ValueArray returns (an empty) array as a Value
func ValueArray() Value {
	return Value{data: make(array, 0)}
}

// ValueDict returns (an empty) dict as a Value
func ValueDict() Value {
	return Value{data: make(dict)}
}

// ValueInt64 returns an int64 as a Value
func ValueInt64(a int64) Value {
	return Value{data: a}
}

// ValueName returns a name as a Value
func ValueName(a string) Value {
	return Value{data: name(a)}
}

// ValueObj returns an object as a Value
func ValueObj(obj, generation int) Value {
	return Value{data: objptr{uint32(obj), uint16(generation)}}
}

// ValueString returns a string as a Value
func ValueString(a string) Value {
	return Value{data: a}
}


// Internal functions

func recurse(akey string, x Value, visited, find int, found []Value) []Value {
	xobj := x.Obj()
	//fmt.Println("recurse", x, xobj, visited)
	// Not allowed to go back. Can create loops.
	if xobj < visited {
		// "Parent" is a know case. The others should be checked.
		if akey != "Parent" {
			log.Println(akey, "may not go back in PDF object graph to", x)
		}
		return found
	}
	if xobj == find {
		found = append(found, x)
		return found
	}
	switch x.Kind() {
	case Stream, Dict:
		for _, key := range x.Keys() {
			found = recurse(key, x.Key(key), xobj, find, found)
		}
		return found
	case Array:
		for i := 0; i < x.Len(); i++ {
			found = recurse(strconv.Itoa(i), x.Index(i), xobj, find, found)
		}
		return found
	default:
		return found
	}
}

func uobjfmt(x interface{}) string {
	switch x := x.(type) {
	default:
		return fmt.Sprint(x)
	case string:
		s := x
		if isPDFDocEncoded(x) {
			s = pdfDocDecode(x)
		}
		if isUTF16(x) {
			s = utf16Decode(x[2:])
		}
		return "(" + s + ")"
	case name:
		return "/" + string(x)
	case dict:
		var keys []string
		for k := range x {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		var buf bytes.Buffer
		buf.WriteString("<<")
		for i, k := range keys {
			elem := x[name(k)]
			if i > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString("/")
			buf.WriteString(k)
			buf.WriteString(" ")
			buf.WriteString(uobjfmt(elem))
		}
		buf.WriteString(">>")
		return buf.String()

	case array:
		var buf bytes.Buffer
		buf.WriteString("[")
		for i, elem := range x {
			if i > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(uobjfmt(elem))
		}
		buf.WriteString("]")
		return buf.String()

	case stream:
		return fmt.Sprintf("%v@%d", uobjfmt(x.hdr), x.offset)

	case objptr:
		return fmt.Sprintf("%d %d R", x.id, x.gen)

	case objdef:
		return fmt.Sprintf("{%d %d obj}%v", x.ptr.id, x.ptr.gen, uobjfmt(x.obj))
	}
}
