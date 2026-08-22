package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, dir string, command string, args ...string) (Result, error)
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ExitError struct {
	Command  string
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("%s exited with code %d", e.Command, e.ExitCode)
}

type ExecRunner struct{}

func (r ExecRunner) Run(ctx context.Context, dir string, command string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, &ExitError{
			Command:  command,
			Args:     append([]string(nil), args...),
			ExitCode: result.ExitCode,
			Stderr:   result.Stderr,
		}
	}

	result.ExitCode = -1
	return result, fmt.Errorf("start %s: %w", command, err)
}

