package command

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

type outputKey struct{}

// WithOutput tees selected command output to a caller-owned log. Commands
// without this option retain their normal capture-only behavior.
func WithOutput(ctx context.Context, output io.Writer) context.Context {
	return context.WithValue(ctx, outputKey{}, output)
}

func CaptureOutput(ctx context.Context, cmd *exec.Cmd) (string, error) {
	var buffer bytes.Buffer
	var output io.Writer = &buffer
	if observer, ok := ctx.Value(outputKey{}).(io.Writer); ok {
		output = io.MultiWriter(&buffer, observer)
	}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	return buffer.String(), err
}
