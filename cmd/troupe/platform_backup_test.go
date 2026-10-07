package troupe

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhu13818202655/troupe/pkg/backup"
	"github.com/zhu13818202655/troupe/pkg/config"
)

func TestParseBackupReadArgs(t *testing.T) {
	path, jsonOut, passphrase, err := parseBackupReadArgs([]string{"backup.troupe-backup", "--json", "--passphrase", "secret"})
	if err != nil {
		t.Fatalf("parseBackupReadArgs() error = %v", err)
	}
	if path != "backup.troupe-backup" {
		t.Fatalf("path = %q, want backup.troupe-backup", path)
	}
	if !jsonOut {
		t.Fatal("jsonOut = false, want true")
	}
	if passphrase != "secret" {
		t.Fatalf("passphrase = %q, want secret", passphrase)
	}
}

func TestParseBackupReadArgsRequiresArchive(t *testing.T) {
	if _, _, _, err := parseBackupReadArgs([]string{"--json"}); err == nil {
		t.Fatal("parseBackupReadArgs() error = nil, want missing archive error")
	}
}

func TestParseBackupCreateArgs(t *testing.T) {
	opts, err := parseBackupCreateArgs([]string{"--output", "backup.troupe-backup"})
	if err != nil {
		t.Fatalf("parseBackupCreateArgs() error = %v", err)
	}
	if opts.output != "backup.troupe-backup" {
		t.Fatalf("output = %q, want backup.troupe-backup", opts.output)
	}
	if opts.compression != backup.CompressionGzip {
		t.Fatalf("compression = %q, want gzip", opts.compression)
	}
}

func TestParseBackupCreateArgsCompressionNone(t *testing.T) {
	opts, err := parseBackupCreateArgs([]string{"--output", "backup.troupe-backup", "--compression", "none"})
	if err != nil {
		t.Fatalf("parseBackupCreateArgs() error = %v", err)
	}
	if opts.compression != backup.CompressionNone {
		t.Fatalf("compression = %q, want none", opts.compression)
	}
}

func TestParseBackupCreateArgsRedactSecrets(t *testing.T) {
	opts, err := parseBackupCreateArgs([]string{"--output", "backup.troupe-backup", "--redact-secrets", "--passphrase", "secret"})
	if err != nil {
		t.Fatalf("parseBackupCreateArgs() error = %v", err)
	}
	if !opts.redactSecrets {
		t.Fatal("redactSecrets = false, want true")
	}
	if opts.passphrase != "secret" {
		t.Fatalf("passphrase = %q, want secret", opts.passphrase)
	}
}

func TestParseBackupCreateArgsRejectsUnknownCompression(t *testing.T) {
	if _, err := parseBackupCreateArgs([]string{"--output", "backup.troupe-backup", "--compression", "zip"}); err == nil {
		t.Fatal("parseBackupCreateArgs() error = nil, want unsupported compression error")
	}
}

func TestParseBackupCreateArgsScopedBackup(t *testing.T) {
	opts, err := parseBackupCreateArgs([]string{"--org", "acme", "--team", "sre", "--output", "backup.troupe-backup"})
	if err != nil {
		t.Fatalf("parseBackupCreateArgs() error = %v", err)
	}
	if opts.orgSlug != "acme" || opts.teamSlug != "sre" {
		t.Fatalf("scope = %s/%s, want acme/sre", opts.orgSlug, opts.teamSlug)
	}
}

func TestParseBackupCreateArgsTeamRequiresOrg(t *testing.T) {
	if _, err := parseBackupCreateArgs([]string{"--team", "sre", "--output", "backup.troupe-backup"}); err == nil {
		t.Fatal("parseBackupCreateArgs() error = nil, want --team requires --org")
	}
}

func TestBackupEntstoreConfigDefaultsEmptyBackendToSQLite(t *testing.T) {
	cfg, backend, err := backupEntstoreConfig(&config.AppConfig{})
	if err != nil {
		t.Fatalf("backupEntstoreConfig() error = %v", err)
	}
	if backend != "sqlite" {
		t.Fatalf("backend = %q, want sqlite", backend)
	}
	if cfg.DSN == "" || cfg.DataDir == "" {
		t.Fatalf("config = %+v, want sqlite DSN and DataDir", cfg)
	}
}

func TestBackupEntstoreConfigDefaultsFileBackendToSQLite(t *testing.T) {
	cfg, backend, err := backupEntstoreConfig(&config.AppConfig{Storage: config.StorageConfig{Backend: "file"}})
	if err != nil {
		t.Fatalf("backupEntstoreConfig() error = %v", err)
	}
	if backend != "sqlite" {
		t.Fatalf("backend = %q, want sqlite", backend)
	}
	if cfg.DSN == "" || cfg.DataDir == "" {
		t.Fatalf("config = %+v, want sqlite DSN and DataDir", cfg)
	}
}

func TestHandlePlatformBackupVerify(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "test.troupe-backup")
	writer, err := backup.Create(archivePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := writer.AddFile("platform/users.jsonl", strings.NewReader("{}\n")); err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}
	manifest := backup.NewManifest("sqlite", "logical", []backup.Scope{{Kind: "platform"}})
	manifest.Entries = []backup.Entry{{Path: "platform/users.jsonl", Kind: "jsonl", Scope: backup.Scope{Kind: "platform"}, Entity: "users", Records: 1}}
	if err := writer.CloseWithManifest(manifest); err != nil {
		t.Fatalf("CloseWithManifest() error = %v", err)
	}

	if err := handlePlatformBackupVerify([]string{archivePath, "--json"}); err != nil {
		t.Fatalf("handlePlatformBackupVerify() error = %v", err)
	}
}
