package callstack

import (
	"bytes"
	"fmt"
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
		trace = append(trace, fmt.Sprintf("%v", frames[i]))
	}
	return strings.Join(trace, " ")
}

// GetLastFrame returns Caller information on the first frame in the stack trace.
func GetLastFrame(frames errors.StackTrace) FrameInfo {
	if len(frames) == 0 {
		return FrameInfo{}
	}
	pc := uintptr(frames[0]) - 1
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return FrameInfo{Func: fmt.Sprintf("unknown func at %v", pc)}
	}
	filePath, lineNo := fn.FileLine(pc)
	return FrameInfo{
		CallStack: GetCallStack(frames),
		Func:      FuncName(fn),
		File:      filePath,
		LineNo:    lineNo,
	}
}

// FuncName given a runtime function spec returns a short function name in
// format `<package name>.<function name>` or if the function has a receiver
// in format `<package name>.(<receiver>).<function name>`.
func FuncName(fn *runtime.Func) string {
	if fn == nil {
		return ""
	}
	funcPath := fn.Name()
	idx := strings.LastIndex(funcPath, "/")
	if idx == -1 {
		return funcPath
	}
	return funcPath[idx+1:]
}

type HasStackTrace interface {
	StackTrace() errors.StackTrace
}

// CallStack represents a stack of program counters.
type CallStack []uintptr

func (cs *CallStack) Format(st fmt.State, verb rune) {
	if verb == 'v' && st.Flag('+') {
		for _, pc := range *cs {
			f := errors.Frame(pc)
			_, _ = fmt.Fprintf(st, "\n%+v", f)
		}
	}
}

func (cs *CallStack) StackTrace() errors.StackTrace {
	f := make([]errors.Frame, len(*cs))
	for i := 0; i < len(f); i++ {
		f[i] = errors.Frame((*cs)[i])
	}
	return f
}

// New creates a new CallStack struct from current stack minus 'skip' number of frames.
func New(skip int) *CallStack {
	skip += 2
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	var st CallStack = pcs[0:n]
	return &st
}

// GoRoutineID returns the current goroutine id.
func GoRoutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
