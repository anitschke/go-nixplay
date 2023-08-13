package encoding

import "strconv"

const quote = `"`

// Encode returns a string that uses Go escape sequences for non-ASCII and
// non-printable characters as defined by IsPrint. In addition some other
// characters such as backslashes (\) and double quotes (") will also be escaped
// using Go escape sequence.
//
// The rational here is that given Nixplay does not document any sort of API we
// really don't have any guarantee what sort of characters it supports now or in
// the future. I did some experimentation and things seem to mostly work except
// for some non-ASCII characters like emoji. So in an effort to make sure we can
// continue to support any sort of weird names that we can come up with we will
// be pretty aggressive with the encoding and encode any non-ASCII or
// non-printable characters.
func Encode(name string) string {
	quotedName := strconv.QuoteToASCII(name)
	safeName := quotedName[1 : len(quotedName)-1]
	return safeName
}

// Decode returns a decoded string that was encoded using Encode.
//
// If the provided string is not a valid encoding (for example it ends with a
// backslash) then an error will be returned.
func Decode(name string) (string, error) {
	return strconv.Unquote(quote + name + quote)
}
