package cbor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"reflect"
	"sort"

	"github.com/x448/float16"
)

const (
	cborFalse byte = 0xf4
	cborTrue  byte = 0xf5
	cborNil   byte = 0xf6
)

var (
	cborNaN              = []byte{0xf9, 0x7e, 0x00}
	cborPositiveInfinity = []byte{0xf9, 0x7c, 0x00}
	cborNegativeInfinity = []byte{0xf9, 0xfc, 0x00}
)

// ModeNaN specifies how to encode NaN.
type ModeNaN int

const (
	// ModeNaN7e00 always encodes NaN to 0xf97e00 (CBOR float16 = 0x7e00).
	ModeNaN7e00 ModeNaN = iota

	// ModeNaNNone never modifies or converts NaN to other representations
	// (float64 NaN stays float64, etc. even if it can use float16 without losing
	// any bits).
	ModeNaNNone
)

// ModeInf specifies how to encode Infinity and overrides ModeFloat.
// ModeFloat is not used for encoding Infinity and NaN values.
type ModeInf int

const (
	// ModeInfFloat16 always converts Inf to lossless IEEE binary16 (float16).
	ModeInfFloat16 ModeInf = iota

	// ModeInfNone never converts (used by CTAP2 Canonical CBOR).
	ModeInfNone
)

// ModeFloat specifies which floating-point format should
// be used as the shortest possible format for CBOR encoding.
// It is not used for encoding Infinity and NaN values.
type ModeFloat int

const (
	// ModeFloat16 specifies float16 as the shortest form that preserves value.
	// E.g. if float64 can convert to float32 while preserving value, then
	// encoding will also try to convert float32 to float16.  So a float64 might
	// encode as CBOR float64, float32 or float16 depending on the value.
	ModeFloat16 ModeFloat = iota

	// ModeFloatNone makes float values encode without any conversion.
	// E.g. a float32 in Go will encode to CBOR float32.  And
	// a float64 in Go will encode to CBOR float64.
	ModeFloatNone
)

// ModeSort identifies supported sorting order.
type ModeSort int

const (
	// ModeSortLengthFirst causes map keys or struct fields to be sorted such that:
	//     - If two keys have different lengths, the shorter one sorts earlier;
	//     - If two keys have the same length, the one with the lower value in
	//       (byte-wise) lexical order sorts earlier.
	// It is used in "Canonical CBOR" encoding in RFC 7049 3.9.
	ModeSortLengthFirst ModeSort = iota

	// ModeSortBytewiseLexical causes map keys or struct fields to be sorted in the
	// bytewise lexicographic order of their deterministic CBOR encodings.
	// It is used in "CTAP2 Canonical CBOR" and "Core Deterministic Encoding"
	// in RFC 7049bis.
	ModeSortBytewiseLexical

	// ModeSortNone means no sorting.
	ModeSortNone
)

func Marshal(v interface{}) ([]byte, error) {
	var b Builder
	b.Marshal(v)
	return b.Bytes()
}

// BuilderContinuation is a continuation-passing interface
// for building length-prefixed byte sequences.
type BuilderContinuation func(*Builder)

type Builder struct {
	ModeNaN   ModeNaN
	ModeInf   ModeInf
	ModeFloat ModeFloat
	ModeSort  ModeSort
	err       error
	result    []byte
	offsets   []mapItem
	tmp       []byte
	mapSize   int
}

func NewBuilder(buffer []byte) *Builder {
	return &Builder{
		result: buffer,
	}
}

// SetError sets the value to be returned as the error from Bytes. Writes
// performed after calling SetError are ignored.
func (b *Builder) SetError(err error) {
	b.err = err
}

// Bytes returns the bytes written by the builder or an error if one has
// occurred during building.
func (b *Builder) Bytes() ([]byte, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.result, nil
}

func (b *Builder) Len() int {
	return len(b.result)
}

func (b *Builder) add(bytes ...byte) {
	if b.err != nil {
		return
	}
	if len(b.result)+len(bytes) < len(bytes) {
		b.err = errors.New("cbor: length overflow")
	}
	b.result = append(b.result, bytes...)
}

