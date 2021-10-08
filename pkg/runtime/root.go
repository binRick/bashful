// Copyright © 2018 Alex Goodman
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package runtime

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/wagoodman/bashful/utils"
)

func AppendIfMissing(slice []int64, i int64) []int64 {
	for _, ele := range slice {
		if ele == i {
			return slice
		}
	}
	return append(slice, i)
}

func GetProcessChildren(p int64) []int64 {
	var child_pids []int64
	tp, err := process.NewProcess(int32(p))
	if err == nil {
		children, err := tp.Children()
		if err == nil {
			for _, child_proc := range children {
				child_pids = AppendIfMissing(child_pids, int64(child_proc.Pid))
				for _, grandchild := range GetProcessChildren(int64(child_proc.Pid)) {
					child_pids = AppendIfMissing(child_pids, int64(grandchild))
				}
			}
		}
	}
	var _child_pids []int64
	for _, pid := range child_pids {
		e, err := process.PidExists(int32(pid))
		if err == nil && e {
			_child_pids = AppendIfMissing(_child_pids, int64(pid))
		}
	}

	return _child_pids
}

const CLEANUP_PROCS = false

func cleanup_procs() {
	if !CLEANUP_PROCS {
		return
	}
	killed_pids := []int64{}
	pids := GetProcessChildren(int64(syscall.Getpid()))
	if len(pids) == 0 {
		return
	}
	for {
		for _, pid := range pids {
			err := syscall.Kill(int(pid), syscall.SIGINT)
			if err == nil {
				killed_pids = append(killed_pids, pid)
			}
		}
		for _, pid := range GetProcessChildren(int64(syscall.Getpid())) {
			err := syscall.Kill(int(pid), syscall.SIGTERM)
			if err == nil {
				killed_pids = append(killed_pids, pid)
			}
		}

		msg := fmt.Sprintf("\n\n[%d] Killed %d/%d procs: %d\n\n", syscall.Getpid(), len(killed_pids), len(pids), killed_pids)
		fmt.Fprintf(os.Stderr, "%s", msg)

		if len(GetProcessChildren(int64(syscall.Getpid()))) == 0 {
			cleanup_procs()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func Setup() {
	sigChannel := make(chan os.Signal, 2)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChannel {
			if sig == syscall.SIGINT {
				cleanup_procs()
				utils.ExitWithErrorMessage(utils.Red("Keyboard Interrupt"))
			} else if sig == syscall.SIGTERM {
				cleanup_procs()
				utils.Exit(0)
			} else {
				utils.ExitWithErrorMessage("Unknown Signal: " + sig.String())
			}
		}
	}()
}
