package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Client struct {
	runner Runner
}

func NewClient(runner Runner) *Client {
	return &Client{runner: runner}
}

func (c *Client) ValidateRepository(ctx context.Context, repo string, remote string) error {
	info, err := os.Stat(repo)
	if err != nil {
		return fmt.Errorf("stat repository: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repository path is not a directory: %s", repo)
	}

	if _, err := c.run(ctx, repo, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("validate git work tree: %w", err)
	}

	if _, err := c.run(ctx, repo, "remote", "get-url", remote); err != nil {
		return fmt.Errorf("validate remote %q: %w", remote, err)
	}

	return nil
}

func (c *Client) IsDirty(ctx context.Context, repo string) (bool, error) {
	out, err := c.run(ctx, repo, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("check working tree: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

func (c *Client) Fetch(ctx context.Context, repo string, remote string, branch string) error {
	if _, err := c.run(ctx, repo, "fetch", remote, branch); err != nil {
		return fmt.Errorf("fetch %s/%s: %w", remote, branch, err)
	}
	return nil
}

func (c *Client) Head(ctx context.Context, repo string) (string, error) {
	return c.revParse(ctx, repo, "HEAD")
}

func (c *Client) RemoteHead(ctx context.Context, repo string, remote string, branch string) (string, error) {
	return c.revParse(ctx, repo, remote+"/"+branch)
}

func (c *Client) MergeBase(ctx context.Context, repo string, left string, right string) (string, error) {
	out, err := c.run(ctx, repo, "merge-base", left, right)
	if err != nil {
		return "", fmt.Errorf("merge-base %s %s: %w", short(left), short(right), err)
	}
	sha := strings.TrimSpace(out)
	if !IsSHA(sha) {
		return "", fmt.Errorf("merge-base returned invalid SHA: %q", sha)
	}
	return sha, nil
}

func (c *Client) FastForward(ctx context.Context, repo string, remote string, branch string) error {
	if _, err := c.run(ctx, repo, "pull", "--ff-only", remote, branch); err != nil {
		return fmt.Errorf("fast-forward %s/%s: %w", remote, branch, err)
	}
	return nil
}

func (c *Client) revParse(ctx context.Context, repo string, ref string) (string, error) {
	out, err := c.run(ctx, repo, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	sha := strings.TrimSpace(out)
	if !IsSHA(sha) {
		return "", fmt.Errorf("rev-parse %s returned invalid SHA: %q", ref, sha)
	}
	return sha, nil
}

func (c *Client) run(ctx context.Context, repo string, args ...string) (string, error) {
	result, err := c.runner.Run(ctx, repo, "git", args...)
	if err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) && strings.TrimSpace(result.Stderr) != "" {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(result.Stderr))
		}
		return "", err
	}
	return result.Stdout, nil
}

func IsSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func short(sha string) string {
	if len(sha) < 7 {
		return sha
	}
	return sha[:7]
}

