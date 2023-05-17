package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
)

// xxx add testing with real account via https://docs.github.com/en/actions/security-guides/encrypted-secrets
//
// This also has a good explanation of how to do this https://dev.to/petrsvihlik/using-environment-protection-rules-to-secure-secrets-when-building-external-forks-with-pullrequesttarget-hci

const (
	testUsernameEnvVar = "GO_NIXPLAY_TEST_ACCOUNT_USERNAME"
	testPasswordEnvVar = "GO_NIXPLAY_TEST_ACCOUNT_PASSWORD"
)

// xxx doc
func TestAccountAuth() (Authorization, error) {
	var err error
	username := os.Getenv(testUsernameEnvVar)
	if username == "" {
		err = fmt.Errorf("the environment variable %q must be set to configure the Nixplay account used for testing", testUsernameEnvVar)
	}

	password := os.Getenv(testPasswordEnvVar)
	if password == "" {
		err = errors.Join(err, fmt.Errorf("the environment variable %q must be set to configure the Nixplay password used for testing", testPasswordEnvVar))
	}

	return Authorization{
		Username: username,
		Password: password,
	}, err
}

func NewDefaultTestAuthorizedClient() (*AuthorizedClient, error) {
	auth, err := TestAccountAuth()
	if err != nil {
		return nil, err
	}
	client := http.Client{}
	return NewAuthorizedClient(context.Background(), &client, auth)
}

func Must(client *AuthorizedClient, err error) *AuthorizedClient {
	if err != nil {
		panic(err)
	}
	return client
}
