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

	"github.com/google/uuid"
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

var DEBUG_MODE = (os.Getenv(`DEBUG_MODE`) == `1`)

const (
	StatusRunning TaskStatus = iota
	StatusPending
	StatusSuccess
	StatusError
)

var DEBUG_CG = false
var DEBUG_CG_TASK = true
var DEBUG_CG_END = true

var swap_max int64 = 1024 * 1000 * 1000
var mem_max int64 = 512 * 1000 * 1000
var proc_max int64 = 200

// NewTask creates a new task in the context of the user configuration at a particular screen location (row)
func NewTask(taskConfig config.TaskConfig, runtimeOptions *config.Options) *Task {
	task := Task{
		Id:      uuid.New(),
		Config:  taskConfig,
		Options: runtimeOptions,
	}

	if false {
		vars_lists := []map[string]string{
			runtimeOptions.Vars,
			runtimeOptions.Env,
		}
		if task.Config.Vars == nil {
			task.Config.Vars = map[string]string{}
		}
		for _, vars_list := range vars_lists {
			//pp.Println(vars_list)
			for vk, vv := range vars_list {
				if len(vk) > 0 {
					_, hasv := task.Config.Vars[vk]
					if !hasv {
						task.Config.Vars[vk] = vv
					}
				}
			}
		}
	}
	/*
		if DEBUG_MODE {
			if task.Config.CmdGenerator != `` {
				pp.Println(task.Config.CmdGenerator)
				pp.Println(task.Config.CmdGeneratorLog)
				pp.Println(task.Config.CmdString)
				pp.Println(task.Config.ForEachList)
				//			os.Exit(1)
			}
		}
	*/

	task.Command = newCommand(task.Config)
	task.events = make(chan TaskEvent)
	task.Status = StatusPending

	for subIndex := range taskConfig.ParallelTasks {
		subTaskConfig := &taskConfig.ParallelTasks[subIndex]

		subTask := NewTask(*subTaskConfig, runtimeOptions)
		task.Children = append(task.Children, subTask)
	}

	//	BASE_CG_PATH := os.Getenv(`CGROUPS_BASE_CG_PATH`)
	//PARENT_CGROUP_PATH := os.Getenv(`PARENT_CGROUP_PATH`)
	//	BASHFUL_CGROUP_PATH := os.Getenv(`BASHFUL_CGROUP_PATH`)

	return &task
}

// UpdateExec reinstantiates the planned command to run based on the given path to an executable
func (task *Task) UpdateExec(execpath string) {
	if task.Config.CmdString == "" {
		task.Config.CmdString = task.Options.ExecReplaceString
	}
	task.Config.CmdString = strings.Replace(task.Config.CmdString, task.Options.ExecReplaceString, execpath, -1)
	task.Config.CmdGenerator = strings.Replace(task.Config.CmdGenerator, task.Options.ExecReplaceString, execpath, -1)
	task.Config.URL = strings.Replace(task.Config.URL, task.Options.ExecReplaceString, execpath, -1)

	task.Command = newCommand(task.Config)

	// note: this will affect the eta, however, this should be handled by the executor (todo: any cleanup here?)
	// if eta, ok := task.Executor.cmdEtaCache[task.Config.CmdString]; ok {
	// 	task.Command.addEstimatedRuntime(eta)
	// }
}

// Kill will stop any running command (including child Tasks) with a -9 signal
func (task *Task) Kill() {
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
					f, err := os.OpenFile(task.Config.StderrLogFile, os.O_APPEND|os.O_WRONLY, 0600)
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
					f, err := os.OpenFile(task.Config.StdoutLogFile, os.O_APPEND|os.O_WRONLY, 0600)
					if err == nil {
						_, _ = f.WriteString(string(message) + "\n")
						f.Close()
					}
				}
			}

			resultChan <- vtclean.Clean(message, false)
		}
	}
	//	cmd_started := time.Now()

	var command_log_file_callback = func(cmd string, exit_code int, ended time.Time) {
		if len(task.Config.CommandLogFile) > 0 {
			duration := time.Since(ended)
			msg := fmt.Sprintf(`%s exited %d after %s`, cmd, exit_code, duration)
			f, err := os.OpenFile(task.Config.CommandLogFile, os.O_APPEND|os.O_WRONLY, 0600)
			if err == nil {
				_, _ = f.WriteString(msg + "\n")
				f.Close()
			}
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
	// Compile the template first (i. e. creating the AST)

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

	task.Command.Cmd.Start()
	msgs := map[string][]string{
		`stdout`: []string{},
		`stderr`: []string{},
	}
	for {
		select {
		case stdoutMsg, ok := <-stdoutChan:
			if len(stdoutMsg) > 0 {
				msgs[`stdout`] = append(msgs[`stdout`], stdoutMsg)
			}
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
	command_log_file_callback(``, task.Command.ReturnCode, task.Command.StopTime)

	// close the write end of the pipe since the child shell is positively no longer writing to it
	task.Command.Cmd.ExtraFiles[0].Close()
	//task.Command.Cmd.ExtraFiles[1].Close()
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

	if len(task.Config.Register) > 0 {
		if task.Config.Registered == nil {
			task.Config.Registered = map[string][]string{}
		}
		//pp.Println(`registering results to `, task.Config.Register, msgs, task.Config.Registered)
		for _, msg := range msgs[`stdout`] {
			//pp.Println(`     - registering result %s to %s`, msg, task.Config.Register)
			_, has := task.Config.Registered[task.Config.Register]
			if !has {
				task.Config.Registered[task.Config.Register] = []string{}
			}
			task.Config.Registered[task.Config.Register] = append(task.Config.Registered[task.Config.Register], msg)
		}
	}
	if false {
		pp.Fprintf(os.Stderr, "%s\n", task.Config.Registered)
		pp.Fprintf(os.Stderr, "%s\n", task)
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
