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
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	"github.com/google/uuid"
	"github.com/jpillora/overseer"
	"github.com/k0kubun/pp"
	"github.com/lunixbochs/vtclean"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/utils"
	terminaldimensions "github.com/wayneashleyberry/terminal-dimensions"
)

// todo: remove these global vars
var (
	sudoPassword string
	exitSignaled bool
)

const (
	StatusRunning TaskStatus = iota
	StatusPending
	StatusSuccess
	StatusError
)

func startNewProcess(path string, args []string, env map[string]string, task *Task) error {

	//	env = append(env, fmt.Sprintf(`BASHFUL_CGROUP_STARTED=%s`, fmt.Sprintf(`%d`, int64(time.Now().UnixNano()))))
	//env = append(env, fmt.Sprintf(`BASHFUL_CGROUP_UUID=%s`, fmt.Sprintf(`%s`, uuid.New().String())))
	BASHFUL_CGROUP_PATH := fmt.Sprintf(`%s/%s`, strings.Split(task.Config.BCG.ParentUUID.String(), `-`)[0], strings.Split(uuid.New().String(), `-`)[0])
	//	env = append(env, fmt.Sprintf(`BASHFUL_CGROUP_PATH=%s`, BASHFUL_CGROUP_PATH))
	//pp.Println(`env:`, task.Command.Cmd.Env)

	parent_cg, err := cgroups.New(cgroups.V1, cgroups.StaticPath(BASHFUL_CGROUP_PATH), cg_limit1)
	if err != nil {
		return err
	}
	if false {
		fmt.Println(parent_cg)
	}
	execSpec := &syscall.ProcAttr{
		Env: task.Command.Cmd.Env,
		//Files: []uintptr{task.Command.Cmd.Stdout.Fd()},
		//		Files: []uintptr{task.Command.Cmd.StdinPipe.Fd(), task.Command.Cmd.StdoutPipe.Fd(), task.Command.Cmd.StderrPipe.Fd()},
	}
	if false {
		fork, err := syscall.ForkExec(task.Command.Cmd.Path, task.Command.Cmd.Args, execSpec)
		if err != nil {
			return fmt.Errorf("failed to forkexec: %v", err)
		}

		fmt.Fprintf(os.Stderr, "start new process success, pid %d.", fork)
	} else {
		//pp.Println(task.BCG.ParentUUID.String())
		task.Command.Cmd.Start()
	}

	return nil
}

// NewTask creates a new task in the context of the user configuration at a particular screen location (row)
func NewTask(taskConfig config.TaskConfig, runtimeOptions *config.Options) *Task {
	task := Task{
		Id:      uuid.New(),
		Config:  taskConfig,
		Options: runtimeOptions,
	}

	task.Command = newCommand(task.Config)

	task.events = make(chan TaskEvent)
	task.Status = StatusPending

	for subIndex := range taskConfig.ParallelTasks {
		subTaskConfig := &taskConfig.ParallelTasks[subIndex]

		subTask := NewTask(*subTaskConfig, runtimeOptions)
		task.Children = append(task.Children, subTask)
	}
	if DEBUG_BF {
		pp.Fprintf(os.Stderr, "NEW TASK>  % %d children\n", syscall.Getpid(), len(task.Children))
	}

	return &task
}

// UpdateExec reinstantiates the planned command to run based on the given path to an executable
func (task *Task) UpdateExec(execpath string) {
	if DEBUG_BF {
		pp.Fprintf(os.Stderr, "UpdateExec>> %s\n", execpath)
	}

	if task.Config.CmdString == "" {
		task.Config.CmdString = task.Options.ExecReplaceString
	}
	task.Config.CmdString = strings.Replace(task.Config.CmdString, task.Options.ExecReplaceString, execpath, -1)
	task.Config.URL = strings.Replace(task.Config.URL, task.Options.ExecReplaceString, execpath, -1)

	task.Command = newCommand(task.Config)

	// note: this will affect the eta, however, this should be handled by the executor (todo: any cleanup here?)
	// if eta, ok := task.Executor.cmdEtaCache[task.Config.CmdString]; ok {
	// 	task.Command.addEstimatedRuntime(eta)
	// }
}

// Kill will stop any running command (including child Tasks) with a -9 signal
func (task *Task) Kill() {
	exec_uuid := uuid.New()
	if DEBUG_BF {
		pp.Fprintf(os.Stderr, "KILL> %s %d\n", exec_uuid.String(), syscall.Getpid())
	}
	if task.Config.CmdString != "" && task.Started && !task.Completed {
		syscall.Kill(-task.Command.Cmd.Process.Pid, syscall.SIGKILL)
	}

	for _, subTask := range task.Children {
		if subTask.Config.CmdString != "" && subTask.Started && !subTask.Completed {
			syscall.Kill(-subTask.Command.Cmd.Process.Pid, syscall.SIGKILL)
		}
	}
}

