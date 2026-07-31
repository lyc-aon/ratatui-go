package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/michaelkelly/ratatui-go/ompui/app"
)

func captureStdoutStderr(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wOut, wErr
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
	}()

	var outBuf, errBuf bytes.Buffer
	outDone := make(chan struct{})
	errDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&outBuf, rOut)
		close(outDone)
	}()
	go func() {
		_, _ = io.Copy(&errBuf, rErr)
		close(errDone)
	}()

	fn()
	_ = wOut.Close()
	_ = wErr.Close()
	<-outDone
	<-errDone
	_ = rOut.Close()
	_ = rErr.Close()
	return outBuf.String(), errBuf.String()
}

func TestRunHelp(t *testing.T) {
	out, _ := captureStdoutStderr(t, func() {
		if code := run([]string{"--help"}); code != app.ExitOK {
			t.Errorf("code=%d", code)
		}
	})
	if !strings.Contains(out, "omp-tui") {
		t.Fatalf("help out=%q", out)
	}
}

func TestRunVersion(t *testing.T) {
	out, _ := captureStdoutStderr(t, func() {
		if code := run([]string{"--version"}); code != app.ExitOK {
			t.Errorf("code=%d", code)
		}
	})
	if !strings.Contains(out, "omp-tui") || !strings.Contains(out, app.Version) {
		t.Fatalf("version out=%q", out)
	}
	out, _ = captureStdoutStderr(t, func() {
		if code := run([]string{"-V"}); code != app.ExitOK {
			t.Errorf("code=%d", code)
		}
	})
	if !strings.Contains(out, app.Version) {
		t.Fatalf("-V out=%q", out)
	}
}

func TestRunParseError(t *testing.T) {
	_, errOut := captureStdoutStderr(t, func() {
		if code := run([]string{"--core-command-json"}); code != app.ExitError {
			t.Errorf("code=%d", code)
		}
	})
	if !strings.Contains(errOut, "omp-tui:") {
		t.Fatalf("stderr=%q", errOut)
	}
	// Also assert parse surface without process I/O.
	f := app.ParseArgs([]string{"--core-command-json"})
	if f.ParseErr == nil {
		t.Fatal("expected parse err")
	}
}

func TestRunMissingCoreConfig(t *testing.T) {
	t.Setenv(app.EnvCoreCommandJSON, "")
	t.Setenv(app.EnvBootstrapJSON, "")
	t.Setenv(app.EnvCoreCWD, "")
	_, errOut := captureStdoutStderr(t, func() {
		if code := run(nil); code != app.ExitError {
			t.Errorf("code=%d", code)
		}
	})
	if !strings.Contains(errOut, "core command") && !strings.Contains(errOut, "omp-tui:") {
		t.Fatalf("stderr=%q", errOut)
	}
}

func TestRunUnknownArg(t *testing.T) {
	t.Setenv(app.EnvCoreCommandJSON, `["x"]`)
	_, errOut := captureStdoutStderr(t, func() {
		if code := run([]string{"--nope"}); code != app.ExitError {
			t.Errorf("code=%d", code)
		}
	})
	if !strings.Contains(errOut, "unknown") && !strings.Contains(errOut, "omp-tui:") {
		t.Fatalf("stderr=%q", errOut)
	}
}
