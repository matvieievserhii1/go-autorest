package azure

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
)

const MockTokenJSON string = `{
	"access_token": "accessToken",
	"refresh_token": "refreshToken",
	"expires_in": "1000",
	"expires_on": "2000",
	"not_before": "3000",
	"resource": "resource",
	"token_type": "type"
}`

var TestToken = Token{
	AccessToken:  "accessToken",
	RefreshToken: "refreshToken",
	ExpiresIn:    "1000",
	ExpiresOn:    "2000",
	NotBefore:    "3000",
	Resource:     "resource",
	Type:         "type",
}

func writeTestTokenFile(t *testing.T, suffix string, contents string) *os.File {
	f, err := ioutil.TempFile(os.TempDir(), suffix)
	if err != nil {
		t.Errorf("azure: unexpected error when creating temp file: %v", err)
	}

	_, err = f.Write([]byte(contents))
	if err != nil {
		t.Errorf("azure: unexpected error when writing temp test file: %v", err)
	}

	return f
}

func TestLoadToken(t *testing.T) {
	f := writeTestTokenFile(t, "testloadtoken", MockTokenJSON)
	defer os.Remove(f.Name())

	expectedToken := TestToken
	actualToken, err := LoadToken(f.Name())
	if err != nil {
		t.Errorf("azure: unexpected error loading token from file: %v", err)
	}

	if *actualToken != expectedToken {
		t.Errorf("azure: failed to decode properly expected(%v) actual(%v)", expectedToken, *actualToken)
	}
}

func TestLoadTokenFailsBadPath(t *testing.T) {
	_, err := LoadToken("/tmp/this_file_should_never_exist_really")
	expectedSubstring := "failed to open file"
	if err == nil || !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", expectedSubstring, err.Error())
	}
}

func TestLoadTokenFailsBadJson(t *testing.T) {
	gibberishJSON := strings.Replace(MockTokenJSON, "expires_on", ";:\"gibberish", -1)
	f := writeTestTokenFile(t, "testloadtokenfailsbadjson", gibberishJSON)
	defer os.Remove(f.Name())

	_, err := LoadToken(f.Name())
	expectedSubstring := "failed to decode contents of file"
	if err == nil || !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", expectedSubstring, err.Error())
	}
}

func token() *Token {
	var token Token
	json.Unmarshal([]byte(MockTokenJSON), &token)
	return &token
}

func TestSaveToken(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "testloadtoken")
	if err != nil {
		t.Errorf("azure: unexpected error when creating temp file: %v", err)
	}
	defer os.Remove(f.Name())

	err = SaveToken(f.Name(), *token())
	if err != nil {
		t.Errorf("azure: unexpected error saving token to file: %v", err)
	}

	var actualToken Token
	var expectedToken Token

	json.Unmarshal([]byte(MockTokenJSON), expectedToken)

	contents, err := ioutil.ReadFile(f.Name())
	if err != nil {
		t.Errorf("!!")
	}
	json.Unmarshal(contents, actualToken)

	if !reflect.DeepEqual(actualToken, expectedToken) {
		t.Errorf("azure: token was not serialized correctly")
	}
}

func TestSaveTokenFailsNoPermission(t *testing.T) {
	err := SaveToken("/usr/thiswontwork/atall", *token())
	expectedSubstring := "failed to create directory"
	if err == nil || !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", expectedSubstring, err.Error())
	}
}

func TestSaveTokenFailsCantCreate(t *testing.T) {
	err := SaveToken("/thiswontwork", *token())
	expectedSubstring := "failed to write token to temp file"
	if err == nil || !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", expectedSubstring, err.Error())
	}
}
