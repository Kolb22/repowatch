package systemd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	validUser = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_.-]*\$?|[0-9]+)$`)
)

type Runner interface {
	Run(ctx context.Context, command string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return output.String(), fmt.Errorf("%s timed out: %w", command, ctx.Err())
		}
		return output.String(), fmt.Errorf("%s failed: %w: %s", command, err, strings.TrimSpace(output.String()))
	}
	return strings.TrimSpace(output.String()), nil
}

type Options struct {
	Name       string
	Repository string
	Remote     string
	Branch     string
	User       string
	Executable string
	Interval   time.Duration
}

type Result struct {
	ServiceName string
	TimerName   string
	Status      string
}

type Installer struct {
	runner  Runner
	unitDir string
}

func NewInstaller(runner Runner, unitDir string) *Installer {
	return &Installer{runner: runner, unitDir: unitDir}
}

func (i *Installer) Install(ctx context.Context, opts Options) (Result, error) {
	if err := validate(opts); err != nil {
		return Result{}, err
	}

	serviceName := "repowatch-" + opts.Name + ".service"
	timerName := "repowatch-" + opts.Name + ".timer"
	if err := writeUnit(filepath.Join(i.unitDir, serviceName), renderService(opts)); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", serviceName, err)
	}
	if err := writeUnit(filepath.Join(i.unitDir, timerName), renderTimer(serviceName, opts.Interval)); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", timerName, err)
	}

	if _, err := i.runner.Run(ctx, "systemctl", "daemon-reload"); err != nil {
		return Result{}, fmt.Errorf("reload systemd: %w", err)
	}
	if _, err := i.runner.Run(ctx, "systemctl", "enable", "--now", timerName); err != nil {
		return Result{}, fmt.Errorf("enable %s: %w", timerName, err)
	}
	status, err := i.runner.Run(ctx, "systemctl", "list-timers", "--all", "--no-pager", timerName)
	if err != nil {
		return Result{}, fmt.Errorf("show %s status: %w", timerName, err)
	}

	return Result{ServiceName: serviceName, TimerName: timerName, Status: status}, nil
}

func writeUnit(destination string, content string) (err error) {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".repowatch-unit-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()

	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	if _, err := temporary.WriteString(content); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, destination)
}

func NormalizeName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" {
		return "repository"
	}
	return name
}

func validate(opts Options) error {
	if !validName.MatchString(opts.Name) {
		return fmt.Errorf("invalid unit name %q", opts.Name)
	}
	if !path.IsAbs(opts.Repository) {
		return errors.New("--repo must be an absolute path")
	}
	if !path.IsAbs(opts.Executable) {
		return errors.New("repowatch executable path must be absolute")
	}
	if !validUser.MatchString(opts.User) {
		return errors.New("--user must be a valid Linux user")
	}
	if opts.Remote == "" || containsSpaceOrControl(opts.Remote) {
		return errors.New("--remote must not contain whitespace")
	}
	if opts.Branch == "" || containsSpaceOrControl(opts.Branch) {
		return errors.New("--branch must not contain whitespace")
	}
	if opts.Interval < time.Second || opts.Interval%time.Second != 0 {
		return errors.New("--interval must be a whole number of seconds")
	}
	if containsControl(opts.Repository) || containsControl(opts.Executable) {
		return errors.New("paths must not contain control characters")
	}
	return nil
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func containsSpaceOrControl(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0
}

func quote(value string) string {
	value = strings.ReplaceAll(value, "%", "%%")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func renderService(opts Options) string {
	return fmt.Sprintf(`[Unit]
Description=Synchronize %s repository using RepoWatch
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
User=%s
ExecStart=%s sync --repo %s --remote %s --branch %s
`, opts.Name, opts.User, quote(opts.Executable), quote(opts.Repository), quote(opts.Remote), quote(opts.Branch))
}

func renderTimer(serviceName string, interval time.Duration) string {
	seconds := int64(interval / time.Second)
	return fmt.Sprintf(`[Unit]
Description=Check repository for updates with RepoWatch

[Timer]
OnBootSec=30s
OnUnitActiveSec=%ds
AccuracySec=1s
Unit=%s

[Install]
WantedBy=timers.target
`, seconds, serviceName)
}