func (task *Task) requiresSudoPassword() bool {
	if task.Config.Sudo && task.Config.CmdString != "" {
		return true
	}
	for _, subTask := range task.Children {
		if subTask.Config.Sudo && subTask.Config.CmdString != "" {
			return true
		}
	}

	return false
}
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// estimateRuntime returns the ETA in seconds until command completion
func (task *Task) estimateRuntime() float64 {
	var etaSeconds float64
	// finalize task by appending to the set of final Tasks
	if task.Config.CmdString != "" && task.Command.EstimatedRuntime != -1 {
		etaSeconds += task.Command.EstimatedRuntime.Seconds()
	}

	var maxParallelEstimatedRuntime float64
	var taskEndSecond []float64
	var currentSecond float64
	var remainingParallelTasks = task.Options.MaxParallelCmds

	for subIndex := range task.Children {
		subTask := task.Children[subIndex]
		if subTask.Config.CmdString != "" && subTask.Command.EstimatedRuntime != -1 {
			// this is a sub task with an eta
			if remainingParallelTasks == 0 {

				// we've started all possible Tasks, now they should stop...
				// select the first task to stop
				remainingParallelTasks++
				minEndSecond, _, err := utils.MinMax(taskEndSecond)
				utils.CheckError(err, "No min eta for empty array!")
				taskEndSecond = utils.RemoveOneValue(taskEndSecond, minEndSecond)
				currentSecond = minEndSecond
			}

			// we are still starting Tasks
			taskEndSecond = append(taskEndSecond, currentSecond+subTask.Command.EstimatedRuntime.Seconds())
			remainingParallelTasks--

			_, maxEndSecond, err := utils.MinMax(taskEndSecond)
			utils.CheckError(err, "No max eta for empty array!")
			maxParallelEstimatedRuntime = math.Max(maxParallelEstimatedRuntime, maxEndSecond)
		}

	}
	etaSeconds += maxParallelEstimatedRuntime
	return etaSeconds
}

