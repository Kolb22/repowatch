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
	dirty       bool
	local       string
	remote      string
	base        string
	fastForward bool
}

func (f *fakeGit) ValidateRepository(context.Context, string, string) error { return nil }
func (f *fakeGit) IsDirty(context.Context, string) (bool, error)            { return f.dirty, nil }
func (f *fakeGit) Fetch(context.Context, string, string, string) error      { return nil }
func (f *fakeGit) Head(context.Context, string) (string, error)             { return f.local, nil }
func (f *fakeGit) RemoteHead(context.Context, string, string, string) (string, error) {
	return f.remote, nil
}
func (f *fakeGit) MergeBase(context.Context, string, string, string) (string, error) {
	return f.base, nil
}
func (f *fakeGit) FastForward(context.Context, string, string, string) error {
	f.fastForward = true
	return nil
}

func TestServiceSyncStates(t *testing.T) {
	tests := []struct {
		name              string
		git               *fakeGit
		wantState         State
		wantCode          ExitCode
		wantFastForwarded bool
	}{
		{
			name:      "up to date",
			git:       &fakeGit{local: localSHA, remote: localSHA},
			wantState: StateUpToDate,
			wantCode:  ExitOK,
		},
		{
			name:              "behind fast forwards",
			git:               &fakeGit{local: localSHA, remote: remoteSHA, base: localSHA},
			wantState:         StateBehind,
			wantCode:          ExitOK,
			wantFastForwarded: true,
		},
		{
			name:      "ahead aborts",
			git:       &fakeGit{local: localSHA, remote: remoteSHA, base: remoteSHA},
			wantState: StateAhead,
			wantCode:  ExitAhead,
		},
		{
			name:      "diverged aborts",
			git:       &fakeGit{local: localSHA, remote: remoteSHA, base: "3333333333333333333333333333333333333333"},
			wantState: StateDiverged,
			wantCode:  ExitDiverged,
		},
		{
			name:      "dirty aborts before fetch",
			git:       &fakeGit{dirty: true},
			wantState: StateDirty,
			wantCode:  ExitDirty,
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
			if tt.git.fastForward != tt.wantFastForwarded {
				t.Fatalf("fastForward = %v, want %v", tt.git.fastForward, tt.wantFastForwarded)
			}
		})
	}
}

func TestCodeForError(t *testing.T) {
	if got := CodeFor(Result{}, errors.New("boom")); got != ExitError {
		t.Fatalf("CodeFor error = %d, want %d", got, ExitError)
	}
}

