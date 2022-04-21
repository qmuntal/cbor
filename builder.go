package cbor

import (
	"bytes"
	"errors"
	"math"

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
	// ModeFloatNone makes float values encode without any conversion.
	// E.g. a float32 in Go will encode to CBOR float32.  And
	// a float64 in Go will encode to CBOR float64.
	ModeFloatNone ModeFloat = iota

	// ModeFloat16 specifies float16 as the shortest form that preserves value.
	// E.g. if float64 can convert to float32 while preserving value, then
	// encoding will also try to convert float32 to float16.  So a float64 might
	// encode as CBOR float64, float32 or float16 depending on the value.
	ModeFloat16
)

type Builder struct {
	ModeNaN   ModeNaN
	ModeInf   ModeInf
	ModeFloat ModeFloat
	err       error
	result    []byte
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
		b.add(t | byte(24))
	}
}

func (b *Builder) addUint16(t uint8, v uint16) {
	if v <= 23 {
		b.add(t | byte(v))
	} else {
		b.add(t|byte(25), byte(v>>8), byte(v))
	}
}

func (b *Builder) addUint32(t uint8, v uint32) {
	if v <= 23 {
		b.add(t | byte(v))
	} else {
		b.add(t|byte(26), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	}
}

func (b *Builder) addUint64(t uint8, v uint64) {
	if v <= 23 {
		b.add(t | byte(v))
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
	if v >= 0 {
		b.AddUint(uint(v))
	} else {
		const t = cborTypeNegativeInt
		v = v*(-1) - 1
		if v <= math.MaxUint8 {
			b.addUint8(t, uint8(v))
		} else if v <= math.MaxUint16 {
			b.addUint16(t, uint16(v))
		} else if v <= math.MaxUint32 {
			b.addUint32(t, uint32(v))
		} else {
			b.addUint64(t, uint64(v))
		}
	}
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

func (b *Builder) addUint(t byte, v uint) {
	if v <= math.MaxUint8 {
		b.addUint8(t, uint8(v))
	} else if v <= math.MaxUint16 {
		b.addUint16(t, uint16(v))
	} else if v <= math.MaxUint32 {
		b.addUint32(t, uint32(v))
	} else {
		b.addUint64(t, uint64(v))
	}
}

func (b *Builder) AddUint(v uint) {
	b.addUint(cborTypePositiveInt, v)
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
		byte(f>>56), byte(f>>48), byte(f>>40), byte(32),
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
	b.addUint(cborTypeByteString, uint(len(v)))
	b.add(v...)
}

func (b *Builder) AddString(v string) {
	if len(v) == 0 {
		b.add(cborTypeByteString)
		return
	}
	b.addUint(cborTypeTextString, uint(len(v)))
	b.add([]byte(v)...)
}

func (b *Builder) AddNil() {
	b.add(cborNil)
}

func (b *Builder) AddSliceHeader(length int) {
	if length == 0 {
		b.add(cborTypeArray)
		return
	}
	b.addUint(cborTypeArray, uint(length))
}

func (b *Builder) AddMapHeader(length int) {
	if length == 0 {
		b.add(cborTypeMap)
		return
	}
	b.addUint(cborTypeMap, uint(length))
}

func (b *Builder) AddTagHeader(length int) {
	if length == 0 {
		b.add(cborTypeTag)
		return
	}
	b.addUint(cborTypeTag, uint(length))
}

type BytewiseSorter [][]byte

func (x BytewiseSorter) Len() int {
	return len(x)
}

func (x BytewiseSorter) Swap(i, j int) {
	x[i], x[j] = x[j], x[i]
}

func (x BytewiseSorter) Less(i, j int) bool {
	return bytes.Compare(x[i], x[j]) <= 0
}

type LengthFirstSorter [][]byte

func (x LengthFirstSorter) Len() int {
	return len(x)
}

func (x LengthFirstSorter) Swap(i, j int) {
	x[i], x[j] = x[j], x[i]
}

func (x LengthFirstSorter) Less(i, j int) bool {
	if len(x[i]) != len(x[j]) {
		return len(x[i]) < len(x[j])
	}
	return bytes.Compare(x[i], x[j]) <= 0
}
