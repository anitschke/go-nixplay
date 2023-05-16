package nixplay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMD5_Pass_RealValue(t *testing.T) {
	type testData struct {
		name      string
		md5String string
		expHash   MD5Hash
	}

	testCases := []testData{
		{
			name:      "zeroValue",
			md5String: "00000000000000000000000000000000",
			expHash:   MD5Hash{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		},
		{
			name:      "realHash",
			md5String: "073089b1d67a56c63b989d4e5f660ab8",
			expHash:   MD5Hash{0x7, 0x30, 0x89, 0xb1, 0xd6, 0x7a, 0x56, 0xc6, 0x3b, 0x98, 0x9d, 0x4e, 0x5f, 0x66, 0xa, 0xb8},
		},
	}

	for _, td := range testCases {
		t.Run(td.name, func(t *testing.T) {
			var hash MD5Hash
			err := hash.UnmarshalText([]byte(td.md5String))
			assert.NoError(t, err)
			assert.Equal(t, hash, td.expHash)
		})
	}
}

func TestParseMD5_Error(t *testing.T) {
	type testData struct {
		name      string
		md5String string
	}

	testCases := []testData{
		{
			name:      "tooShort",
			md5String: "0",
		},
		{
			name:      "tooLong",
			md5String: "0000000000000000000000000000000000000000000",
		},
		{
			name:      "invalidCharacters",
			md5String: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		},
	}

	for _, td := range testCases {
		t.Run(td.name, func(t *testing.T) {
			var hash MD5Hash
			err := hash.UnmarshalText([]byte(td.md5String))
			assert.Error(t, err)
		})
	}
}
