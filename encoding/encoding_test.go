package encoding

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

func TestEncoding(t *testing.T) {
	type testData struct {
		description string
		decoded     string
		encoded     string
	}

	// This list of encoding decoding was originally pulled from the rclone
	// encoding decode test suite and more cases where added such as unicode
	// characters.
	tests := []testData{
		{"number", "1234567890", "1234567890"},
		{"lowercase letter", "abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrstuvwxyz"},
		{"uppercase letter", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{`control char \x00`, "\x00", `\x00`},
		{`control char \x01`, "\x01", `\x01`},
		{`control char \x02`, "\x02", `\x02`},
		{`control char \x03`, "\x03", `\x03`},
		{`control char \x04`, "\x04", `\x04`},
		{`control char \x05`, "\x05", `\x05`},
		{`control char \x06`, "\x06", `\x06`},
		{`control char \x07`, "\x07", `\a`},
		{`control char \x08`, "\x08", `\b`},
		{`control char \x09`, "\x09", `\t`},
		{`control char \x0A`, "\x0A", `\n`},
		{`control char \x0B`, "\x0B", `\v`},
		{`control char \x0C`, "\x0C", `\f`},
		{`control char \x0D`, "\x0D", `\r`},
		{`control char \x0E`, "\x0E", `\x0e`},
		{`control char \x0F`, "\x0F", `\x0f`},
		{`control char \x10`, "\x10", `\x10`},
		{`control char \x11`, "\x11", `\x11`},
		{`control char \x12`, "\x12", `\x12`},
		{`control char \x13`, "\x13", `\x13`},
		{`control char \x14`, "\x14", `\x14`},
		{`control char \x15`, "\x15", `\x15`},
		{`control char \x16`, "\x16", `\x16`},
		{`control char \x17`, "\x17", `\x17`},
		{`control char \x18`, "\x18", `\x18`},
		{`control char \x19`, "\x19", `\x19`},
		{`control char \x1A`, "\x1A", `\x1a`},
		{`control char \x1B`, "\x1B", `\x1b`},
		{`control char \x1C`, "\x1C", `\x1c`},
		{`control char \x1D`, "\x1D", `\x1d`},
		{`control char \x1E`, "\x1E", `\x1e`},
		{`control char \x1F`, "\x1F", `\x1f`},
		{`control char \x7F`, "\x7F", `\x7f`},
		{"dot", ".", "."},
		{"dot dot", "..", ".."},
		{`punctuation !`, `!`, `!`},
		{`punctuation "`, `"`, `\"`},
		{`punctuation #`, `#`, `#`},
		{`punctuation $`, `$`, `$`},
		{`punctuation %`, `%`, `%`},
		{`punctuation &`, `&`, `&`},
		{`punctuation '`, `'`, `'`},
		{`punctuation (`, `(`, `(`},
		{`punctuation )`, `)`, `)`},
		{`punctuation *`, `*`, `*`},
		{`punctuation +`, `+`, `+`},
		{`punctuation ,`, `,`, `,`},
		{`punctuation -`, `-`, `-`},
		{`punctuation .`, `.`, `.`},
		{`punctuation /`, `/`, `/`},
		{`punctuation \`, `\`, `\\`},
		{`punctuation :`, `:`, `:`},
		{`punctuation ;`, `;`, `;`},
		{`punctuation <`, `<`, `<`},
		{`punctuation =`, `=`, `=`},
		{`punctuation >`, `>`, `>`},
		{`punctuation ?`, `?`, `?`},
		{`punctuation @`, `@`, `@`},
		{`punctuation [`, `[`, `[`},
		{`punctuation ]`, `]`, `]`},
		{`punctuation ^`, `^`, `^`},
		{`punctuation _`, `_`, `_`},
		{"punctuation `", "`", "`"},
		{`punctuation {`, `{`, `{`},
		{`punctuation }`, `}`, `}`},
		{`punctuation |`, `|`, `|`},
		{`punctuation ~`, `~`, `~`},
		{"leading trailing space", " space ", " space "},
		{"leading trailing tilde", "~tilde~", "~tilde~"},
		{"leading trailing quote", `"quote"`, `\"quote\"`},
		{"leading trailing backslash", `\backslash\`, `\\backslash\\`},
		{"leading trailing CR", "\rCR\r", `\rCR\r`},
		{"leading trailing LF", "\nLF\n", `\nLF\n`},
		{"leading trailing HT", "\tHT\t", `\tHT\t`},
		{"leading trailing VT", "\vVT\v", `\vVT\v`},
		{"leading trailing dot", ".dot.", ".dot."},
		{"invalid UTF-8", "invalid utf-8\xfe", `invalid utf-8\xfe`},
		{"URL encoding", "test%46.txt", "test%46.txt"},
		{"Japanese Kanji", "\u6f22\u5b57", `\u6f22\u5b57`}, // Some Kanji from https://en.wikipedia.org/wiki/Kanji
		{"Emoji", "\U0001f60a", `\U0001f60a`},              // SMILING FACE WITH SMILING EYES emoji
		{"Full Width Characters", "\uff26\uff55\uff4c\uff4c\uff37\uff49\uff44\uff54\uff48", `\uff26\uff55\uff4c\uff4c\uff37\uff49\uff44\uff54\uff48`}, // "FullWidth" in full width characters
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			t.Run("encode", func(t *testing.T) {
				act := Encode(tt.decoded)
				assert.Equal(t, act, tt.encoded)
			})
			t.Run("decode", func(t *testing.T) {
				act, err := Decode(tt.encoded)
				assert.NoError(t, err)
				assert.Equal(t, act, tt.decoded)
			})

			t.Run("round trip", func(t *testing.T) {
				actEncoded := Encode(tt.decoded)
				actDecoded, err := Decode(actEncoded)
				assert.NoError(t, err)
				assert.Equal(t, actDecoded, tt.decoded)

				actDecoded, err = Decode(tt.encoded)
				assert.NoError(t, err)
				actEncoded = Encode(actDecoded)
				assert.Equal(t, actEncoded, tt.encoded)
			})

			t.Run("encoding valid", func(t *testing.T) {
				// All characters in the encoded string should be valid
				// printable ASCII characters
				for _, c := range tt.encoded {
					assert.LessOrEqual(t, c, unicode.MaxASCII)
					assert.True(t, unicode.IsPrint(c))
				}
			})
		})
	}

}
