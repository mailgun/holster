/*
Copyright 2022 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package steve

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"

	"github.com/mailgun/holster/v4/errors"
	"github.com/sirupsen/logrus"
)

// Event handler for `ExecJob`.
type ExecHandler interface {
	// Receives exit code after command finishes.
	Done(exitCode int)
}

// `ExecAction` implements `Action` for use with `EZJob`.
type ExecAction struct {
	cmd     string
	args    []string
	handler ExecHandler
}

func NewExecAction(handler ExecHandler, cmd string, args ...string) *ExecAction {
	return &ExecAction{
		handler: handler,
		cmd:     cmd,
		args:    args,
	}
}

// Create an `EZJob` to exec a shell command.
func NewExecJob(handler ExecHandler, cmd string, args ...string) *EZJob {
	action := NewExecAction(handler, cmd, args...)
	execJob := NewEZJob(action)
	return execJob
}

func (a *ExecAction) Run(ctx context.Context, writer io.Writer) error {
	log.WithContext(ctx).WithFields(logrus.Fields{
		"cmd": append([]string{a.cmd}, a.args...),
	}).Info("Starting ExecAction task")

	ecmd := exec.CommandContext(ctx, a.cmd, a.args...)
	outPipe, err := ecmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "error in cmd.StdoutPipe")
	}

	errPipe, err := ecmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "error in cmd.StderrPipe")
	}

	if err = ecmd.Start(); err != nil {
		return errors.Wrap(err, "error in cmd.Start")
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		_, err := io.Copy(writer, outPipe)
		if err != nil {
			log.WithContext(ctx).WithError(err).Error("error reading stdout")
		}
		wg.Done()
	}()

	go func() {
		_, err := io.Copy(writer, errPipe)
		if err != nil {
			log.WithContext(ctx).WithError(err).Error("error reading stderr")
		}
		wg.Done()
	}()

	// When both pipes are closed, the subprocess is done.
	wg.Wait()

	var exitCode int
	err = ecmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if waitStatus, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = waitStatus.ExitStatus() // -1 if signaled
			}
		}
	}

	// Pass exit code to caller in `Done` event.
	defer a.handler.Done(exitCode)

	finishLog := log.WithContext(ctx).WithFields(logrus.Fields{
		"exitCode": exitCode,
	})

	if exitCode != 0 {
		finishLog.WithError(err).Error("ExecJob task failed")
		return fmt.Errorf("Exit code %d", exitCode)
	}

	finishLog.Info("ExecJob task finished")
	return nil
}

func (a *ExecAction) Status(status Status) {
	// Nothing to do.
}
