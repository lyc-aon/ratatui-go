package client

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Command is an argv vector for the Bun OMP core. No shell is involved:
// Path is the executable, Args are passed verbatim (not including Path).
type Command struct {
	// Path is the executable path or name looked up on PATH.
	Path string
	// Args are the process arguments excluding Path (argv[1:]).
	Args []string
	// Dir is the child working directory. Empty keeps the parent cwd.
	Dir string
	// Env is the child environment as KEY=VAL entries.
	// nil inherits the parent environment; non-nil replaces it entirely.
	// Use append(os.Environ(), "K=V") to extend.
	Env []string
}

// Process is the narrow child-process surface the client needs.
// Tests inject fakes via [Options.ProcessFactory].
type Process interface {
	// Start launches the process. Stdin/Stdout/Stderr must already be wired.
	Start() error
	// Wait blocks until the process exits. It is called exactly once by the client.
	Wait() error
	// Kill forcefully terminates the process tree best-effort.
	Kill() error
	// Stdin is the write end of the child's stdin pipe (JSONL commands).
	Stdin() io.WriteCloser
	// Stdout is the read end of the child's stdout pipe (JSONL events/responses).
	Stdout() io.ReadCloser
	// PID returns the OS process id, or 0 if unknown.
	PID() int
	// ExitCode returns the exit code after Wait; -1 if not yet available.
	ExitCode() int
}

// ProcessFactory constructs a Process for opts.Command.
// The factory must create stdin/stdout pipes and attach stderr as directed by
// [ProcessSpec] before returning. The client calls Start.
type ProcessFactory func(ctx context.Context, spec ProcessSpec) (Process, error)

// ProcessSpec is the spawn request handed to a [ProcessFactory].
type ProcessSpec struct {
	Command Command
	// Stderr is where the child stderr should be written.
	// Default factory uses this directly as cmd.Stderr (no pipe).
	Stderr io.Writer
	// ExtraFiles, if non-nil, are attached as extra inherited files (rare).
	ExtraFiles []*os.File
}

// execProcess is the default os/exec-backed Process.
type execProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	once    sync.Once
	waitErr error
	code    int
}

// DefaultProcessFactory builds an exec.Cmd process with stdin/stdout pipes
// and stderr attached to spec.Stderr (or os.Stderr when nil).
func DefaultProcessFactory(ctx context.Context, spec ProcessSpec) (Process, error) {
	if spec.Command.Path == "" {
		return nil, ErrInvalidCommand
	}
	// CommandContext: parent cancel after Start still kills the child if the
	// Start ctx is cancelled before Wait. Graceful shutdown is via stdin close;
	// the Start ctx should outlive normal operation (use context.Background or
	// a long-lived parent).
	cmd := exec.CommandContext(ctx, spec.Command.Path, spec.Command.Args...)
	if spec.Command.Dir != "" {
		cmd.Dir = spec.Command.Dir
	}
	if spec.Command.Env != nil {
		cmd.Env = spec.Command.Env
	}
	stderr := spec.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	cmd.Stderr = stderr
	if len(spec.ExtraFiles) > 0 {
		cmd.ExtraFiles = spec.ExtraFiles
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	return &execProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		code:   -1,
	}, nil
}

func (p *execProcess) Start() error {
	if p == nil || p.cmd == nil {
		return ErrNoProcess
	}
	return p.cmd.Start()
}

func (p *execProcess) Wait() error {
	if p == nil || p.cmd == nil {
		return ErrNoProcess
	}
	p.once.Do(func() {
		p.waitErr = p.cmd.Wait()
		if p.cmd.ProcessState != nil {
			p.code = p.cmd.ProcessState.ExitCode()
		} else {
			p.code = -1
		}
	})
	return p.waitErr
}

func (p *execProcess) Kill() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *execProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *execProcess) Stdout() io.ReadCloser { return p.stdout }

func (p *execProcess) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *execProcess) ExitCode() int {
	if p == nil {
		return -1
	}
	return p.code
}
