package auth

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/pbkdf2"
)

func TestAuthorizedClient_LoginPass(t *testing.T) {
	auth, err := TestAccountAuth()

	// GitHub prevents secrets from being printed to the log (which is good) but
	// when running tests in a GitHub action I ran into some issues where the
	// signing wasn't working correctly. So to help debug we will log a salted
	// and hashed username and password. This should be good enough for
	// security, and allows us to debug to see that the secret is getting injected
	// into GitHub correctly (by comparing with hash on local machine).
	saltAndHash := func(secret string) string {
		salt := "B2NMwfqjjMcRtWsXqsFZ5Mf" // cspell:disable-line
		return hex.EncodeToString(pbkdf2.Key([]byte(secret), []byte(salt), 1000000, 32, sha512.New))
	}
	t.Log(saltAndHash(auth.Username))
	t.Log(saltAndHash(auth.Password))

	assert.NoError(t, err)
	client := http.Client{}
	authClient, err := NewAuthorizedClient(context.Background(), &client, auth)
	assert.NoError(t, err)
	assert.NotNil(t, authClient)
}

func TestAuthorizedClient_LoginFail_EmptyLogin(t *testing.T) {
	invalidAuth := Authorization{
		Username: "",
		Password: "",
	}
	client := http.Client{}
	authClient, err := NewAuthorizedClient(context.Background(), &client, invalidAuth)
	assert.ErrorContains(t, err, "Please enter password")
	assert.ErrorContains(t, err, "Please enter your email address")
	assert.ErrorContains(t, err, "Please check your username and password")
	assert.Nil(t, authClient)
}

func TestAuthorizedClient_LoginFail_InvalidLogin(t *testing.T) {
	invalidAuth := Authorization{
		Username: "ThisIsNotAValidUser",
		Password: "ThisIsNotAValidPassword",
	}
	client := http.Client{}
	authClient, err := NewAuthorizedClient(context.Background(), &client, invalidAuth)
	assert.ErrorContains(t, err, "Please check your username and password")
	assert.Nil(t, authClient)
}

func TestAuthorizedClient_SendRequest(t *testing.T) {
	// This is a simple test to authorize a client and then make a request to an
	// endpoint to get some user data so we can verify that we have the cookies,
	// tokens and what not set correctly. For this we will use the endpoint that
	// is used to get user profile. We can know we got a good response back
	// because we expect the "old_username" property to be
	// "${testUsername@nixplay.com}"

	auth, err := TestAccountAuth()
	assert.NoError(t, err)
	client := http.Client{}
	authClient, err := NewAuthorizedClient(context.Background(), &client, auth)
	require.NoError(t, err)

	userProfileURL := "https://api.nixplay.com/user/profile/edit/"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, userProfileURL, http.NoBody)
	require.NoError(t, err)

	resp, err := authClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	type profileResponse struct {
		OldUsername string `json:"old_username"`
	}

	var decodedResponse profileResponse
	err = json.Unmarshal(body, &decodedResponse)
	require.NoError(t, err)

	expOldUsername := auth.Username + "@mynixplay.com"
	assert.Equal(t, decodedResponse.OldUsername, expOldUsername)
}
