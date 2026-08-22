package main

import (
	"bytes"
	"testing"

	syncpkg "github.com/Kolb22/repowatch/internal/sync"
)

func TestRunRequiresCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != int(syncpkg.ExitError) {
		t.Fatalf("code = %d, want %d", code, syncpkg.ExitError)
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty")
	}
}

func TestRunSyncRequiresRepo(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"sync"}, &stdout, &stderr)
	if code != int(syncpkg.ExitError) {
		t.Fatalf("code = %d, want %d", code, syncpkg.ExitError)
	}
	if stderr.String() != "ERROR: --repo is required\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunInstallRequiresRepo(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"install"}, &stdout, &stderr)
	if code != int(syncpkg.ExitError) {
		t.Fatalf("code = %d, want %d", code, syncpkg.ExitError)
	}
	if stderr.String() != "ERROR: --repo is required\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunUninstallRequiresName(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"uninstall"}, &stdout, &stderr)
	if code != int(syncpkg.ExitError) {
		t.Fatalf("code = %d, want %d", code, syncpkg.ExitError)
	}
	if stderr.String() != "ERROR: repository name is required\nusage: sudo repowatch uninstall NAME\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
