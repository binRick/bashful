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

package config

import (
	"fmt"
	"os"
	"strings"
)

var VERBOSE_MODE = (os.Getenv(`__BASHFUL_VERBOSE_MODE`) == `true`)

// NewTaskConfig creates a new TaskConfig populated with sane default values (derived from the global Options)
func NewTaskConfig() (obj TaskConfig) {
	obj.IgnoreFailure = globalOptions.IgnoreFailure
	obj.StopOnFailure = globalOptions.StopOnFailure
	obj.ShowTaskOutput = globalOptions.ShowTaskOutput
	obj.EventDriven = globalOptions.EventDriven
	obj.CollapseOnCompletion = globalOptions.CollapseOnCompletion
	if VERBOSE_MODE {
		obj.CollapseOnCompletion = false

	}
	return obj
}

// UnmarshalYAML parses and creates a TaskConfig from a given user yaml string
func (taskConfig *TaskConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults TaskConfig
	defaultValues := defaults(NewTaskConfig())

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*taskConfig = TaskConfig(defaultValues)

	return nil
}

// allow passing a single value or multiple values into a yaml string (e.g. `tags: thing` or `{tags: [thing1, thing2]}`)
func (a *stringArray) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*a = []string{single}
	} else {
		*a = multi
	}
	return nil
}

func (taskConfig *TaskConfig) compile(config *Config) (tasks []TaskConfig) {
	taskConfig.OrigCmdString = taskConfig.CmdString
	taskConfig.OrigCmdGenerator = taskConfig.CmdGenerator

	taskConfig.CmdString = config.replaceArguments(taskConfig.CmdString)
	taskConfig.CmdGenerator = taskConfig.CmdGenerator
	if taskConfig.Name == "" {
		taskConfig.Name = taskConfig.CmdString
	} else {
		taskConfig.Name = config.replaceArguments(taskConfig.Name)
	}

	for _, l := range taskConfig.ForEachList {
		for _, _l := range l {
			taskConfig.ForEach = append(taskConfig.ForEach, fmt.Sprintf(`%s`, _l))
		}
	}
	if len(taskConfig.ForEach) > 0 {
		for _, replicaValue := range taskConfig.ForEach {
			// make replacements of select attributes on a copy of the Config
			newConfig := *taskConfig

			// ensure we don't re-compile a replica with more replica's of itself
			newConfig.ForEach = make([]string, 0)

			if newConfig.Name == "" {
				newConfig.Name = newConfig.CmdString
			}
			newConfig.ReplicaReplaceString = config.Options.ReplicaReplaceString
			newConfig.Name = strings.Replace(newConfig.Name, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.CmdGenerator = strings.Replace(newConfig.CmdGenerator, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.TimehistoryJsonLogFile = strings.Replace(newConfig.TimehistoryJsonLogFile, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.CmdGeneratorLog = strings.Replace(newConfig.CmdGeneratorLog, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.CmdString = strings.Replace(newConfig.CmdString, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.URL = strings.Replace(newConfig.URL, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.CommandLogFile = strings.Replace(newConfig.CommandLogFile, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.StdoutLogFile = strings.Replace(newConfig.StdoutLogFile, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.StderrLogFile = strings.Replace(newConfig.StderrLogFile, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.PreCmdString = strings.Replace(newConfig.PreCmdString, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.RescueCmdString = strings.Replace(newConfig.RescueCmdString, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.DebugCmdLog = strings.Replace(newConfig.DebugCmdLog, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.DebugCmdString = strings.Replace(newConfig.DebugCmdString, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.PostCmdString = strings.Replace(newConfig.PostCmdString, config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.CurrentItem = replicaValue

			newConfig.Tags = make(stringArray, len(taskConfig.Tags))
			for k := range taskConfig.Tags {
				newConfig.Tags[k] = strings.Replace(taskConfig.Tags[k], config.Options.ReplicaReplaceString, replicaValue, -1)
			}

			// insert the copy after current index
			tasks = append(tasks, newConfig)
		}
	}
	return tasks
}

func (taskConfig *TaskConfig) validate() error {
	if taskConfig.CmdString == "" && len(taskConfig.ParallelTasks) == 0 && taskConfig.URL == "" {
		return fmt.Errorf("task '" + taskConfig.Name + "' misconfigured (A configured task must have at least 'cmd', 'url', or 'parallel-tasks' configured)")
	}
	return nil
}
