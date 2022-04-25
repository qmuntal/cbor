package cbor

import (
	"bytes"
	"encoding/hex"
	"io"
	"math"
	"math/big"
	"testing"
)

type marshalTest struct {
	cborData []byte
	values   []interface{}
}

type marshalErrorTest struct {
	name         string
	value        interface{}
	wantErrorMsg string
}

type inner struct {
	X, Y, z int64
}

type outer struct {
	IntField          int
	FloatField        float32
	BoolField         bool
	StringField       string
	ByteStringField   []byte
	ArrayField        []string
	MapField          map[string]bool
	NestedStructField *inner
	unexportedField   int64
}

func hexDecode(s string) []byte {
	data, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return data
}

func bigIntOrPanic(s string) big.Int {
	bi, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("failed to convert " + s + " to big.Int")
	}
	return *bi
}

// CBOR test data are from https://tools.ietf.org/html/rfc7049#appendix-A.
var marshalTests = []marshalTest{
	// positive integer
	{hexDecode("00"), []interface{}{uint(0), uint8(0), uint16(0), uint32(0), uint64(0), int(0), int8(0), int16(0), int32(0), int64(0)}},
	{hexDecode("01"), []interface{}{uint(1), uint8(1), uint16(1), uint32(1), uint64(1), int(1), int8(1), int16(1), int32(1), int64(1)}},
	{hexDecode("0a"), []interface{}{uint(10), uint8(10), uint16(10), uint32(10), uint64(10), int(10), int8(10), int16(10), int32(10), int64(10)}},
	{hexDecode("17"), []interface{}{uint(23), uint8(23), uint16(23), uint32(23), uint64(23), int(23), int8(23), int16(23), int32(23), int64(23)}},
	{hexDecode("1818"), []interface{}{uint(24), uint8(24), uint16(24), uint32(24), uint64(24), int(24), int8(24), int16(24), int32(24), int64(24)}},
	{hexDecode("1819"), []interface{}{uint(25), uint8(25), uint16(25), uint32(25), uint64(25), int(25), int8(25), int16(25), int32(25), int64(25)}},
	{hexDecode("1864"), []interface{}{uint(100), uint8(100), uint16(100), uint32(100), uint64(100), int(100), int8(100), int16(100), int32(100), int64(100)}},
	{hexDecode("18ff"), []interface{}{uint(255), uint8(255), uint16(255), uint32(255), uint64(255), int(255), int16(255), int32(255), int64(255)}},
	{hexDecode("190100"), []interface{}{uint(256), uint16(256), uint32(256), uint64(256), int(256), int16(256), int32(256), int64(256)}},
	{hexDecode("1903e8"), []interface{}{uint(1000), uint16(1000), uint32(1000), uint64(1000), int(1000), int16(1000), int32(1000), int64(1000)}},
	{hexDecode("19ffff"), []interface{}{uint(65535), uint16(65535), uint32(65535), uint64(65535), int(65535), int32(65535), int64(65535)}},
	{hexDecode("1a00010000"), []interface{}{uint(65536), uint32(65536), uint64(65536), int(65536), int32(65536), int64(65536)}},
	{hexDecode("1a000f4240"), []interface{}{uint(1000000), uint32(1000000), uint64(1000000), int(1000000), int32(1000000), int64(1000000)}},
	{hexDecode("1affffffff"), []interface{}{uint(4294967295), uint32(4294967295), uint64(4294967295), int64(4294967295)}},
	{hexDecode("1b000000e8d4a51000"), []interface{}{uint64(1000000000000), int64(1000000000000)}},
	{hexDecode("1bffffffffffffffff"), []interface{}{uint64(18446744073709551615)}},
	// negative integer
	{hexDecode("20"), []interface{}{int(-1), int8(-1), int16(-1), int32(-1), int64(-1)}},
	{hexDecode("29"), []interface{}{int(-10), int8(-10), int16(-10), int32(-10), int64(-10)}},
	{hexDecode("37"), []interface{}{int(-24), int8(-24), int16(-24), int32(-24), int64(-24)}},
	{hexDecode("3818"), []interface{}{int(-25), int8(-25), int16(-25), int32(-25), int64(-25)}},
	{hexDecode("3863"), []interface{}{int(-100), int8(-100), int16(-100), int32(-100), int64(-100)}},
	{hexDecode("38ff"), []interface{}{int(-256), int16(-256), int32(-256), int64(-256)}},
	{hexDecode("390100"), []interface{}{int(-257), int16(-257), int32(-257), int64(-257)}},
	{hexDecode("3903e7"), []interface{}{int(-1000), int16(-1000), int32(-1000), int64(-1000)}},
	{hexDecode("39ffff"), []interface{}{int(-65536), int32(-65536), int64(-65536)}},
	{hexDecode("3a00010000"), []interface{}{int(-65537), int32(-65537), int64(-65537)}},
	{hexDecode("3affffffff"), []interface{}{int64(-4294967296)}},
	// byte string
	{hexDecode("40"), []interface{}{[]byte{}}},
	{hexDecode("4401020304"), []interface{}{[]byte{1, 2, 3, 4}, [...]byte{1, 2, 3, 4}}},
	// text string
	{hexDecode("60"), []interface{}{""}},
	{hexDecode("6161"), []interface{}{"a"}},
	{hexDecode("6449455446"), []interface{}{"IETF"}},
	{hexDecode("62225c"), []interface{}{"\"\\"}},
	{hexDecode("62c3bc"), []interface{}{"ü"}},
	{hexDecode("63e6b0b4"), []interface{}{"水"}},
	{hexDecode("64f0908591"), []interface{}{"𐅑"}},
	// array
	{
		hexDecode("80"),
		[]interface{}{
			[0]int{},
			[]uint{},
			// []uint8{},
			[]uint16{},
			[]uint32{},
			[]uint64{},
			[]int{},
			[]int8{},
			[]int16{},
			[]int32{},
			[]int64{},
			[]string{},
			[]bool{}, []float32{}, []float64{}, []interface{}{},
		},
	},
	{
		hexDecode("83010203"),
		[]interface{}{
			[...]int{1, 2, 3},
			[]uint{1, 2, 3},
			// []uint8{1, 2, 3},
			[]uint16{1, 2, 3},
			[]uint32{1, 2, 3},
			[]uint64{1, 2, 3},
			[]int{1, 2, 3},
			[]int8{1, 2, 3},
			[]int16{1, 2, 3},
			[]int32{1, 2, 3},
			[]int64{1, 2, 3},
			[]interface{}{1, 2, 3},
		},
	},
	{
		hexDecode("8301820203820405"),
		[]interface{}{
			[...]interface{}{1, [...]int{2, 3}, [...]int{4, 5}},
			[]interface{}{1, []uint{2, 3}, []uint{4, 5}},
			// []interface{}{1, []uint8{2, 3}, []uint8{4, 5}},
			[]interface{}{1, []uint16{2, 3}, []uint16{4, 5}},
			[]interface{}{1, []uint32{2, 3}, []uint32{4, 5}},
			[]interface{}{1, []uint64{2, 3}, []uint64{4, 5}},
			[]interface{}{1, []int{2, 3}, []int{4, 5}},
			[]interface{}{1, []int8{2, 3}, []int8{4, 5}},
			[]interface{}{1, []int16{2, 3}, []int16{4, 5}},
			[]interface{}{1, []int32{2, 3}, []int32{4, 5}},
			[]interface{}{1, []int64{2, 3}, []int64{4, 5}},
			[]interface{}{1, []interface{}{2, 3}, []interface{}{4, 5}},
		},
	},
	{
		hexDecode("98190102030405060708090a0b0c0d0e0f101112131415161718181819"),
		[]interface{}{
			[...]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			// []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]uint16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]int8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]int16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
		},
	},
	{
		hexDecode("826161a161626163"),
		[]interface{}{
			[...]interface{}{"a", map[string]string{"b": "c"}},
			[]interface{}{"a", map[string]string{"b": "c"}},
			[]interface{}{"a", map[interface{}]interface{}{"b": "c"}},
		},
	},
	// map
	{
		hexDecode("a0"),
		[]interface{}{
			map[uint]bool{},
			map[uint8]bool{},
			map[uint16]bool{},
			map[uint32]bool{},
			map[uint64]bool{},
			map[int]bool{},
			map[int8]bool{},
			map[int16]bool{},
			map[int32]bool{},
			map[int64]bool{},
			map[float32]bool{},
			map[float64]bool{},
			map[bool]bool{},
			map[string]bool{},
			map[interface{}]interface{}{},
		},
	},
	{
		hexDecode("a201020304"),
		[]interface{}{
			map[uint]uint{3: 4, 1: 2},
			map[uint8]uint8{3: 4, 1: 2},
			map[uint16]uint16{3: 4, 1: 2},
			map[uint32]uint32{3: 4, 1: 2},
			map[uint64]uint64{3: 4, 1: 2},
			map[int]int{3: 4, 1: 2},
			map[int8]int8{3: 4, 1: 2},
			map[int16]int16{3: 4, 1: 2},
			map[int32]int32{3: 4, 1: 2},
			map[int64]int64{3: 4, 1: 2},
			map[interface{}]interface{}{3: 4, 1: 2},
		},
	},
	{
		hexDecode("a26161016162820203"),
		[]interface{}{
			map[string]interface{}{"a": 1, "b": []interface{}{2, 3}},
			map[interface{}]interface{}{"b": []interface{}{2, 3}, "a": 1},
		},
	},
	{
		hexDecode("a56161614161626142616361436164614461656145"),
		[]interface{}{
			map[string]string{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"},
			map[interface{}]interface{}{"b": "B", "a": "A", "c": "C", "e": "E", "d": "D"},
		},
	},
	// tag
	{
		hexDecode("c074323031332d30332d32315432303a30343a30305a"),
		[]interface{}{Tag{0, "2013-03-21T20:04:00Z"}, RawTag{0, hexDecode("74323031332d30332d32315432303a30343a30305a")}},
	}, // 0: standard date/time
	{
		hexDecode("c11a514b67b0"),
		[]interface{}{Tag{1, uint64(1363896240)}, RawTag{1, hexDecode("1a514b67b0")}},
	}, // 1: epoch-based date/time
	{
		hexDecode("c249010000000000000000"),
		[]interface{}{
			bigIntOrPanic("18446744073709551616"),
			Tag{2, []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
			RawTag{2, hexDecode("49010000000000000000")},
		},
	}, // 2: positive bignum: 18446744073709551616
	{
		hexDecode("c349010000000000000000"),
		[]interface{}{
			bigIntOrPanic("-18446744073709551617"),
			Tag{3, []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
			RawTag{3, hexDecode("49010000000000000000")},
		},
	}, // 3: negative bignum: -18446744073709551617
	{
		hexDecode("c1fb41d452d9ec200000"),
		[]interface{}{Tag{1, float64(1363896240.5)}, RawTag{1, hexDecode("fb41d452d9ec200000")}},
	}, // 1: epoch-based date/time
	{
		hexDecode("d74401020304"),
		[]interface{}{Tag{23, []byte{0x01, 0x02, 0x03, 0x04}}, RawTag{23, hexDecode("4401020304")}},
	}, // 23: expected conversion to base16 encoding
	{
		hexDecode("d818456449455446"),
		[]interface{}{Tag{24, []byte{0x64, 0x49, 0x45, 0x54, 0x46}}, RawTag{24, hexDecode("456449455446")}},
	}, // 24: encoded cborBytes data item
	{
		hexDecode("d82076687474703a2f2f7777772e6578616d706c652e636f6d"),
		[]interface{}{Tag{32, "http://www.example.com"}, RawTag{32, hexDecode("76687474703a2f2f7777772e6578616d706c652e636f6d")}},
	}, // 32: URI
	// primitives
	{hexDecode("f4"), []interface{}{false}},
	{hexDecode("f5"), []interface{}{true}},
	{hexDecode("f6"), []interface{}{nil, []byte(nil), []int(nil), map[uint]bool(nil), (*int)(nil), io.Reader(nil)}},
	// nan, positive and negative inf
	{hexDecode("f97c00"), []interface{}{math.Inf(1)}},
	{hexDecode("f97e00"), []interface{}{math.NaN()}},
	{hexDecode("f9fc00"), []interface{}{math.Inf(-1)}},
	// float32
	{hexDecode("fa47c35000"), []interface{}{float32(100000.0)}},
	{hexDecode("fa7f7fffff"), []interface{}{float32(3.4028234663852886e+38)}},
	// float64
	{hexDecode("fb3ff199999999999a"), []interface{}{float64(1.1)}},
	{hexDecode("fb7e37e43c8800759c"), []interface{}{float64(1.0e+300)}},
	{hexDecode("fbc010666666666666"), []interface{}{float64(-4.1)}},
	// More testcases not covered by https://tools.ietf.org/html/rfc7049#appendix-A.
	{
		hexDecode("d83dd183010203"), // 61(17([1, 2, 3])), nested tags 61 and 17
		[]interface{}{Tag{61, Tag{17, []interface{}{uint64(1), uint64(2), uint64(3)}}}, RawTag{61, hexDecode("d183010203")}},
	},
}

var exMarshalTests = []marshalTest{
	{
		// array of nils
		hexDecode("83f6f6f6"),
		[]interface{}{
			[]interface{}{nil, nil, nil},
		},
	},
}

func TestMarshal(t *testing.T) {
	testMarshal(t, marshalTests)
	testMarshal(t, exMarshalTests)
}

func testMarshal(t *testing.T, testCases []marshalTest) {
	for _, tc := range testCases {
		for _, value := range tc.values {
			if b, err := Marshal(value); err != nil {
				t.Errorf("Marshal(%v) returned error %v", value, err)
			} else if !bytes.Equal(b, tc.cborData) {
				t.Errorf("Marshal(%v) = 0x%x, want 0x%x", value, b, tc.cborData)
			}
		}
		r := RawBytes(tc.cborData)
		if b, err := Marshal(r); err != nil {
			t.Errorf("Marshal(%v) returned error %v", r, err)
		} else if !bytes.Equal(b, r) {
			t.Errorf("Marshal(%v) returned %v, want %v", r, b, r)
		}
	}
}