// run executes a Tasks primary command (not child task commands) and monitors command events
func (task *Task) Execute(eventChan chan TaskEvent, waiter *sync.WaitGroup, environment map[string]string) {

	task.Command.StartTime = time.Now()
	//exec_uuid := uuid.New()

	eventChan <- TaskEvent{Task: task, Status: StatusRunning, ReturnCode: -1}
	waiter.Add(1)
	if DEBUG_BF {
		pp.Fprintf(os.Stderr, "Execute Task> %d | %s %s\n\n", syscall.Getpid(), task.Command.Cmd.Path, strings.Join(task.Command.Cmd.Args, ` `))
		pp.Println(task)
	}
	defer waiter.Done()

	stdoutChan := make(chan string, 1000)
	stderrChan := make(chan string, 1000)
	stdoutPipe, _ := task.Command.Cmd.StdoutPipe()
	stderrPipe, _ := task.Command.Cmd.StderrPipe()

	readPipe := func(resultChan chan string, pipe io.ReadCloser, _type string) {
		defer close(resultChan)
		scanner := bufio.NewScanner(pipe)
		scanner.Split(variableSplitFunc)
		for scanner.Scan() {
			message := scanner.Text()
			if _type == `stderr` {
				if len(task.Config.StderrLogFile) > 0 {
					if !FileExists(task.Config.StderrLogFile) {
						_f, err := os.OpenFile(task.Config.StderrLogFile, os.O_CREATE, 0600)
						if err == nil {
							_f.Close()
						}
					}
					f, err := os.OpenFile(task.Config.StderrLogFile, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						_, _ = f.WriteString(string(message) + "\n")
						f.Close()
					}
				}
			}
			if _type == `stdout` {
				if len(task.Config.StdoutLogFile) > 0 {
					if !FileExists(task.Config.StdoutLogFile) {
						_f, err := os.OpenFile(task.Config.StdoutLogFile, os.O_CREATE, 0600)
						if err == nil {
							_f.Close()
						}
					}
					f, err := os.OpenFile(task.Config.StdoutLogFile, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						_, _ = f.WriteString(string(message) + "\n")
						f.Close()
					}
				}
			}

			resultChan <- vtclean.Clean(message, false)
		}
	}

	go readPipe(stdoutChan, stdoutPipe, `stdout`)
	go readPipe(stderrChan, stderrPipe, `stderr`)

	for _, env := range os.Environ() {
		envPair := strings.SplitN(env, "=", 2)
		k := envPair[0]
		v := envPair[1]
		task.Command.Cmd.Env = append(task.Command.Cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range environment {
		task.Command.Cmd.Env = append(task.Command.Cmd.Env, fmt.Sprintf("%s=%s", k, v))
		if false {
		}
	}
	if task.Options.Env != nil {
		for k, v := range task.Options.Env {
			task.Command.Cmd.Env = append(task.Command.Cmd.Env, fmt.Sprintf("%s=%s", k, v))
			if false {
			}
		}

	}
	if task.Config.Env != nil {
		for k, v := range task.Config.Env {
			task.Command.Cmd.Env = append(task.Command.Cmd.Env, fmt.Sprintf("%s=%s", k, v))
			if false {
			}
		}
	}
	/*
		fds := []uintptr{
			readPipe.Fd(),
			stdoutPipe.Fd(),
			stderrPipe.Fd(),
		}
	*/
	err := startNewProcess(task.Command.Cmd.Path, task.Command.Cmd.Args, task.Config.Env, task)

	if false {
		var prog = func(state overseer.State) {

		}

		overseer.Run(overseer.Config{
			Program: prog,
			Address: ":5001",
			Debug:   true, //display log of overseer actions
		})
	}

	if DEBUG_BF {
		fmt.Fprintf(os.Stderr, "Started PID> %s\n", pp.Sprintf(`%s`, task.Config))
	}
	//  if cli.BashfulCgroup.ParentCgroup.Add(cgroups.Process{Pid: task.Command.Cmd.Process.Pid}) != nil {
	//	fmt.Fprintf(os.Stderr, "Started PID> %d\n", task.Command.Cmd.Process.Pid)
	//}

	for {
		select {
		case stdoutMsg, ok := <-stdoutChan:
			if ok {
				// it seems that we are getting a bit behind... burn off elements without showing them on the screen
				if len(stdoutChan) > 100 {
					continue
				}

				// todo: we should always throw the TaskEvent? let the TaskEvent handler deal with TaskEvent/polling...
				if task.Config.EventDriven {
					// this is TaskEvent driven... (signal this TaskEvent)
					eventChan <- TaskEvent{Task: task, Status: StatusRunning, Stdout: utils.Blue(stdoutMsg), ReturnCode: -1}
				}
				// else {
				// 	// on a polling interval... (do not create an TaskEvent)
				// 	task.Display.Values.Msg = utils.Blue(stdoutMsg)
				// }

			} else {
				stdoutChan = nil
			}
		case stderrMsg, ok := <-stderrChan:
			if ok {

				// todo: we should always throw the TaskEvent? let the TaskEvent handler deal with TaskEvent/polling...
				if task.Config.EventDriven {
					// either this is TaskEvent driven... (signal this TaskEvent)
					eventChan <- TaskEvent{Task: task, Status: StatusRunning, Stderr: utils.Red(stderrMsg), ReturnCode: -1}
				}
				// else {
				// 	// or on a polling interval... (do not create an TaskEvent)
				// 	task.Display.Values.Msg = utils.Red(stderrMsg)
				// }
				task.Command.errorBuffer.WriteString(stderrMsg + "\n")
			} else {
				stderrChan = nil
			}
		}
		if stdoutChan == nil && stderrChan == nil {
			break
		}
	}

	returnCode := 0
	returnCodeMsg := "unknown"
	if err := task.Command.Cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an Exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				returnCode = status.ExitStatus()
			}
		} else {
			returnCode = -1
			returnCodeMsg = "Failed to run: " + err.Error()
			eventChan <- TaskEvent{Task: task, Status: StatusError, Stderr: returnCodeMsg, ReturnCode: returnCode}
			task.Command.errorBuffer.WriteString(returnCodeMsg + "\n")
		}
	}
	task.Command.ReturnCode = returnCode
	task.Command.StopTime = time.Now()

	// close the write end of the pipe since the child shell is positively no longer writing to it
	task.Command.Cmd.ExtraFiles[0].Close()
	data, err := ioutil.ReadAll(task.Command.EnvReadFile)
	utils.CheckError(err, "Could not read env vars from child shell")

	if environment != nil {
		lines := strings.Split(string(data[:]), "\n")
		for _, line := range lines {
			fields := strings.SplitN(strings.TrimSpace(line), "=", 2)
			if len(fields) == 2 {
				environment[fields[0]] = fields[1]
			} else if len(fields) == 1 {
				environment[fields[0]] = ""
			}
		}
	}

	if returnCode == 0 || task.Config.IgnoreFailure {
		eventChan <- TaskEvent{Task: task, Status: StatusSuccess, Complete: true, ReturnCode: returnCode}
	} else {
		eventChan <- TaskEvent{Task: task, Status: StatusError, Complete: true, ReturnCode: returnCode}
		if task.Config.StopOnFailure {
			exitSignaled = true
		}
	}
}

// variableSplitFunc splits a bytestream based on either newline characters or by length (if the string is too long)
func variableSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {

	// Return nothing if at end of file and no data passed
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Case: \n
	if i := strings.Index(string(data), "\n"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// Case: \r
	if i := strings.Index(string(data), "\r"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// Case: it's just too long
	terminalWidth, _ := terminaldimensions.Width()
	if len(data) > int(terminalWidth*2) {
		return int(terminalWidth * 2), data[0:int(terminalWidth*2)], nil
	}

	// TODO: by some ansi escape sequences

	// If at end of file with data return the data
	if atEOF {
		return len(data), data, nil
	}

	return
}
