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
	"time"

	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/log"
	"github.com/wagoodman/bashful/utils"
)

var DEBUG_BF = true

func newExecutorStats() *TaskStatistics {
	return &TaskStatistics{
		Failed:    make([]*Task, 0),
		Completed: make([]*Task, 0),
	}
}

func newExecutor(cfg *config.Config) *Executor {
	executor := &Executor{
		Environment:   make(map[string]string, 0),
		eventHandlers: make([]EventHandler, 0),
		config:        cfg,
		Tasks:         make([]*Task, 0),
		Statistics:    newExecutorStats(),
		cmdEtaCache:   make(map[string]time.Duration, 0),
		Registered:    map[string]string{},
	}

	for _, taskConfig := range cfg.TaskConfigs {
		// finalize task by appending to the set of final Tasks
		task := NewTask(taskConfig, &cfg.Options)
		executor.Tasks = append(executor.Tasks, task)
	}

	return executor
}

// estimateRuntime fetches and reads a cache file from disk containing CmdString-to-ETASeconds. Note: this this must be done before fetching/parsing the run.yaml
func (executor *Executor) readEtaCache() {
	// create the cache dirs if they do not already exist
	if _, err := os.Stat(executor.config.CachePath); os.IsNotExist(err) {
		os.Mkdir(executor.config.CachePath, 0755)
	}

	// read the time cache
	executor.cmdEtaCache = make(map[string]time.Duration)
	if utils.DoesFileExist(executor.config.EtaCachePath) {
		err := utils.Load(executor.config.EtaCachePath, &executor.cmdEtaCache)
		log.LogToMain(fmt.Sprintf("unable to load command eta cache: %v", err), log.StyleError)
	}

}

// estimateRuntime accumulates the ETA for all planned tasks
func (executor *Executor) estimateRuntime() {
	executor.readEtaCache()

	for _, task := range executor.Tasks {
		if task.Config.CmdString != "" || task.Config.URL != "" {
			executor.Statistics.Total++
			if eta, ok := executor.cmdEtaCache[task.Config.CmdString]; ok {
				task.Command.addEstimatedRuntime(eta)
			}
		}

		for _, subTask := range task.Children {
			if DEBUG_BF {
				//				pp.Fprintf(os.Stderr, "Subtask> %s %d\n", strings.Split(task.Id.String(), `-`)[0], syscall.Getpid())
			}
			if subTask.Config.CmdString != "" || subTask.Config.URL != "" {
				executor.Statistics.Total++
				if eta, ok := executor.cmdEtaCache[subTask.Config.CmdString]; ok {
					subTask.Command.addEstimatedRuntime(eta)
				}
			}
		}

		executor.config.TotalEtaSeconds += task.estimateRuntime()
	}
}

func (executor *Executor) addEventHandler(handler EventHandler) {
	handler.AddRuntimeData(executor.Statistics)
	executor.eventHandlers = append(executor.eventHandlers, handler)
}

// startNextSubTasks will kick start the maximum allowed number of commands (both primary and child task commands). Repeated invocation will iterate to new commands (and not repeat already markCompleted commands)
func (executor *Executor) startNextSubTasks(task *Task) {
	// Note that the parent task result channel and waiter are used for all Tasks and child Tasks
	if task.Config.CmdString != "" && !task.Started && executor.Statistics.Running < task.Options.MaxParallelCmds {
		go task.Execute(task.events, &task.waiter, executor.Environment)
		task.Started = true
		executor.Statistics.Running++
	}
	for idx := 0; executor.Statistics.Running < task.Options.MaxParallelCmds && idx < len(task.Children); idx++ {
		if task.Children[idx].Started {
			continue
		}
		subTask := task.Children[idx]
		go subTask.Execute(task.events, &task.waiter, nil)
		subTask.Started = true
		executor.Statistics.Running++
	}
}

// Execute will run the current Tasks primary command and/or all child commands. When execution has markCompleted, the screen frame will advance.
func (executor *Executor) execute(task *Task) error {

	for _, handler := range executor.eventHandlers {
		handler.Register(task)
	}

	executor.startNextSubTasks(task)

	for executor.Statistics.Running > 0 {
		event := <-task.events

		// manage completed tasks...
		if event.Complete {
			event.Task.Completed = true

			/*
				if task.Registered != nil {
					for _, k = range task.Registered {
						pp.Println(k)
						//        applied_vars = append(applied_vars, map[string]string{k, taskConfig.Registered})
					}
				}
			*/
			//pp.Fprintf(os.Stderr, "%s\n", task.Config.Registered)
			//pp.Fprintf(os.Stderr, "%s\n", executor.Registered)
			//			os.Exit(1)
			executor.Statistics.Completed = append(executor.Statistics.Completed, event.Task)
			executor.cmdEtaCache[task.Config.CmdString] = event.Task.Command.StopTime.Sub(event.Task.Command.StartTime)
			executor.Statistics.Running--

			executor.startNextSubTasks(task)

			task.Status = event.Status

			if event.Status == StatusError {
				// keep note of the failed task for an after task report
				task.FailedChildren++
				executor.Statistics.Failed = append(executor.Statistics.Failed, event.Task)
			}
		}

		// notify all handlers...
		for _, handler := range executor.eventHandlers {
			handler.OnEvent(task, event)
		}
	}

	close(task.events)

	if !exitSignaled {
		task.waiter.Wait()
	}

	// we should be done with all tasks/subtasks at this point, unregister everything
	for _, subTask := range task.Children {
		for _, handler := range executor.eventHandlers {
			handler.Unregister(subTask)
		}
	}
	for _, handler := range executor.eventHandlers {
		handler.Unregister(task)
	}
	return nil
}

var DEBUG_EXEC_BF = false

var global_task_vars = map[string]interface{}{}

func (executor *Executor) run() error {
	for _, task := range executor.Tasks {
		executor.execute(task)
		if exitSignaled {
			log.LogToMain("signaled to exit", log.StyleMajor)
			break
		}
	}
	for _, handler := range executor.eventHandlers {
		handler.Close()
	}

	err := utils.Save(executor.config.EtaCachePath, &executor.cmdEtaCache)
	if err != nil {
		log.LogToMain(fmt.Sprintf("unable to save command eta cache: %v", err), log.StyleError)
	}

	return nil
}