func (b *Builder) addUnknown(t byte, fn BuilderContinuation) {
	offset := b.Len()
	b.addUint8(t, 0)
	fn(b)
	length := b.Len() - offset - 1
	if length <= 23 {
		b.result[offset] = t | byte(length)
	} else {
		if length <= math.MaxUint8 {
			b.add(0)
			copy(b.result[offset+1+1:], b.result[offset+1:])
			b.result[offset] = t | byte(24)
			b.result[offset+1] = byte(length)
		} else if length <= math.MaxUint16 {
			b.add(0, 0)
			copy(b.result[offset+1+2:], b.result[offset+1:])
			b.result[offset] = t | byte(25)
			binary.BigEndian.PutUint16(b.result[offset+1:], uint16(length))
		} else if length <= math.MaxUint32 {
			b.add(0, 0, 0, 0)
			copy(b.result[offset+1+4:], b.result[offset+1:])
			b.result[offset] = t | byte(26)
			binary.BigEndian.PutUint32(b.result[offset+1:], uint32(length))
		} else {
			b.add(0, 0, 0, 0, 0, 0, 0, 0)
			copy(b.result[offset+1+8:], b.result[offset+1:])
			b.result[offset] = t | byte(27)
			binary.BigEndian.PutUint64(b.result[offset+1:], uint64(length))
		}
	}
}

func (b *Builder) Marshal(v interface{}) {
	if b.err != nil {
		return
	}
	switch v := v.(type) {
	case nil:
		b.AddNil()
	case *bool:
		if v == nil {
			b.AddNil()
		} else {
			b.AddBool(*v)
		}
	case bool:
		b.AddBool(v)
	case []bool:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddBool(x)
				}
			})
		}
	case *int8:
		if v == nil {
			b.AddNil()
		} else {
			b.AddInt8(*v)
		}
	case int8:
		b.AddInt8(v)
	case []int8:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddInt8(x)
				}
			})
		}
	case *uint8:
		if v == nil {
			b.AddNil()
		} else {
			b.AddUint8(*v)
		}
	case uint8:
		b.AddUint8(v)
	case []uint8:
		if v == nil {
			b.AddNil()
		} else {
			b.AddBytes(v)
		}
	case *int16:
		if v == nil {
			b.AddNil()
		} else {
			b.AddInt16(*v)
		}
	case int16:
		b.AddInt16(v)
	case []int16:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddInt16(x)
				}
			})
		}
	case *uint16:
		if v == nil {
			b.AddNil()
		} else {
			b.AddUint16(*v)
		}
	case uint16:
		b.AddUint16(v)
	case []uint16:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddUint16(x)
				}
			})
		}
	case *int32:
		if v == nil {
			b.AddNil()
		} else {
			b.AddInt32(*v)
		}
	case int32:
		b.AddInt32(v)
	case []int32:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddInt32(x)
				}
			})
		}
	case *uint32:
		if v == nil {
			b.AddNil()
		} else {
			b.AddUint32(*v)
		}
	case uint32:
		b.AddUint32(v)
	case []uint32:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddUint32(x)
				}
			})
		}
	case *int64:
		if v == nil {
			b.AddNil()
		} else {
			b.AddInt64(*v)
		}
	case int64:
		b.AddInt64(v)
	case []int64:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddInt64(x)
				}
			})
		}
	case *uint64:
		if v == nil {
			b.AddNil()
		} else {
			b.AddUint64(*v)
		}
	case uint64:
		b.AddUint64(v)
	case []uint64:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddUint64(x)
				}
			})
		}
	case *int:
		if v == nil {
			b.AddNil()
		} else {
			b.AddInt(*v)
		}
	case int:
		b.AddInt(v)
	case []int:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddInt(x)
				}
			})
		}
	case *uint:
		if v == nil {
			b.AddNil()
		} else {
			b.AddUint(*v)
		}
	case uint:
		b.AddUint(v)
	case []uint:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddUint(x)
				}
			})
		}
	case *float32:
		if v == nil {
			b.AddNil()
		} else {
			b.AddFloat32(*v)
		}
	case float32:
		b.AddFloat32(v)
	case []float32:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddFloat32(x)
				}
			})
		}
	case *float64:
		if v == nil {
			b.AddNil()
		} else {
			b.AddFloat64(*v)
		}
	case float64:
		b.AddFloat64(v)
	case []float64:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.AddFloat64(x)
				}
			})
		}
	case *string:
		if v == nil {
			b.AddNil()
		} else {
			b.AddString(*v)
		}
	case string:
		b.AddString(v)
	case []interface{}:
		if v == nil {
			b.AddNil()
		} else {
			b.AddArray(uint64(len(v)), func(b *Builder) {
				for _, x := range v {
					b.Marshal(x)
				}
			})
		}
	case map[interface{}]interface{}:
		if v == nil {
			b.AddNil()
		} else {
			fn := b.AddMap(len(v))
			for k, v := range v {
				fn(func(b *Builder) {
					b.Marshal(k)
				}, func(b *Builder) {
					b.Marshal(v)
				})
			}
		}
	case MarshalingValue:
		if v == nil {
			b.AddNil()
		} else {
			if err := v.MarshalCBORValue(b); err != nil {
				b.SetError(err)
			}
		}
	default:
		// Fallback to reflect-based encoding.
		b.value(reflect.Indirect(reflect.ValueOf(v)))
	}
}

