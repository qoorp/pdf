package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
)

// Startxref returns the value of startxref.
func (r *Reader) Startxref() int64 {
	return r.startxref
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
	zero := Value{}
	if value == zero {
		delete(x, name(key))
	} else {
		x[name(key)] = value.data
	}
	return nil
}

// Ustring is like String, but here strings are delimited with () instead of ""
// This makes them usable in PDF files.
func (v Value) Ustring() string {
	return uobjfmt(v.data)
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

// ValueString returns a string as a Value
func ValueString(a string) Value {
	return Value{data: a}
}


// Internal functions

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
