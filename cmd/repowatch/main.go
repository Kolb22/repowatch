package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	gitpkg "github.com/Kolb22/repowatch/internal/git"
	syncpkg "github.com/Kolb22/repowatch/internal/sync"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: repowatch sync --repo /path/to/repo [--remote origin] [--branch main]")
		return int(syncpkg.ExitError)
	}

	switch args[0] {
	case "sync":
		return runSync(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		printUsage(stderr)
		return int(syncpkg.ExitError)
	}
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
	fmt.Fprintln(w, "usage: repowatch sync --repo /path/to/repo [--remote origin] [--branch main] [--timeout 30s] [--quiet]")
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

