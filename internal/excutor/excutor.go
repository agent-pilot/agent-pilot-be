package excutor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
)

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

type ExecResult struct {
	Args   []string
	Stdout []byte
	Stderr []byte
	Err    error
}

// Run executes a CLI binary with args and captures stdout/stderr.
func (e *Executor) Run(ctx context.Context, argstr string) ExecResult {
	args := SplitArgs(argstr)
	cmd := exec.CommandContext(ctx, "lark-cli", args...)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return ExecResult{
		Args:   args,
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
		Err:    err,
	}
}

func SplitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	tokenStarted := false
	flush := func() {
		if tokenStarted {
			out = append(out, cur.String())
		}
		cur.Reset()
		tokenStarted = false
	}

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				tokenStarted = true
			} else {
				cur.WriteRune(r)
				tokenStarted = true
			}
		case r == '\'' || r == '"':
			quote = r
			tokenStarted = true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
			tokenStarted = true
		}
	}
	flush()
	return out
}
