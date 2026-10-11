package interp

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/big"
	"time"

	"eyelang/ir"
)

// encode saves a value as bytes: entity state, job arguments and recorded
// steps are stored this way. Functions can't be saved.
func (in *Interp) encode(v Value, at ir.Pos) []byte {
	var b bytes.Buffer
	in.writeValue(&b, v, at)
	return b.Bytes()
}

func putUvarint(b *bytes.Buffer, n uint64) {
	var buf [binary.MaxVarintLen64]byte
	b.Write(buf[:binary.PutUvarint(buf[:], n)])
}

func putVarint(b *bytes.Buffer, n int64) {
	var buf [binary.MaxVarintLen64]byte
	b.Write(buf[:binary.PutVarint(buf[:], n)])
}

func putString(b *bytes.Buffer, s string) {
	putUvarint(b, uint64(len(s)))
	b.WriteString(s)
}

func (in *Interp) writeValue(b *bytes.Buffer, v Value, at ir.Pos) {
	switch x := v.(type) {
	case int64:
		b.WriteByte('i')
		putVarint(b, x)
	case *big.Int:
		b.WriteByte('I')
		putString(b, x.String())
	case float64:
		b.WriteByte('f')
		putUvarint(b, math.Float64bits(x))
	case string:
		b.WriteByte('s')
		putString(b, x)
	case bool:
		if x {
			b.WriteByte('T')
		} else {
			b.WriteByte('F')
		}
	case rune:
		b.WriteByte('c')
		putVarint(b, int64(x))
	case ir.Unit:
		b.WriteByte('u')
	case ir.Duration:
		b.WriteByte('d')
		putVarint(b, int64(x))
	case ir.Size:
		b.WriteByte('z')
		putVarint(b, int64(x))
	case ir.Date:
		b.WriteByte('D')
		putVarint(b, int64(x.Year))
		putVarint(b, int64(x.Month))
		putVarint(b, int64(x.Day))
	case ir.TimeOfDay:
		b.WriteByte('t')
		putVarint(b, int64(x.Hour))
		putVarint(b, int64(x.Minute))
		putVarint(b, int64(x.Second))
	case time.Time:
		b.WriteByte('W')
		putVarint(b, x.UnixNano())
		putString(b, x.Location().String())
	case *List:
		b.WriteByte('L')
		in.writeValues(b, x.Elems, at)
	case *Set:
		b.WriteByte('S')
		in.writeValues(b, x.Elems, at)
	case *Tuple:
		b.WriteByte('P')
		in.writeValues(b, x.Elems, at)
	case *Map:
		b.WriteByte('M')
		in.writeValues(b, x.Keys, at)
		in.writeValues(b, x.Vals, at)
	case *Record:
		// A record of a declared type is saved with its type's name, so it
		// keeps its Show and operators when it is read back.
		if x.Type != nil {
			b.WriteByte('N')
			putString(b, typeName(x.Type))
		} else {
			b.WriteByte('R')
		}
		putUvarint(b, uint64(len(x.Names)))
		for i, n := range x.Names {
			putString(b, n)
			in.writeValue(b, x.Vals[i], at)
		}
	case Empty:
		b.WriteByte('E')
	case *Variant:
		b.WriteByte('V')
		putString(b, variantName(x.V))
		in.writeValues(b, x.Vals, at)
	case *EntityRef:
		b.WriteByte('e')
		putString(b, entityName(x.Entity))
		in.writeValue(b, x.ID, at)
	case *RunVal:
		b.WriteByte('r')
		putVarint(b, x.ID)
	case *ApprovalVal:
		b.WriteByte('A')
		putString(b, x.What)
		putString(b, x.By)
	case *RangeVal:
		b.WriteByte('g')
		var flags byte
		if x.Inclusive {
			flags |= 1
		}
		if x.Start != nil {
			flags |= 2
		}
		if x.End != nil {
			flags |= 4
		}
		b.WriteByte(flags)
		if x.Start != nil {
			in.writeValue(b, x.Start, at)
		}
		if x.End != nil {
			in.writeValue(b, x.End, at)
		}
	default:
		in.faultf(at, "%s can't be saved", kindName(v))
	}
}

func (in *Interp) writeValues(b *bytes.Buffer, vs []Value, at ir.Pos) {
	putUvarint(b, uint64(len(vs)))
	for _, v := range vs {
		in.writeValue(b, v, at)
	}
}

