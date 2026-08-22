package sync

import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	localSHA  = "1111111111111111111111111111111111111111"
	remoteSHA = "2222222222222222222222222222222222222222"
)

type fakeGit struct {
	dirty          bool
	branch         string
	local          string
	remote         string
	base           string
	fetched        bool
	fastForwardSHA string
}

func (f *fakeGit) ValidateRepository(context.Context, string, string) error { return nil }
func (f *fakeGit) CurrentBranch(context.Context, string) (string, error)    { return f.branch, nil }
func (f *fakeGit) IsDirty(context.Context, string) (bool, error)            { return f.dirty, nil }
func (f *fakeGit) Fetch(context.Context, string, string, string) error {
	f.fetched = true
	return nil
}
func (f *fakeGit) Head(context.Context, string) (string, error) { return f.local, nil }
func (f *fakeGit) RemoteHead(context.Context, string, string, string) (string, error) {
	return f.remote, nil
}
func (f *fakeGit) MergeBase(context.Context, string, string, string) (string, error) {
	return f.base, nil
}
func (f *fakeGit) FastForward(_ context.Context, _ string, targetSHA string) error {
	f.fastForwardSHA = targetSHA
	return nil
}

func TestServiceSyncStates(t *testing.T) {
	tests := []struct {
		name               string
		git                *fakeGit
		wantState          State
		wantCode           ExitCode
		wantFastForwardSHA string
		wantFetched        bool
		wantReason         string
	}{
		{
			name:        "up to date",
			git:         &fakeGit{branch: "main", local: localSHA, remote: localSHA},
			wantState:   StateUpToDate,
			wantCode:    ExitOK,
			wantFetched: true,
		},
		{
			name:               "behind fast forwards to inspected SHA",
			git:                &fakeGit{branch: "main", local: localSHA, remote: remoteSHA, base: localSHA},
			wantState:          StateBehind,
			wantCode:           ExitOK,
			wantFastForwardSHA: remoteSHA,
			wantFetched:        true,
		},
		{
			name:        "ahead aborts",
			git:         &fakeGit{branch: "main", local: localSHA, remote: remoteSHA, base: remoteSHA},
			wantState:   StateAhead,
			wantCode:    ExitAhead,
			wantFetched: true,
		},
		{
			name:        "diverged aborts",
			git:         &fakeGit{branch: "main", local: localSHA, remote: remoteSHA, base: "3333333333333333333333333333333333333333"},
			wantState:   StateDiverged,
			wantCode:    ExitDiverged,
			wantFetched: true,
		},
		{
			name:      "dirty aborts before fetch",
			git:       &fakeGit{branch: "main", dirty: true},
			wantState: StateDirty,
			wantCode:  ExitDirty,
		},
		{
			name:       "wrong branch aborts before fetch",
			git:        &fakeGit{branch: "develop"},
			wantState:  StateWrongBranch,
			wantCode:   ExitWrongBranch,
			wantReason: `current branch "develop" does not match configured branch "main"`,
		},
		{
			name:       "detached head aborts before fetch",
			git:        &fakeGit{},
			wantState:  StateWrongBranch,
			wantCode:   ExitWrongBranch,
			wantReason: `detached HEAD does not match configured branch "main"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.git)
			result, err := service.Sync(context.Background(), Options{
				Repository: "/tmp/repo",
				Remote:     "origin",
				Branch:     "main",
				Timeout:    time.Second,
			})
			if err != nil {
				t.Fatalf("Sync returned error: %v", err)
			}
			if result.State != tt.wantState {
				t.Fatalf("state = %s, want %s", result.State, tt.wantState)
			}
			if got := CodeFor(result, nil); got != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", got, tt.wantCode)
			}
			if tt.git.fastForwardSHA != tt.wantFastForwardSHA {
				t.Fatalf("fast-forward SHA = %q, want %q", tt.git.fastForwardSHA, tt.wantFastForwardSHA)
			}
			if tt.git.fetched != tt.wantFetched {
				t.Fatalf("fetched = %v, want %v", tt.git.fetched, tt.wantFetched)
			}
			if tt.wantReason != "" && result.Reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", result.Reason, tt.wantReason)
			}
		})
	}
}

func TestCodeForError(t *testing.T) {
	if got := CodeFor(Result{}, errors.New("boom")); got != ExitError {
		t.Fatalf("CodeFor error = %d, want %d", got, ExitError)
	}
}
