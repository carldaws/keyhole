package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func socketTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "keyhole")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestListenCreatesAPrivateDirectory(t *testing.T) {
	dir := filepath.Join(socketTempDir(t), "sockets")

	listener, err := listen(filepath.Join(dir, "work.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("directory mode %o", perm)
	}
}

func TestListenRefusesASharedDirectory(t *testing.T) {
	dir := socketTempDir(t)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := listen(filepath.Join(dir, "work.sock")); err == nil || !strings.Contains(err.Error(), "must be private") {
		t.Fatalf("got %v", err)
	}
}

func TestListenReplacesAStaleSocket(t *testing.T) {
	socket := filepath.Join(socketTempDir(t), "work.sock")
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	stale.Close()

	listener, err := listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestListenLeavesALiveSocketAlone(t *testing.T) {
	socket := filepath.Join(socketTempDir(t), "work.sock")
	running, err := listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()

	if _, err := listen(socket); err == nil || !strings.Contains(err.Error(), "already being served") {
		t.Fatalf("got %v", err)
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatalf("the running socket is gone: %v", err)
	}
	conn.Close()
}

func TestListenLeavesOtherFilesAlone(t *testing.T) {
	path := filepath.Join(socketTempDir(t), "notes.txt")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := listen(path); err == nil {
		t.Fatal("listened over a regular file")
	}
	if contents, _ := os.ReadFile(path); string(contents) != "keep me" {
		t.Fatalf("file now holds %q", contents)
	}
}
