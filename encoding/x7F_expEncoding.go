//go:build go1.19
// +build go1.19

package encoding

// get_x7F_expEncoding
//
// It seems that there is some version dependent encoding in the Go standard
// library with the character "\x7F". In newer versions of Go this gets encoded
// to `\x7f`, but in versions prior to 1.19 this gets encoded to `\u007f`. This
// causes me some issues because I am on version 1.20 on my machine but rclone
// is still using 1.18 so I would prefer to run my tests in GitHub CI at that
// version to ensure that everything will work ok when integrated with RClone.
// So what we are doing here is defining two versions of get_x7F_expEncoding to
// get the two expected encodings for "\x7F" and we are using Go build
// constraints to switch which file is used based on the go version. This lets
// us use the correct expected value in the test based on the go version we are
// using. This should be ok for our use case as either encoding is valid for us
// as long as the round trip works correctly.
func get_x7F_expEncoding() string {
	return `\x7f`
}
