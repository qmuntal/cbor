package cbor

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
