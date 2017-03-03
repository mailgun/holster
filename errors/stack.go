package errors

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	pkg "github.com/pkg/errors"
)

type Caller struct {
	Func   string
	File   string
	LineNo int
}

func GetCaller(depth int) Caller {
	funcName := func(pc uintptr) string {
		funcPath := runtime.FuncForPC(pc).Name()
		idx := strings.LastIndex(funcPath, ".")
		if idx == -1 {
			return funcPath
		}
		return funcPath[idx+1:] + "()"
	}
	if pc, path, line, ok := runtime.Caller(depth + 1); ok {
		return Caller{Func: funcName(pc), File: filepath.Base(path), LineNo: line}
	}
	return Caller{}
}

// stack represents a stack of program counters.
type stack []uintptr

func (s *stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case st.Flag('+'):
			for _, pc := range *s {
				f := pkg.Frame(pc)
				fmt.Fprintf(st, "\n%+v", f)
			}
		}
	}
}

func (s *stack) StackTrace() pkg.StackTrace {
	f := make([]pkg.Frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = pkg.Frame((*s)[i])
	}
	return f
}

func callers() *stack {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	var st stack = pcs[0:n]
	return &st
}
