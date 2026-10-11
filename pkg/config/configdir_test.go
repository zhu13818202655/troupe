package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigDir(t *testing.T) {
	tests := []struct {
		name string
		// files to create in each candidate directory; a nil slice means the
		// directory itself is not created.
		currentFiles []string
		legacyFiles  []string
		wantSuffix   string
	}{
		{
			name:       "fresh install uses the new directory",
			wantSuffix: configDirName,
		},
		{
			name:        "legacy with state is reused",
			legacyFiles: []string{"config.yaml"},
			wantSuffix:  legacyConfigDirName,
		},
		{
			name:        "empty legacy directory is not reused",
			legacyFiles: []string{},
			wantSuffix:  configDirName,
		},
		{
			name:         "current with state wins over legacy",
			currentFiles: []string{"config.yaml"},
			legacyFiles:  []string{"config.yaml"},
			wantSuffix:   configDirName,
		},
		{
			// Regression guard: an empty troupe dir created by a bind mount or
			// `mkdir -p` must not shadow a populated legacy dir, otherwise the
			// credential store is silently orphaned (a fresh .store_key cannot
			// decrypt the old credentials.enc).
			name:        "empty current does not shadow populated legacy",
			legacyFiles: []string{".store_key"},
			wantSuffix:  legacyConfigDirName,
		},
		{
			name:         "current holding only credentials wins",
			currentFiles: []string{"credentials.enc"},
			legacyFiles:  []string{"config.yaml"},
			wantSuffix:   configDirName,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()

			write := func(dir string, files []string) {
				if files == nil {
					return
				}
				full := filepath.Join(base, dir)
				if err := os.MkdirAll(full, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", full, err)
				}
				for _, f := range files {
					if err := os.WriteFile(filepath.Join(full, f), []byte("x"), 0o600); err != nil {
						t.Fatalf("write %s: %v", f, err)
					}
				}
			}
			write(configDirName, tc.currentFiles)
			write(legacyConfigDirName, tc.legacyFiles)

			got := resolveConfigDir(base)
			if got != filepath.Join(base, tc.wantSuffix) {
				t.Fatalf("resolveConfigDir() = %q, want suffix %q", got, tc.wantSuffix)
			}
		})
	}
}

func TestResolveConfigDirHonorsXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}
	if want := filepath.Join(xdg, configDirName); dir != want {
		t.Fatalf("GetConfigDir() = %q, want %q", dir, want)
	}
}
