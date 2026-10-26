package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestPlistReadsBackAsWritten(t *testing.T) {
	if _, err := exec.LookPath("plutil"); err != nil {
		t.Skip("plutil is only on macOS")
	}
	arguments := []string{"/opt/homebrew/bin/keyhole", "serve", "-account", "Home & <Away>", "-socket", "/Users/me/.keyhole/home.sock"}
	plist, err := renderPlist("com.github.carldaws.keyhole.home", arguments, "/Users/me/Library/Logs/keyhole-home.log")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "keyhole.plist")
	if err := os.WriteFile(path, plist, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	var job struct {
		Label             string
		ProgramArguments  []string
		RunAtLoad         bool
		KeepAlive         bool
		StandardErrorPath string
	}
	if err := json.Unmarshal(out, &job); err != nil {
		t.Fatal(err)
	}
	if job.Label != "com.github.carldaws.keyhole.home" ||
		!slices.Equal(job.ProgramArguments, arguments) ||
		!job.RunAtLoad || !job.KeepAlive ||
		job.StandardErrorPath != "/Users/me/Library/Logs/keyhole-home.log" {
		t.Fatalf("got %+v", job)
	}
}
