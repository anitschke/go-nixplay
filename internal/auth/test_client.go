package auth

import (
	"fmt"
	"os"
)

// xxx add cspell github integration

// xxx add testing with real account to github CI via https://docs.github.com/en/actions/security-guides/encrypted-secrets
//
// This also has a good explanation of how to do this https://dev.to/petrsvihlik/using-environment-protection-rules-to-secure-secrets-when-building-external-forks-with-pullrequesttarget-hci

const (
	testUsernameEnvVar = "GO_NIXPLAY_TEST_ACCOUNT_USERNAME"
	testPasswordEnvVar = "GO_NIXPLAY_TEST_ACCOUNT_PASSWORD"
)

// TestAccountAuth gets the test account Authorization 
//
// Authorization details are obtained from the
// "GO_NIXPLAY_TEST_ACCOUNT_USERNAME" and "GO_NIXPLAY_TEST_ACCOUNT_PASSWORD"
// environment variables. For more details ../../README.md#Testing xxx
func TestAccountAuth() (Authorization, error) {
	username := os.Getenv(testUsernameEnvVar)
	password := os.Getenv(testPasswordEnvVar)

	if username == "" || password == "" {
		return Authorization{}, fmt.Errorf("the environment variables %q and %q must be set to configure the Nixplay account used for testing", testUsernameEnvVar, testPasswordEnvVar)
	}

	return Authorization{
		Username: username,
		Password: password,
	}, nil
}
