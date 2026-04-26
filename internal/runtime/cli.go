package runtime

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

type ExecResult struct {
	Args   []string
	Stdout []byte
	Stderr []byte
	Err    error
}

func RunCLI(ctx context.Context, bin string, args []string) ExecResult {
	cmd := exec.CommandContext(ctx, bin, args...)
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
