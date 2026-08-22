package systemd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	calls []string
}

func (f *fakeRunner) Run(_ context.Context, command string, args ...string) (string, error) {
	f.calls = append(f.calls, command+" "+strings.Join(args, " "))
	return "timer active", nil
}

func TestInstallerCreatesAndEnablesUnits(t *testing.T) {
	runner := &fakeRunner{}
	dir := t.TempDir()
	installer := NewInstaller(runner, dir)

	result, err := installer.Install(context.Background(), Options{
		Name:       "my-app",
		Repository: "/opt/my app",
		Remote:     "origin",
		Branch:     "main",
		User:       "deploy",
		Executable: "/usr/local/bin/repowatch",
		Interval:   45 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TimerName != "repowatch-my-app.timer" || result.Status != "timer active" {
		t.Fatalf("unexpected result: %+v", result)
	}

	service, err := os.ReadFile(filepath.Join(dir, result.ServiceName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(service), `--repo "/opt/my app"`) {
		t.Fatalf("service does not quote repository path:\n%s", service)
	}

	timer, err := os.ReadFile(filepath.Join(dir, result.TimerName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(timer), "OnUnitActiveSec=45s") {
		t.Fatalf("unexpected timer:\n%s", timer)
	}

	wantCalls := []string{
		"systemctl daemon-reload",
		"systemctl enable --now repowatch-my-app.timer",
		"systemctl list-timers --all --no-pager repowatch-my-app.timer",
	}
	if strings.Join(runner.calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestInstallerRejectsUnsafeOptions(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Options)
	}{
		{name: "unit name", change: func(o *Options) { o.Name = "../bad" }},
		{name: "relative repository", change: func(o *Options) { o.Repository = "app" }},
		{name: "invalid user", change: func(o *Options) { o.User = "bad user" }},
		{name: "remote whitespace", change: func(o *Options) { o.Remote = "bad remote" }},
		{name: "short interval", change: func(o *Options) { o.Interval = 500 * time.Millisecond }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Name: "app", Repository: "/opt/app", Remote: "origin", Branch: "main",
				User: "deploy", Executable: "/usr/local/bin/repowatch", Interval: 30 * time.Second,
			}
			tt.change(&opts)
			if _, err := NewInstaller(&fakeRunner{}, t.TempDir()).Install(context.Background(), opts); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	if got := NormalizeName("my app"); got != "my-app" {
		t.Fatalf("NormalizeName() = %q", got)
	}
}

func TestInstallerListsInstallationsByName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"repowatch-zeta.timer",
		"repowatch-alpha.timer",
		"repowatch-alpha.service",
		"unrelated.timer",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("unit"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	installations, err := NewInstaller(&fakeRunner{}, dir).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(installations) != 2 {
		t.Fatalf("installations = %#v", installations)
	}
	if installations[0].Name != "alpha" || installations[1].Name != "zeta" {
		t.Fatalf("installations are not sorted: %#v", installations)
	}
}

func TestInstallerUninstallRemovesOnlyUnits(t *testing.T) {
	runner := &fakeRunner{}
	dir := t.TempDir()
	repository := filepath.Join(dir, "nodesentinel")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	repositoryFile := filepath.Join(repository, "keep.txt")
	if err := os.WriteFile(repositoryFile, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"repowatch-nodesentinel.service",
		"repowatch-nodesentinel.timer",
		"unrelated.service",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("unit"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := NewInstaller(runner, dir).Uninstall(context.Background(), "nodesentinel")
	if err != nil {
		t.Fatal(err)
	}
	if result.ServiceName != "repowatch-nodesentinel.service" || result.TimerName != "repowatch-nodesentinel.timer" {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, name := range []string{result.ServiceName, result.TimerName} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s was not removed", name)
		}
	}
	for _, name := range []string{repositoryFile, filepath.Join(dir, "unrelated.service")} {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("unrelated file %s was changed: %v", name, err)
		}
	}

	wantCalls := []string{
		"systemctl disable --now repowatch-nodesentinel.timer",
		"systemctl daemon-reload",
	}
	if strings.Join(runner.calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestInstallerUninstallRejectsUnsafeName(t *testing.T) {
	runner := &fakeRunner{}
	if _, err := NewInstaller(runner, t.TempDir()).Uninstall(context.Background(), "../nodesentinel"); err == nil {
		t.Fatal("expected validation error")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}
