package main

import (
	"os"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realWriter actually writes files.
type realWriter struct{}

var _ entrypoint.Writer = (*realWriter)(nil)

func (*realWriter) Write(file string) error {
	if file == "" {
		return nil
	}
	_, err := os.Create(file)
	return err
}
