package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func serve(account, socket, path, wrap string) error {
	listener, err := listen(socket)
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           newHandler(newOnePassword(account), path, wrap),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return server.Serve(listener)
}

func listen(socket string) (net.Listener, error) {
	if err := checkServable(socket); err != nil {
		return nil, err
	}
	return net.Listen("unix", socket)
}

func checkServable(socket string) error {
	if err := privateDir(filepath.Dir(socket)); err != nil {
		return err
	}
	return removeStaleSocket(socket)
}

func privateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s must be private to you, so others can't reach the socket (chmod 700 %[1]s)", dir)
	}
	return nil
}

func removeStaleSocket(socket string) error {
	info, err := os.Lstat(socket)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().Type() != fs.ModeSocket {
		return fmt.Errorf("%s exists and is not a socket", socket)
	}
	if conn, err := net.Dial("unix", socket); err == nil {
		conn.Close()
		return fmt.Errorf("%s is already being served", socket)
	}
	return os.Remove(socket)
}
