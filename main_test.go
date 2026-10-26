package main

import (
	"slices"
	"strings"
	"testing"
)

func TestOptionsLeaveOutDefaults(t *testing.T) {
	cases := []struct {
		name    string
		options options
		want    []string
	}{
		{"defaults", options{account: "Home", socket: "/s", path: "/"}, []string{"-account", "Home", "-socket", "/s"}},
		{"path and wrap", options{account: "Home", socket: "/s", path: "/api/secrets", wrap: "secrets"}, []string{"-account", "Home", "-socket", "/s", "-path", "/api/secrets", "-wrap", "secrets"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.options.args(); !slices.Equal(got, c.want) {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestOptionsRefuseASocketTooLongToBind(t *testing.T) {
	fits := options{account: "Home", socket: "/" + strings.Repeat("s", maxSocket-1), path: "/"}
	if err := fits.check(); err != nil {
		t.Fatal(err)
	}
	tooLong := options{account: "Home", socket: "/" + strings.Repeat("s", maxSocket), path: "/"}
	if err := tooLong.check(); err == nil {
		t.Fatal("took a socket path too long to bind")
	}
}

func TestTakeNameRefusesNamesUnfitForPaths(t *testing.T) {
	for _, args := range [][]string{{}, {"-account", "Home"}, {"../home"}, {"home work"}, {""}, {"Home"}} {
		if _, _, err := takeName(args); err == nil {
			t.Errorf("took a name from %q", args)
		}
	}
	name, rest, err := takeName([]string{"home", "-account", "Home"})
	if err != nil || name != "home" || !slices.Equal(rest, []string{"-account", "Home"}) {
		t.Fatalf("got %q %q %v", name, rest, err)
	}
}
