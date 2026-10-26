package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type agent struct {
	label, plistPath, logPath string
}

func newAgent(name, home string) agent {
	label := "com.github.carldaws.keyhole." + name
	return agent{
		label:     label,
		plistPath: filepath.Join(home, "Library", "LaunchAgents", label+".plist"),
		logPath:   filepath.Join(home, "Library", "Logs", "keyhole-"+name+".log"),
	}
}

func (a agent) install(executable string, o options) error {
	if _, err := a.unload(); err != nil {
		return err
	}
	if err := checkServable(o.socket); err != nil {
		return err
	}
	contents, err := renderPlist(a.label, append([]string{executable, "serve"}, o.args()...), a.logPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.plistPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(a.plistPath, contents, 0o644); err != nil {
		return err
	}
	return launchctl("bootstrap", domain(), a.plistPath)
}

func (a agent) uninstall() error {
	loaded, err := a.unload()
	if err != nil {
		return err
	}
	err = os.Remove(a.plistPath)
	if errors.Is(err, fs.ErrNotExist) {
		if loaded {
			return nil
		}
		return fmt.Errorf("%s is not installed", a.label)
	}
	return err
}

func (a agent) unload() (loaded bool, err error) {
	if exec.Command("launchctl", "print", a.target()).Run() != nil {
		return false, nil
	}
	return true, launchctl("bootout", a.target())
}

func (a agent) target() string {
	return domain() + "/" + a.label
}

func domain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func launchctl(args ...string) error {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}

var plistTemplate = template.Must(template.New("plist").Funcs(template.FuncMap{"xml": escapeXML}).Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{xml .Label}}</string>
	<key>ProgramArguments</key>
	<array>
	{{- range .Arguments}}
		<string>{{xml .}}</string>
	{{- end}}
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardErrorPath</key>
	<string>{{xml .Log}}</string>
</dict>
</plist>
`))

func renderPlist(label string, arguments []string, log string) ([]byte, error) {
	var plist bytes.Buffer
	err := plistTemplate.Execute(&plist, struct {
		Label     string
		Arguments []string
		Log       string
	}{label, arguments, log})
	return plist.Bytes(), err
}

func escapeXML(s string) string {
	var escaped strings.Builder
	xml.EscapeText(&escaped, []byte(s))
	return escaped.String()
}