// decode reads a value saved by encode.
func (in *Interp) decode(data []byte, at ir.Pos) Value {
	r := bytes.NewReader(data)
	v := in.readValue(r, at)
	if r.Len() != 0 {
		in.faultf(at, "saved value has %d bytes left over", r.Len())
	}
	return v
}

func (in *Interp) readString(r *bytes.Reader, at ir.Pos) string {
	n, err := binary.ReadUvarint(r)
	if err != nil || n > uint64(r.Len()) {
		in.faultf(at, "corrupt saved value")
	}
	buf := make([]byte, n)
	_, _ = r.Read(buf)
	return string(buf)
}

func (in *Interp) readVarint(r *bytes.Reader, at ir.Pos) int64 {
	n, err := binary.ReadVarint(r)
	if err != nil {
		in.faultf(at, "corrupt saved value")
	}
	return n
}

func (in *Interp) readValues(r *bytes.Reader, at ir.Pos) []Value {
	n, err := binary.ReadUvarint(r)
	if err != nil || n > uint64(r.Len()) {
		in.faultf(at, "corrupt saved value")
	}
	vs := make([]Value, n)
	for i := range vs {
		vs[i] = in.readValue(r, at)
	}
	return vs
}

func (in *Interp) readValue(r *bytes.Reader, at ir.Pos) Value {
	tag, err := r.ReadByte()
	if err != nil {
		in.faultf(at, "corrupt saved value")
	}
	switch tag {
	case 'i':
		return in.readVarint(r, at)
	case 'I':
		v, _ := new(big.Int).SetString(in.readString(r, at), 10)
		return normInt(v)
	case 'f':
		n, _ := binary.ReadUvarint(r)
		return math.Float64frombits(n)
	case 's':
		return in.readString(r, at)
	case 'T':
		return true
	case 'F':
		return false
	case 'c':
		return rune(in.readVarint(r, at))
	case 'u':
		return ir.Unit{}
	case 'd':
		return ir.Duration(in.readVarint(r, at))
	case 'z':
		return ir.Size(in.readVarint(r, at))
	case 'D':
		return ir.Date{Year: int(in.readVarint(r, at)), Month: int(in.readVarint(r, at)), Day: int(in.readVarint(r, at))}
	case 't':
		return ir.TimeOfDay{Hour: int(in.readVarint(r, at)), Minute: int(in.readVarint(r, at)), Second: int(in.readVarint(r, at))}
	case 'W':
		ns := in.readVarint(r, at)
		loc, err := loadLocation(in.readString(r, at))
		if err != nil {
			loc = time.UTC
		}
		return time.Unix(0, ns).In(loc)
	case 'L':
		return &List{Elems: in.readValues(r, at)}
	case 'S':
		return newSet(in.readValues(r, at))
	case 'P':
		return &Tuple{Elems: in.readValues(r, at)}
	case 'M':
		keys := in.readValues(r, at)
		vals := in.readValues(r, at)
		return newMap(keys, vals)
	case 'R', 'N':
		rec := &Record{}
		if tag == 'N' {
			// A type the program no longer declares leaves the record
			// without one; its fields are still read.
			rec.Type = in.types[in.readString(r, at)]
		}
		n, _ := binary.ReadUvarint(r)
		for i := uint64(0); i < n; i++ {
			rec.Names = append(rec.Names, in.readString(r, at))
			rec.Vals = append(rec.Vals, in.readValue(r, at))
		}
		return rec
	case 'E':
		return Empty{}
	case 'V':
		name := in.readString(r, at)
		v, ok := in.variants[name]
		if !ok {
			in.faultf(at, "saved value names %s, which the program no longer declares", name)
		}
		return &Variant{V: v, Vals: in.readValues(r, at)}
	case 'e':
		name := in.readString(r, at)
		e, ok := in.entities[name]
		if !ok {
			in.faultf(at, "saved value names entity %s, which the program no longer declares", name)
		}
		return &EntityRef{Entity: e, ID: in.readValue(r, at)}
	case 'r':
		return &RunVal{ID: in.readVarint(r, at)}
	case 'A':
		return &ApprovalVal{What: in.readString(r, at), By: in.readString(r, at)}
	case 'g':
		flags, _ := r.ReadByte()
		rv := &RangeVal{Inclusive: flags&1 != 0}
		if flags&2 != 0 {
			rv.Start = in.readValue(r, at)
		}
		if flags&4 != 0 {
			rv.End = in.readValue(r, at)
		}
		return rv
	}
	in.faultf(at, "corrupt saved value")
	return nil
}
