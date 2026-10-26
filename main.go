package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	account := flag.String("account", "", "1Password account name or UUID")
	socket := flag.String("socket", "", "Unix socket to serve the account on")
	path := flag.String("path", "/", "path to answer requests on")
	wrap := flag.String("wrap", "", "key that requests wrap their references in, if any")
	flag.Parse()
	if *account == "" || *socket == "" || flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}
	if !strings.HasPrefix(*path, "/") {
		fmt.Fprintln(os.Stderr, "keyhole: -path must start with /")
		os.Exit(2)
	}

	if err := serve(*account, *socket, *path, *wrap); err != nil {
		fmt.Fprintln(os.Stderr, "keyhole:", err)
		os.Exit(1)
	}
}
