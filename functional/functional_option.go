package functional

import "io"

type FunctionalOption interface {
	Apply(t *T)
}

type withWriterOption struct {
	writer io.Writer
}

func (o *withWriterOption) Apply(t *T) {
	t.writer = o.writer
	t.errWriter = o.writer
}

// WithWriter sets log output writer.
func WithWriter(writer io.Writer) FunctionalOption {
	return &withWriterOption{writer: writer}
}

type withArgs struct {
	args []string
}

func (o *withArgs) Apply(t *T) {
	t.args = o.args
}

func WithArgs(args ...string) FunctionalOption {
	return &withArgs{args: args}
}
