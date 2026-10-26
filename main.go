package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"syscall"
)

const usage = `usage:
  keyhole serve -account <account> -socket <path> [-path /] [-wrap <key>]
  keyhole install <name> -account <account> [-socket <path>] [-path /] [-wrap <key>]
  keyhole uninstall <name>
  keyhole version
`

var version string

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "(devel)"
}

type usageError string

func (e usageError) Error() string { return string(e) }

type options struct {
	account, socket, path, wrap string
}

func (o *options) register(flags *flag.FlagSet) {
	flags.StringVar(&o.account, "account", "", "1Password account name or UUID")
	flags.StringVar(&o.socket, "socket", "", "Unix socket to serve the account on")
	flags.StringVar(&o.path, "path", "/", "path to answer requests on")
	flags.StringVar(&o.wrap, "wrap", "", "key that requests wrap their references in, if any")
}

var maxSocket = len(syscall.RawSockaddrUnix{}.Path) - 1

func (o *options) check() error {
	switch {
	case o.account == "":
		return usageError("-account is required")
	case o.socket == "":
		return usageError("-socket is required")
	case len(o.socket) > maxSocket:
		return usageError(fmt.Sprintf("-socket must be at most %d bytes long", maxSocket))
	case !strings.HasPrefix(o.path, "/"):
		return usageError("-path must start with /")
	}
	return nil
}

func (o *options) args() []string {
	args := []string{"-account", o.account, "-socket", o.socket}
	if o.path != "/" {
		args = append(args, "-path", o.path)
	}
	if o.wrap != "" {
		args = append(args, "-wrap", o.wrap)
	}
	return args
}

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	command := os.Args[1]
	args := os.Args[2:]
	var err error
	switch command {
	case "serve":
		err = runServe(args)
	case "install":
		err = runInstall(args)
	case "uninstall":
		err = runUninstall(args)
	case "version":
		fmt.Println(buildVersion())
	case "help", "-h", "-help", "--help":
		fmt.Print(usage)
	default:
		err = usageError(fmt.Sprintf("unknown command %q", command))
	}

	var invalid usageError
	switch {
	case errors.As(err, &invalid):
		fmt.Fprintf(os.Stderr, "keyhole: %s\n\n%s", err, usage)
		os.Exit(2)
	case err != nil:
		fmt.Fprintln(os.Stderr, "keyhole:", err)
		os.Exit(1)
	}
}

func runServe(args []string) error {
	var o options
	flags := flag.NewFlagSet("keyhole serve", flag.ExitOnError)
	o.register(flags)
	flags.Parse(args)
	if flags.NArg() > 0 {
		return usageError("serve takes no arguments besides its flags")
	}
	if err := o.check(); err != nil {
		return err
	}
	return serve(o.account, o.socket, o.path, o.wrap)
}

func runInstall(args []string) error {
	name, args, err := takeName(args)
	if err != nil {
		return err
	}
	var o options
	flags := flag.NewFlagSet("keyhole install", flag.ExitOnError)
	o.register(flags)
	flags.Parse(args)
	if flags.NArg() > 0 {
		return usageError("install takes one name, before its flags")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if o.socket == "" {
		o.socket = filepath.Join(home, ".keyhole", name+".sock")
	}
	if o.socket, err = filepath.Abs(o.socket); err != nil {
		return err
	}
	if err := o.check(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return newAgent(name, home).install(executable, o)
}

func runUninstall(args []string) error {
	name, args, err := takeName(args)
	if err != nil {
		return err
	}
	if len(args) > 0 {
		return usageError("uninstall takes only a name")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return newAgent(name, home).uninstall()
}

func takeName(args []string) (string, []string, error) {
	switch {
	case len(args) == 0:
		return "", nil, usageError("give the keyhole a name")
	case strings.HasPrefix(args[0], "-"):
		return "", nil, usageError("give the keyhole's name before its flags")
	case !validName.MatchString(args[0]):
		return "", nil, usageError("name the keyhole with lowercase letters, digits, - and _")
	}
	return args[0], args[1:], nil
}
