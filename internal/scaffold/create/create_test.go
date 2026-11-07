package create

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGeneratedEnvPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not enforced on Windows")
	}
	root := t.TempDir()
	if err := createFiles(root, "example.com/project"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf(".env exposes permissions to other users: %o", info.Mode().Perm())
	}
}

func TestGeneratedRouterHasNoWildcardCORS(t *testing.T) {
	root := t.TempDir()
	if err := createFiles(root, "example.com/project"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "internal", "router", "router.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, wildcard := range []string{"https://*", "http://*", "AllowOriginFunc"} {
		if strings.Contains(string(content), wildcard) {
			t.Errorf("generated router contains permissive CORS: %q", wildcard)
		}
	}
}

func TestCreateFilesPreservesExistingEnv(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		name := "regular"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".env")
			target := path
			if symlink {
				target = filepath.Join(t.TempDir(), "secret")
			}
			const secret = "DBSTRING=existing-secret\n"
			if err := os.WriteFile(target, []byte(secret), 0600); err != nil {
				t.Fatal(err)
			}
			if symlink {
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if err := createFiles(root, "example.com/project"); err == nil {
				t.Fatal("expected existing .env to be rejected")
			}
			got, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != secret {
				t.Fatal("existing secret was overwritten")
			}
		})
	}
}

func TestCreateFilesRejectsEscapingDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "internal")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := createFiles(root, "example.com/project"); err == nil {
		t.Error("escaping directory symlink was accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside project: %v, %v", entries, err)
	}
}
