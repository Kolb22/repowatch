package sync

import (
	"context"
	"fmt"
	"time"
)

type State string

const (
	StateUpToDate State = "UP_TO_DATE"
	StateBehind   State = "BEHIND"
	StateAhead    State = "AHEAD"
	StateDiverged State = "DIVERGED"
	StateDirty    State = "DIRTY"
)

type ExitCode int

const (
	ExitOK       ExitCode = 0
	ExitError    ExitCode = 1
	ExitDirty    ExitCode = 2
	ExitAhead    ExitCode = 3
	ExitDiverged ExitCode = 4
)

type GitClient interface {
	ValidateRepository(ctx context.Context, repo string, remote string) error
	IsDirty(ctx context.Context, repo string) (bool, error)
	Fetch(ctx context.Context, repo string, remote string, branch string) error
	Head(ctx context.Context, repo string) (string, error)
	RemoteHead(ctx context.Context, repo string, remote string, branch string) (string, error)
	MergeBase(ctx context.Context, repo string, left string, right string) (string, error)
	FastForward(ctx context.Context, repo string, remote string, branch string) error
}

type Options struct {
	Repository string
	Remote     string
	Branch     string
	Timeout    time.Duration
}

type Result struct {
	Repository string
	Remote     string
	Branch     string
	State      State
	Previous   string
	Current    string
	Updated    bool
	Reason     string
}

type Service struct {
	git GitClient
}

func NewService(git GitClient) *Service {
	return &Service{git: git}
}

func (s *Service) Sync(parent context.Context, opts Options) (Result, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()

	result := Result{
		Repository: opts.Repository,
		Remote:     opts.Remote,
		Branch:     opts.Branch,
	}

	if err := s.git.ValidateRepository(ctx, opts.Repository, opts.Remote); err != nil {
		return result, fmt.Errorf("validate repository: %w", err)
	}

	dirty, err := s.git.IsDirty(ctx, opts.Repository)
	if err != nil {
		return result, err
	}
	if dirty {
		result.State = StateDirty
		result.Reason = "local changes detected; synchronization aborted"
		return result, nil
	}

	if err := s.git.Fetch(ctx, opts.Repository, opts.Remote, opts.Branch); err != nil {
		return result, err
	}

	local, err := s.git.Head(ctx, opts.Repository)
	if err != nil {
		return result, err
	}
	remote, err := s.git.RemoteHead(ctx, opts.Repository, opts.Remote, opts.Branch)
	if err != nil {
		return result, err
	}

	result.Previous = local
	result.Current = remote

	if local == remote {
		result.State = StateUpToDate
		return result, nil
	}

	base, err := s.git.MergeBase(ctx, opts.Repository, local, remote)
	if err != nil {
		return result, err
	}

	switch base {
	case local:
		result.State = StateBehind
		if err := s.git.FastForward(ctx, opts.Repository, opts.Remote, opts.Branch); err != nil {
			return result, err
		}
		result.State = StateBehind
		result.Updated = true
		result.Current = remote
		return result, nil
	case remote:
		result.State = StateAhead
		result.Reason = fmt.Sprintf("local repository contains commits not present on %s/%s", opts.Remote, opts.Branch)
		return result, nil
	default:
		result.State = StateDiverged
		result.Reason = "local and remote histories differ"
		return result, nil
	}
}

func CodeFor(result Result, err error) ExitCode {
	if err != nil {
		return ExitError
	}
	switch result.State {
	case StateDirty:
		return ExitDirty
	case StateAhead:
		return ExitAhead
	case StateDiverged:
		return ExitDiverged
	default:
		return ExitOK
	}
}

