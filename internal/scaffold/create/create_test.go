package create

import (
	"os"
	"path/filepath"
	"runtime"
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
