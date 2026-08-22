package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsSHA(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "valid", in: "0123456789abcdef0123456789abcdef01234567", want: true},
		{name: "uppercase invalid", in: "0123456789ABCDEF0123456789ABCDEF01234567", want: false},
		{name: "short invalid", in: "abc", want: false},
		{name: "empty invalid", in: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSHA(tt.in); got != tt.want {
				t.Fatalf("IsSHA(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestExecRunner(t *testing.T) {
	runner := ExecRunner{}

	t.Run("successful command", func(t *testing.T) {
		result, err := runner.Run(context.Background(), "", "git", "--version")
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("exit code = %d, want 0", result.ExitCode)
		}
		if !strings.Contains(result.Stdout, "git version") {
			t.Fatalf("stdout = %q, want git version", result.Stdout)
		}
	})

	t.Run("non-zero command", func(t *testing.T) {
		result, err := runner.Run(context.Background(), "", "git", "definitely-not-a-git-command")
		if err == nil {
			t.Fatal("Run returned nil error")
		}
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("error type = %T, want *ExitError", err)
		}
		if result.ExitCode == 0 {
			t.Fatal("exit code = 0, want non-zero")
		}
	})

	t.Run("executable not found", func(t *testing.T) {
		_, err := runner.Run(context.Background(), "", "repowatch-no-such-executable")
		if err == nil {
			t.Fatal("Run returned nil error")
		}
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("error type = %T, did not expect *ExitError", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		_, err := runner.Run(ctx, "", "git", "--version")
		if err == nil {
			t.Fatal("Run returned nil error")
		}
	})
}

func TestClientWithLocalRepositories(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	ctx := context.Background()
	remote := filepath.Join(t.TempDir(), "remote.git")
	work := filepath.Join(t.TempDir(), "work")
	clone := filepath.Join(t.TempDir(), "clone")

	runGit(t, "", "init", "--bare", remote)
	runGit(t, "", "clone", remote, work)
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "RepoWatch Test")
	mustWrite(t, filepath.Join(work, "README.md"), "hello\n")
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "initial")
	runGit(t, work, "branch", "-M", "main")
	runGit(t, work, "push", "-u", "origin", "main")
	runGit(t, "", "clone", "--branch", "main", remote, clone)

	client := NewClient(ExecRunner{})
	if err := client.ValidateRepository(ctx, clone, "origin"); err != nil {
		t.Fatalf("ValidateRepository returned error: %v", err)
	}
	branch, err := client.CurrentBranch(ctx, clone)
	if err != nil {
		t.Fatalf("CurrentBranch returned error: %v", err)
	}
	if branch != "main" {
		t.Fatalf("branch = %q, want main", branch)
	}

	dirty, err := client.IsDirty(ctx, clone)
	if err != nil {
		t.Fatalf("IsDirty returned error: %v", err)
	}
	if dirty {
		t.Fatal("IsDirty = true, want false")
	}

	head, err := client.Head(ctx, clone)
	if err != nil {
		t.Fatalf("Head returned error: %v", err)
	}
	remoteHead, err := client.RemoteHead(ctx, clone, "origin", "main")
	if err != nil {
		t.Fatalf("RemoteHead returned error: %v", err)
	}
	if head != remoteHead {
		t.Fatalf("head = %s, remote = %s", head, remoteHead)
	}

	base, err := client.MergeBase(ctx, clone, head, remoteHead)
	if err != nil {
		t.Fatalf("MergeBase returned error: %v", err)
	}
	if base != head {
		t.Fatalf("base = %s, want %s", base, head)
	}

	mustWrite(t, filepath.Join(work, "remote-a.txt"), "commit A\n")
	runGit(t, work, "add", "remote-a.txt")
	runGit(t, work, "commit", "-m", "remote commit A")
	runGit(t, work, "push", "origin", "main")
	if err := client.Fetch(ctx, clone, "origin", "main"); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	targetSHA, err := client.RemoteHead(ctx, clone, "origin", "main")
	if err != nil {
		t.Fatalf("RemoteHead after fetch returned error: %v", err)
	}

	mustWrite(t, filepath.Join(work, "remote-b.txt"), "commit B\n")
	runGit(t, work, "add", "remote-b.txt")
	runGit(t, work, "commit", "-m", "remote commit B")
	runGit(t, work, "push", "origin", "main")
	newerRemoteSHA := gitOutput(t, work, "rev-parse", "HEAD")
	if newerRemoteSHA == targetSHA {
		t.Fatal("second remote commit did not advance HEAD")
	}

	if err := client.FastForward(ctx, clone, targetSHA); err != nil {
		t.Fatalf("FastForward returned error: %v", err)
	}
	updatedHead, err := client.Head(ctx, clone)
	if err != nil {
		t.Fatalf("Head after FastForward returned error: %v", err)
	}
	if updatedHead != targetSHA {
		t.Fatalf("updated HEAD = %s, want inspected target %s", updatedHead, targetSHA)
	}
	if updatedHead == newerRemoteSHA {
		t.Fatal("fast-forward unexpectedly used the newer, uninspected remote commit")
	}

	runGit(t, clone, "checkout", "--detach", targetSHA)
	branch, err = client.CurrentBranch(ctx, clone)
	if err != nil {
		t.Fatalf("CurrentBranch for detached HEAD returned error: %v", err)
	}
	if branch != "" {
		t.Fatalf("detached branch = %q, want empty", branch)
	}
	runGit(t, clone, "switch", "main")

	mustWrite(t, filepath.Join(clone, "local.txt"), "dirty\n")
	dirty, err = client.IsDirty(ctx, clone)
	if err != nil {
		t.Fatalf("IsDirty after write returned error: %v", err)
	}
	if !dirty {
		t.Fatal("IsDirty = false, want true")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

