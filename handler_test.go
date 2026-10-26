package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeVault struct {
	secrets map[string]string
	asked   int
}

func (v *fakeVault) Resolve(_ context.Context, reference string) (string, error) {
	v.asked++
	secret, ok := v.secrets[reference]
	if !ok {
		return "", errors.New("no item matched")
	}
	return secret, nil
}

func newFakeVault() *fakeVault {
	return &fakeVault{secrets: map[string]string{
		"op://Home/Postgres/password": "postgres-password",
		"op://Home/Mail/token":        "mail-token",
	}}
}

func post(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var body T
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestResolvesEachName(t *testing.T) {
	recorder := post(newHandler(newFakeVault(), "/", ""), "/", `{"postgres":"op://Home/Postgres/password","mail":"op://Home/Mail/token"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d", recorder.Code)
	}
	values := decode[map[string]string](t, recorder)
	if len(values) != 2 || values["postgres"] != "postgres-password" || values["mail"] != "mail-token" {
		t.Fatalf("got %v", values)
	}
}

func TestNamesEveryFailureAndReleasesNothing(t *testing.T) {
	recorder := post(newHandler(newFakeVault(), "/", ""), "/", `{"postgres":"op://Home/Postgres/password","a":"op://Home/A/x","b":"op://Home/B/x"}`)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "postgres-password") {
		t.Fatal("released a secret alongside the failures")
	}
	failure := decode[errorReply](t, recorder)
	if failure.Error != "could not resolve a (op://Home/A/x): no item matched; b (op://Home/B/x): no item matched" {
		t.Fatalf("got %q", failure.Error)
	}
}

func TestRejectsRequestsKeyholeDoesNotUnderstand(t *testing.T) {
	cases := []struct {
		wrap string
		body string
	}{
		{"", `{"postgres":`},
		{"", `null`},
		{"", `{"postgres":{"vault":"Home"}}`},
		{"", `{"postgres":"op://Home/Postgres/password"} {"mail":"op://Home/Mail/token"}`},
		{"secrets", `{}`},
		{"secrets", `{"postgres":"op://Home/Postgres/password"}`},
		{"secrets", `{"secrets":{"postgres":"op://Home/Postgres/password"},"approval":"required"}`},
	}
	for _, c := range cases {
		vault := newFakeVault()
		recorder := post(newHandler(vault, "/", c.wrap), "/", c.body)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("wrap %q, %s: status %d", c.wrap, c.body, recorder.Code)
		}
		if vault.asked != 0 {
			t.Errorf("wrap %q, %s: asked 1Password anyway", c.wrap, c.body)
		}
	}
}

func TestAnswersOnlyOnItsPath(t *testing.T) {
	body := `{"postgres":"op://Home/Postgres/password"}`
	cases := []struct {
		path   string
		target string
		status int
	}{
		{"/", "/", http.StatusOK},
		{"/", "/secrets", http.StatusNotFound},
		{"/api/secrets", "/api/secrets", http.StatusOK},
		{"/api/secrets", "/", http.StatusNotFound},
		{"/api/secrets", "/api/secrets/more", http.StatusNotFound},
	}
	for _, c := range cases {
		if recorder := post(newHandler(newFakeVault(), c.path, ""), c.target, body); recorder.Code != c.status {
			t.Errorf("path %s, request to %s: status %d", c.path, c.target, recorder.Code)
		}
	}
}

func TestUnwrapsRequestsAndRepliesFlat(t *testing.T) {
	recorder := post(newHandler(newFakeVault(), "/", "secrets"), "/", `{"secrets":{"postgres":"op://Home/Postgres/password"}}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d", recorder.Code)
	}
	if values := decode[map[string]string](t, recorder); len(values) != 1 || values["postgres"] != "postgres-password" {
		t.Fatalf("got %v", values)
	}
}
