package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizedClient_LoginPass(t *testing.T) {
	auth, err := TestAccountAuth()
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
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, userProfileURL, bytes.NewReader([]byte{}))
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