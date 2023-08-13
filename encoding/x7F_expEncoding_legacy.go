//go:build !go1.19
// +build !go1.19

package encoding

// get_x7F_expEncoding
//
// See get_x7F_expEncoding in x7F_expEncoding.go
func get_x7F_expEncoding() string {
	return `\u007f`
}
