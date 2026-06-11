package cli

import (
	"fmt"
	"io"
)

type output struct {
	writer io.Writer
	err    error
}

func newOutput(writer io.Writer) *output {
	return &output{writer: writer}
}

func (o *output) print(args ...any) {
	if o.err != nil {
		return
	}
	_, o.err = fmt.Fprint(o.writer, args...)
}

func (o *output) printf(format string, args ...any) {
	if o.err != nil {
		return
	}
	_, o.err = fmt.Fprintf(o.writer, format, args...)
}

func (o *output) println(args ...any) {
	if o.err != nil {
		return
	}
	_, o.err = fmt.Fprintln(o.writer, args...)
}

func (o *output) result() error {
	return o.err
}
