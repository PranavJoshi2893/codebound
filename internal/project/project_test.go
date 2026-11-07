package project

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOptionsValidate(t *testing.T) {
	for _, options := range []Options{
		{RootDir: "my-project"},
		{RootDir: "my_project", Mod: "github.com/user/project/v2"},
	} {
		if err := options.Validate(); err != nil {
			t.Errorf("valid options %+v: %v", options, err)
		}
	}
	for _, options := range []Options{
		{RootDir: "tiny"}, {RootDir: "1project"}, {RootDir: "../project"},
		{RootDir: "my project"}, {RootDir: "project", Mod: "github.com/user/my project"},
		{RootDir: "project", Mod: "github.com/user/project/v1"},
		{RootDir: "project", Mod: "-h"},
	} {
		if err := options.Validate(); err == nil {
			t.Errorf("accepted invalid options %+v", options)
		}
	}
}

func TestCreateUsesScaffold(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test tool stubs use Unix shell scripts")
	}
	root := t.TempDir()
	t.Chdir(root)
	tools := t.TempDir()
	// Keep this test offline while exercising the real generator and templates.
	goTool := "#!/bin/sh\nif [ \"$1 $2\" = \"mod init\" ]; then printf 'module %s\\n' \"$4\" > go.mod; fi\n"
	for name, script := range map[string]string{"go": goTool, "git": "#!/bin/sh\nexit 0\n"} {
		if err := os.WriteFile(filepath.Join(tools, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, options := range []Options{
		{RootDir: "default-project"},
		{RootDir: "custom-project", Mod: "github.com/user/project"},
	} {
		if err := Create(options); err != nil {
			t.Fatal(err)
		}
		path := options.Mod
		if path == "" {
			path = options.RootDir
		}
		content, err := os.ReadFile(filepath.Join(options.RootDir, "go.mod"))
		if err != nil || !strings.Contains(string(content), "module "+path) {
			t.Fatalf("unexpected go.mod: %s, %v", content, err)
		}
		if _, err := os.Stat(filepath.Join(options.RootDir, "cmd", "api", "main.go")); err != nil {
			t.Fatal(err)
		}
		if err := Create(options); err == nil {
			t.Fatal("existing project directory was not rejected")
		}
	}
}

func TestCreateValidatesBeforeFilesystemChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid-project")
	if err := Create(Options{RootDir: path}); err == nil {
		t.Fatal("expected invalid project name to fail")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid creation touched the filesystem: %v", err)
	}
}
