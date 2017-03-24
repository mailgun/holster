package stack

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type FrameInfo struct {
	CallStack string
	Func      string
	File      string
	LineNo    int
}

func GetCallStack(frames errors.StackTrace) string {
	var trace []string
	for i := len(frames) - 1; i >= 0; i-- {
		trace = append(trace, fmt.Sprintf("%n()", frames[i]))
	}
	return strings.Join(trace, " ")
}

// Returns Caller information on the first frame in the stack trace
func GetLastFrame(frames errors.StackTrace) FrameInfo {
	funcName := func(funcPath string) string {
		idx := strings.LastIndex(funcPath, ".")
		if idx == -1 {
			return funcPath
		}
		return funcPath[idx+1:] + "()"
	}

	if len(frames) == 0 {
		return FrameInfo{}
	}
	pc := uintptr(frames[0]) - 1
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return FrameInfo{Func: fmt.Sprintf("unknown func at %f", pc)}
	}
	path, line := fn.FileLine(pc)
	return FrameInfo{
		CallStack: GetCallStack(frames),
		Func:      funcName(fn.Name()),
		File:      filepath.Base(path),
		LineNo:    line}
}

type HasStackTrace interface {
	StackTrace() errors.StackTrace
}

// stack represents a stack of program counters.
type Stack []uintptr

func (s *Stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case st.Flag('+'):
			for _, pc := range *s {
				f := errors.Frame(pc)
				fmt.Fprintf(st, "\n%+v", f)
			}
		}
	}
}

func (s *Stack) StackTrace() errors.StackTrace {
	f := make([]errors.Frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = errors.Frame((*s)[i])
	}
	return f
}

// Creates a new Stack{} struct from current stack minus 'skip' number of frames
func New(skip int) *Stack {
	skip += 2
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	var st Stack = pcs[0:n]
	return &st
}

// Returns the current goroutine id
// 	logrus.Infof("[%d] Info about this go routine", stack.GoRoutineID())
func GoRoutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
