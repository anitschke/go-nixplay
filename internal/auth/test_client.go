package auth

import (
	"fmt"
	"os"

	"github.com/anitschke/go-nixplay/types"
)

const (
	testUsernameEnvVar = "GO_NIXPLAY_TEST_ACCOUNT_USERNAME"
	testPasswordEnvVar = "GO_NIXPLAY_TEST_ACCOUNT_PASSWORD"
)

// TestAccountAuth gets the test account Authorization
//
// Authorization details are obtained from the
// "GO_NIXPLAY_TEST_ACCOUNT_USERNAME" and "GO_NIXPLAY_TEST_ACCOUNT_PASSWORD"
// environment variables. For more details see
// https://github.com/anitschke/go-nixplay/#testing
func TestAccountAuth() (types.Authorization, error) {
	username := os.Getenv(testUsernameEnvVar)
	password := os.Getenv(testPasswordEnvVar)

	if username == "" || password == "" {
		return types.Authorization{}, fmt.Errorf("the environment variables %q and %q must be set to configure the Nixplay account used for testing", testUsernameEnvVar, testPasswordEnvVar)
	}

	return types.Authorization{
		Username: username,
		Password: password,
	}, nil
}
