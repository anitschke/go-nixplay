package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/anitschke/go-nixplay/httpx"
	"golang.org/x/net/publicsuffix"
)

const (
	loginURL = "https://api.nixplay.com/www-login/"
)

type loginResponse struct {
	Valid   bool            `json:"valid"`
	Success bool            `json:"success"`
	Errors  json.RawMessage `json:"errors"`
	Token   string          `json:"token"`
}

type loginError struct {
	Messages [][]string `json:"messages"` // For some reason this is an array of arrays
}

// parseErrors parses errors in the login response back from Nixplay.
//
// The login response sent back from nixplay is a bit of a pain. If the login
// passed then it returns an empty array, but if it failed then it returns a
// json object that describes what field had an error and what the error was. So
// here we will parse the json.RawMessage and turn in to a go error if there was
// an error.
func (r loginResponse) parseErrors() error {
	if string(r.Errors) == "[]" {
		return nil
	}

	var fieldToError map[string]loginError
	json.Unmarshal(r.Errors, &fieldToError)
	var errs []error
	for field, errorObj := range fieldToError {
		if field == "email" {
			field = "username"
		}
		for _, messages := range errorObj.Messages {
			for _, message := range messages {
				if field == "__all__" {
					errs = append(errs, fmt.Errorf("issue with login: %s", message))
				} else {
					errs = append(errs, fmt.Errorf("issue with login property %q: %s", field, message))
				}
			}
		}
	}
	return errors.Join(errs...)
}

type Authorization struct {
	Username string
	Password string
}

type auth struct {
	token     string
	csrfToken string
	jar       http.CookieJar
}

type AuthorizedClient struct {
	client httpx.Client
	auth   auth
}

var _ = (httpx.Client)((*AuthorizedClient)(nil))

func NewAuthorizedClient(ctx context.Context, client httpx.Client, authIn Authorization) (*AuthorizedClient, error) {
	auth, err := doAuth(ctx, client, authIn)
	if err != nil {
		return nil, err
	}
	return &AuthorizedClient{
		client: client,
		auth:   auth,
	}, nil
}

func doAuth(ctx context.Context, client httpx.Client, authIn Authorization) (auth, error) {
	parsedLoginURL, err := url.Parse(loginURL)
	if err != nil {
		return auth{}, err
	}

	loginForm := url.Values{
		"email":    {authIn.Username},
		"password": {authIn.Password},
	}
	req, err := httpx.NewPostFormRequest(ctx, loginURL, loginForm)
	if err != nil {
		return auth{}, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return auth{}, fmt.Errorf("failed to log in to Nixplay: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return auth{}, fmt.Errorf("failed to log in to Nixplay: %s", resp.Status)
	}

	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return auth{}, err
	}

	cookies := resp.Cookies()
	allowedCookies := make([]*http.Cookie, 0, len(cookies))
	var csrfToken string
	for _, c := range cookies {
		if !strings.HasSuffix(c.Domain, ".nixplay.com") {
			continue
		}
		allowedCookies = append(allowedCookies, c)
		// Keep track of the CSRF token
		if c.Name == "prod.csrftoken" {
			csrfToken = c.Value
		}
	}
	jar.SetCookies(parsedLoginURL, allowedCookies)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth{}, fmt.Errorf("failed to read login response body: %w", err)
	}

	var response loginResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return auth{}, fmt.Errorf("failed to parse response body: %w", err)
	}
	if err := response.parseErrors(); err != nil {
		return auth{}, err
	}

	if csrfToken == "" {
		return auth{}, errors.New("CSRF token not set in log in response")
	}
	return auth{
		token:     response.Token,
		csrfToken: csrfToken,
		jar:       jar,
	}, nil
}

func (c *AuthorizedClient) Do(req *http.Request) (*http.Response, error) {
	for _, cookie := range c.auth.jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}
	req.Header.Set("X-CSRFToken", c.auth.csrfToken)
	req.Header.Set("Origin", "https://app.nixplay.com")
	req.Header.Set("Referer", "https://app.nixplay.com/")

	resp, err := c.client.Do(req)

	if err == nil {
		if rc := resp.Cookies(); len(rc) > 0 {
			c.auth.jar.SetCookies(req.URL, rc)
		}
	}
	return resp, err
}