func (b *Builder) value(v reflect.Value) {
	if b.err != nil {
		return
	}
	k := v.Kind()
	if !v.IsValid() {
		b.AddNil()
		return
	}
	t := v.Type()
	switch t {
	case typeBigInt:
		vbi := v.Interface().(big.Int)
		sign := vbi.Sign()
		bi := new(big.Int).SetBytes(vbi.Bytes()) // bi is absolute value of v
		if sign < 0 {
			// For negative number, convert to CBOR encoded number (-v-1).
			bi.Sub(bi, big.NewInt(1))
		}
		if bi.IsUint64() {
			if sign >= 0 {
				b.addUint64(cborTypePositiveInt, bi.Uint64())
			} else {
				b.addUint64(cborTypeNegativeInt, bi.Uint64())
			}
			return
		}
		var tagNum uint64 = 2
		if sign < 0 {
			tagNum = 3
		}
		b.AddTag(tagNum)
		b.AddBytes(bi.Bytes())
		return
	}
	if reflect.PtrTo(t).Implements(typeMarshalingValue) {
		m, ok := v.Interface().(MarshalingValue)
		if !ok {
			pv := reflect.New(v.Type())
			pv.Elem().Set(v)
			m = pv.Interface().(MarshalingValue)
		}
		if err := m.MarshalCBORValue(b); err != nil {
			b.SetError(err)
		}
		return
	}
	switch k {
	case reflect.String:
		b.AddString(v.String())
	case reflect.Array, reflect.Slice:
		l := v.Len()
		if t.Elem().Kind() == reflect.Uint8 {
			if k == reflect.Slice && v.IsNil() {
				b.AddNil()
				break
			}
			if l == 0 {
				b.addUint8(cborTypeByteString, 0)
				break
			}
			b.addUint64(cborTypeByteString, uint64(l))
			for i := 0; i < l; i++ {
				b.add(byte(v.Index(i).Uint()))
			}

		} else {
			b.AddArray(uint64(l), func(b *Builder) {
				for i := 0; i < l; i++ {
					b.value(v.Index(i))
				}
			})
		}
	case reflect.Map:
		if v.IsNil() {
			b.AddNil()
			break
		}
		fn := b.AddMap(v.Len())
		iter := v.MapRange()
		for iter.Next() {
			fn(func(b *Builder) {
				b.value(iter.Key())
			}, func(b *Builder) {
				b.value(iter.Value())
			})
		}
	case reflect.Struct:
		t := v.Type()
		l := v.NumField()
		b.AddArray(uint64(l), func(b *Builder) {
			for i := 0; i < l; i++ {
				if v := v.Field(i); v.CanSet() || t.Field(i).Name != "_" {
					b.value(v)
				}
			}
		})

	case reflect.Bool:
		b.AddBool(v.Bool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.AddInt64(v.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		b.AddUint64(v.Uint())

	case reflect.Float32, reflect.Float64:
		switch v.Type().Kind() {
		case reflect.Float32:
			b.AddFloat32(float32(v.Float()))
		case reflect.Float64:
			b.AddFloat64(v.Float())
		}

	case reflect.Complex64, reflect.Complex128:
		b.AddArray(2, func(b *Builder) {
			switch v.Type().Kind() {
			case reflect.Complex64:
				x := v.Complex()
				b.AddFloat32(float32(real(x)))
				b.AddFloat32(float32(imag(x)))
			case reflect.Complex128:
				x := v.Complex()
				b.AddFloat64(float64(real(x)))
			}
		})
	case reflect.Interface:
		if v.IsNil() {
			b.AddNil()
			break
		}
		b.value(v.Elem())
	default:
		b.SetError(errors.New("cbor: invalid type" + v.String()))
	}
}

// AddValue calls MarshalCBORValue on v, passing a pointer to the builder to append to.
// If MarshalCBORValue returns an error, it is set on the Builder so that subsequent
// appends don't have an effect.
func (b *Builder) AddValue(v MarshalingValue) {
	err := v.MarshalCBORValue(b)
	if err != nil {
		b.err = err
	}
}

func (b *Builder) AddRawBytes(v []byte) {
	b.add(v...)
}

func (b *Builder) AddBool(v bool) {
	d := cborFalse
	if v {
		d = cborTrue
	}
	b.add(d)
}

func (b *Builder) addUint8(t uint8, v uint8) {
	if v <= 23 {
		b.add(t | v)
	} else {
		b.add(t|byte(24), v)
	}
}

func (b *Builder) addUint16(t uint8, v uint16) {
	if v <= math.MaxUint8 {
		b.addUint8(t, uint8(v))
	} else {
		b.add(t|byte(25), byte(v>>8), byte(v))
	}
}

func (b *Builder) addUint32(t uint8, v uint32) {
	if v <= math.MaxUint8 {
		b.addUint8(t, uint8(v))
	} else if v <= math.MaxUint16 {
		b.addUint16(t, uint16(v))
	} else {
		b.add(t|byte(26), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	}
}

func (b *Builder) addUint64(t uint8, v uint64) {
	if v <= math.MaxUint8 {
		b.addUint8(t, uint8(v))
	} else if v <= math.MaxUint16 {
		b.addUint16(t, uint16(v))
	} else if v <= math.MaxUint32 {
		b.addUint32(t, uint32(v))
	} else {
		b.add(
			t|byte(27),
			byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v),
		)
	}
}

func (b *Builder) AddInt8(v int8) {
	if v >= 0 {
		b.AddUint8(uint8(v))
	} else {
		b.addUint8(cborTypeNegativeInt, uint8(v*(-1)-1))
	}
}

func (b *Builder) AddInt16(v int16) {
	if v >= 0 {
		b.AddUint16(uint16(v))
	} else {
		b.addUint16(cborTypeNegativeInt, uint16(v*(-1)-1))
	}
}

func (b *Builder) AddInt32(v int32) {
	if v >= 0 {
		b.AddUint32(uint32(v))
	} else {
		b.addUint32(cborTypeNegativeInt, uint32(v*(-1)-1))
	}
}

func (b *Builder) AddInt64(v int64) {
	if v >= 0 {
		b.AddUint64(uint64(v))
	} else {
		b.addUint64(cborTypeNegativeInt, uint64(v*(-1)-1))
	}
}

func (b *Builder) AddInt(v int) {
	b.AddInt64(int64(v))
}

func (b *Builder) AddUint8(v uint8) {
	b.addUint8(cborTypePositiveInt, v)
}

func (b *Builder) AddUint16(v uint16) {
	b.addUint16(cborTypePositiveInt, v)
}

func (b *Builder) AddUint32(v uint32) {
	b.addUint32(cborTypePositiveInt, v)
}

func (b *Builder) AddUint64(v uint64) {
	b.addUint64(cborTypePositiveInt, v)
}

func (b *Builder) AddUint(v uint) {
	b.addUint64(cborTypePositiveInt, uint64(v))
}

func (b *Builder) addFloat16(v float16.Float16) {
	f := uint16(v)
	b.add(cborTypePrimitives|byte(25), byte(f>>8), byte(f))
}

func (b *Builder) addFloat32(v float32) {
	f := math.Float32bits(v)
	b.add(cborTypePrimitives|byte(26), byte(f>>24), byte(f>>16), byte(f>>8), byte(f))
}

func (b *Builder) addFloat64(v float64) {
	f := math.Float64bits(v)
	b.add(
		cborTypePrimitives|byte(27),
		byte(f>>56), byte(f>>48), byte(f>>40), byte(f>>32),
		byte(f>>24), byte(f>>16), byte(f>>8), byte(f),
	)
}

func (b *Builder) AddFloat32(v float32) {
	if math.IsNaN(float64(v)) {
		if b.ModeNaN == ModeNaN7e00 {
			b.add(cborNaN...)
			return
		}
	} else if math.IsInf(float64(v), 0) {
		if b.ModeInf == ModeInfFloat16 {
			if v > 0 {
				b.add(cborPositiveInfinity...)
			} else {
				b.add(cborNegativeInfinity...)
			}
			return
		}
	}
	if b.ModeFloat == ModeFloat16 {
		var f16 float16.Float16
		p := float16.PrecisionFromfloat32(v)
		if p == float16.PrecisionExact {
			// Roundtrip float32->float16->float32 test isn't needed.
			f16 = float16.Fromfloat32(v)
		} else if p == float16.PrecisionUnknown {
			// Try roundtrip float32->float16->float32 to determine if float32 can fit into float16.
			f16 = float16.Fromfloat32(v)
			if f16.Float32() == v {
				p = float16.PrecisionExact
			}
		}
		if p == float16.PrecisionExact {
			b.addFloat16(f16)
			return
		}
	}
	b.addFloat32(v)
}

func (b *Builder) AddFloat64(v float64) {
	if math.IsNaN(float64(v)) {
		if b.ModeNaN == ModeNaN7e00 {
			b.add(cborNaN...)
			return
		}
	} else if math.IsInf(float64(v), 0) {
		if b.ModeInf == ModeInfFloat16 {
			if v > 0 {
				b.add(cborPositiveInfinity...)
			} else {
				b.add(cborNegativeInfinity...)
			}
			return
		}
	}
	if b.ModeFloat == ModeFloatNone || cannotFitFloat32(v) {
		b.addFloat64(v)
	} else {
		b.AddFloat32(float32(v))
	}
}

func cannotFitFloat32(v float64) bool {
	f32 := float32(v)
	return float64(f32) != v
}

func (b *Builder) AddBytes(v []byte) {
	if v == nil {
		b.add(cborNil)
		return
	}
	if len(v) == 0 {
		b.add(cborTypeByteString)
		return
	}
	b.addUint64(cborTypeByteString, uint64(len(v)))
	b.add(v...)
}

func (b *Builder) AddBytesUnknownLength(fn BuilderContinuation) {
	b.addUnknown(cborTypeByteString, fn)
}

func (b *Builder) AddString(v string) {
	if len(v) == 0 {
		b.add(cborTypeTextString)
		return
	}
	b.addUint64(cborTypeTextString, uint64(len(v)))
	b.add([]byte(v)...)
}

func (b *Builder) AddNil() {
	b.add(cborNil)
}

func (b *Builder) AddArray(n uint64, fn BuilderContinuation) {
	b.addUint64(cborTypeArray, n)
	fn(b)
}

type AddMapItemFunc func(fnkey, fnvalue BuilderContinuation)

func (b *Builder) AddMap(length int) AddMapItemFunc {
	b.mapSize = 0
	b.addUint64(cborTypeMap, uint64(length))
	if len(b.offsets) < length {
		b.offsets = append(b.offsets, make([]mapItem, length-len(b.offsets))...)
	}
	return b.addMapItem
}

func (b *Builder) AddTag(number uint64) {
	b.addUint64(cborTypeTag, number)
}

type mapItem struct {
	offset    int
	keyLength int
}

func (b *Builder) sort() {
	keyFn := func(i int) []byte {
		mi := b.offsets[i]
		return b.result[mi.offset : mi.offset+mi.keyLength]
	}
	itemFn := func(i int) []byte {
		mi := b.offsets[i]
		max := len(b.result)
		if i < b.mapSize-1 {
			max = b.offsets[i+1].offset
		}
		return b.result[mi.offset:max]
	}
	x := keyFn(b.mapSize - 1)
	idx := sort.Search(b.mapSize-1, func(i int) bool {
		y := keyFn(i)
		if b.ModeSort == ModeSortLengthFirst && len(x) != len(y) {
			return len(x) < len(y)
		}
		return bytes.Compare(x, y) <= 0
	})
	if idx < b.mapSize-1 {
		last := itemFn(b.mapSize - 1)
		if len(b.tmp) < len(last) {
			b.tmp = append(b.tmp, make([]byte, len(last)-len(b.tmp))...)
		}
		newOffset := b.offsets[idx].offset
		copy(b.tmp, last)
		copy(b.result[newOffset+len(last):], b.result[newOffset:])
		copy(b.result[newOffset:], b.tmp[:len(last)])
		lastOffset := b.offsets[b.mapSize-1]
		for i := b.mapSize - 1; i > idx; i-- {
			prev := b.offsets[i-1]
			b.offsets[i] = mapItem{
				offset:    prev.offset + len(last),
				keyLength: prev.keyLength,
			}
		}
		lastOffset.offset = newOffset
		b.offsets[idx] = lastOffset
	}
}

func (b *Builder) addMapItem(k, v BuilderContinuation) {
	offset := b.Len()
	k(b)
	keyLength := b.Len() - offset
	v(b)
	b.offsets[b.mapSize] = mapItem{
		offset:    offset,
		keyLength: keyLength,
	}
	b.mapSize++
	if b.ModeSort != ModeSortNone {
		b.sort()
	}
}
