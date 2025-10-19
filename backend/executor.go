package backend

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

// CommandRunner is the interface for running commands.
type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin io.Reader) (stdout, stderr []byte, err error)
	Start(ctx context.Context, name string, args []string, stdin io.Reader) (stdout, stderr io.ReadCloser, wait func() error, err error)
}

// ExecCommandRunner uses os/exec.
type ExecCommandRunner struct{}

// Run runs a command.
func (ExecCommandRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	stdout, stderr, err = outBuf.Bytes(), errBuf.Bytes(), cmd.Run()
	return stdout, stderr, err
}

// Start starts a command.
func (ExecCommandRunner) Start(ctx context.Context, name string, args []string, stdin io.Reader) (stdout, stderr io.ReadCloser, wait func() error, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}

	return stdoutPipe, stderrPipe, cmd.Wait, nil
}

// Executor runs commands.
type Executor struct {
	runner     CommandRunner
	binaryPath string
	timeout    time.Duration
}

// NewExecutor creates an executor.
func NewExecutor(binaryPath string, timeout time.Duration) (*Executor, error) {
	if _, err := os.Stat(binaryPath); err != nil {
		return nil, fmt.Errorf("binary not found: %w", err)
	}

	return &Executor{
		binaryPath: binaryPath,
		timeout:    timeout,
		runner:     ExecCommandRunner{},
	}, nil
}

// NewExecutorWithRunner creates an executor with a custom runner.
func NewExecutorWithRunner(binaryPath string, timeout time.Duration, runner CommandRunner) *Executor {
	return &Executor{
		binaryPath: binaryPath,
		timeout:    timeout,
		runner:     runner,
	}
}

// Execute runs the command and returns output.
func (e *Executor) Execute(ctx context.Context, args []string, stdin io.Reader) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	return e.runner.Run(ctx, e.binaryPath, args, stdin)
}

// Stream runs the command and streams output line by line.
func (e *Executor) Stream(ctx context.Context, args []string, stdin io.Reader) (<-chan StreamChunk, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)

	stdout, stderr, wait, err := e.runner.Start(ctx, e.binaryPath, args, stdin)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("manager: executor: failed to start command: %w", err)
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer close(ch)
		defer cancel()

		// Read stderr in background
		stderrBuf := new(bytes.Buffer)
		stderrDone := make(chan struct{})
		go func() {
			if _, err := io.Copy(stderrBuf, stderr); err != nil {
				slog.Error("Failed to read stderr", "error", err)
			}
			close(stderrDone)
		}()

		// Stream stdout
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamChunk{Error: ctx.Err(), Done: true}
				return
			case ch <- StreamChunk{Data: append(scanner.Bytes(), '\n')}:
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: err, Done: true}
			return
		}

		<-stderrDone
		err := wait()

		if err != nil {
			if s := stderrBuf.String(); s != "" {
				ch <- StreamChunk{Error: fmt.Errorf("%w: %s", err, s), Done: true}
			} else {
				ch <- StreamChunk{Error: err, Done: true}
			}
		} else {
			ch <- StreamChunk{Done: true}
		}
	}()

	return ch, nil
}
