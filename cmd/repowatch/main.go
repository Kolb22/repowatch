package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	gitpkg "github.com/Kolb22/repowatch/internal/git"
	syncpkg "github.com/Kolb22/repowatch/internal/sync"
	systemdpkg "github.com/Kolb22/repowatch/internal/systemd"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return int(syncpkg.ExitError)
	}

	switch args[0] {
	case "sync":
		return runSync(args[1:], stdout, stderr)
	case "install":
		return runInstall(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		printUsage(stderr)
		return int(syncpkg.ExitError)
	}
}

func runInstall(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)

	repo := fs.String("repo", "", "absolute path to local Git repository")
	remote := fs.String("remote", "origin", "Git remote")
	branch := fs.String("branch", "main", "remote branch")
	interval := fs.Duration("interval", 30*time.Second, "polling interval")
	name := fs.String("name", "", "systemd unit identifier (defaults to repository directory name)")
	serviceUser := fs.String("user", defaultServiceUser(), "Linux user that owns the repository")

	if err := fs.Parse(args); err != nil {
		return int(syncpkg.ExitError)
	}
	if *repo == "" {
		fmt.Fprintln(stderr, "ERROR: --repo is required")
		return int(syncpkg.ExitError)
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintln(stderr, "ERROR: repowatch install is only supported on Linux")
		return int(syncpkg.ExitError)
	}
	info, err := os.Stat(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: inspect repository: %v\n", err)
		return int(syncpkg.ExitError)
	}
	if !info.IsDir() {
		fmt.Fprintln(stderr, "ERROR: --repo must point to a directory")
		return int(syncpkg.ExitError)
	}

	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: locate repowatch executable: %v\n", err)
		return int(syncpkg.ExitError)
	}

	unitName := *name
	if unitName == "" {
		unitName = systemdpkg.NormalizeName(filepath.Base(filepath.Clean(*repo)))
	}

	installer := systemdpkg.NewInstaller(systemdpkg.ExecRunner{}, "/etc/systemd/system")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := installer.Install(ctx, systemdpkg.Options{
		Name:       unitName,
		Repository: *repo,
		Remote:     *remote,
		Branch:     *branch,
		User:       *serviceUser,
		Executable: executable,
		Interval:   *interval,
	})
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: %v\n", err)
		fmt.Fprintln(stderr, "Hint: run this command with sudo.")
		return int(syncpkg.ExitError)
	}

	fmt.Fprintf(stdout, "Installed: %s and %s\n", result.ServiceName, result.TimerName)
	fmt.Fprintf(stdout, "Polling: every %s\n", interval.String())
	if result.Status != "" {
		fmt.Fprintln(stdout, result.Status)
	}
	return int(syncpkg.ExitOK)
}

func runSync(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)

	repo := fs.String("repo", "", "path to local Git repository")
	remote := fs.String("remote", "origin", "Git remote")
	branch := fs.String("branch", "main", "remote branch")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum time for Git operations")
	quiet := fs.Bool("quiet", false, "suppress successful output")

	if err := fs.Parse(args); err != nil {
		return int(syncpkg.ExitError)
	}
	if *repo == "" {
		fmt.Fprintln(stderr, "ERROR: --repo is required")
		return int(syncpkg.ExitError)
	}

	client := gitpkg.NewClient(gitpkg.ExecRunner{})
	service := syncpkg.NewService(client)
	result, err := service.Sync(context.Background(), syncpkg.Options{
		Repository: *repo,
		Remote:     *remote,
		Branch:     *branch,
		Timeout:    *timeout,
	})

	code := syncpkg.CodeFor(result, err)
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: %v\n", err)
		return int(code)
	}

	if !*quiet || code != syncpkg.ExitOK {
		printResult(stdout, result)
	}
	return int(code)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  repowatch sync --repo /path/to/repo [--remote origin] [--branch main] [--timeout 30s] [--quiet]")
	fmt.Fprintln(w, "  sudo repowatch install --repo /path/to/repo [--interval 30s] [--user USER] [--name NAME]")
}

func defaultServiceUser() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
		return sudoUser
	}
	current, err := user.Current()
	if err != nil {
		return ""
	}
	return current.Username
}

func printResult(w io.Writer, result syncpkg.Result) {
	fmt.Fprintf(w, "Repository: %s\n", result.Repository)
	fmt.Fprintf(w, "Remote: %s/%s\n", result.Remote, result.Branch)
	if result.Updated {
		fmt.Fprintln(w, "Status: UPDATED")
		fmt.Fprintf(w, "Previous: %s\n", abbreviate(result.Previous))
		fmt.Fprintf(w, "Current:  %s\n", abbreviate(result.Current))
		return
	}
	fmt.Fprintf(w, "Status: %s\n", result.State)
	if result.Reason != "" {
		fmt.Fprintf(w, "Reason: %s\n", result.Reason)
	}
}

func abbreviate(sha string) string {
	if len(sha) < 7 {
		return sha
	}
	return sha[:7]
}
