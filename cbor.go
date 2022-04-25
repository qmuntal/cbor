package cbor

import (
	"math/big"
	"reflect"
	"time"
)

const (
	cborTypePositiveInt uint8 = 0x00
	cborTypeNegativeInt uint8 = 0x20
	cborTypeByteString  uint8 = 0x40
	cborTypeTextString  uint8 = 0x60
	cborTypeArray       uint8 = 0x80
	cborTypeMap         uint8 = 0xa0
	cborTypeTag         uint8 = 0xc0
	cborTypePrimitives  uint8 = 0xe0
)

type Marshaler interface {
	MarshalCBOR(*Builder)
}

type RawBytes []byte

func (r RawBytes) MarshalCBOR(b *Builder) {
	b.AddRawBytes(r)
}

type Tag struct {
	Number  uint64
	Content interface{}
}

func (t Tag) MarshalCBOR(b *Builder) {
	b.AddTag(t.Number)
	b.Add(t.Content)
}

type RawTag struct {
	Number  uint64
	Content RawBytes
}

func (t RawTag) MarshalCBOR(b *Builder) {
	b.AddTag(t.Number)
	b.AddRawBytes(t.Content)
}

var (
	typeMarshaler = reflect.TypeOf((*Marshaler)(nil)).Elem()
	typeBigInt    = reflect.TypeOf(big.Int{})
	typeTime      = reflect.TypeOf(time.Time{})
)
