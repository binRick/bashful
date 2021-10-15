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

package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/containerd/cgroups"
	mapset "github.com/deckarep/golang-set"
	"github.com/google/uuid"
	"github.com/k0kubun/pp"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/shirou/gopsutil/process"

	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/log"
	"github.com/wagoodman/bashful/pkg/runtime"
	"github.com/wagoodman/bashful/pkg/runtime/handler"
	"github.com/wagoodman/bashful/utils"
)

// todo: put these in a cli struct instance instead, then most logic can be in the cli struct
var tags, onlyTags string
var listTagsMode bool
var devMode bool
var cgroupsMode bool
var listTasksMode bool

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the given yaml file with bashful",
	Long:  `Execute the given yaml file with bashful`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		if tags != "" && onlyTags != "" {
			utils.ExitWithErrorMessage("Options 'tags' and 'only-tags' are mutually exclusive.")
		}

		cli := config.Cli{
			YamlPath: args[0],
		}

		if len(args) > 1 {
			cli.Args = args[1:]
		} else {
			cli.Args = []string{}
		}

		for _, value := range strings.Split(tags, ",") {
			if value != "" {
				cli.RunTags = append(cli.RunTags, value)
			}
		}

		for _, value := range strings.Split(onlyTags, ",") {
			if value != "" {
				cli.ExecuteOnlyMatchedTags = true
				cli.RunTags = append(cli.RunTags, value)
			}
		}

		// todo: make this a function for CLI (addTag or something)
		cli.RunTagSet = mapset.NewSet()
		for _, tag := range cli.RunTags {
			cli.RunTagSet.Add(tag)
		}

		yamlString, err := ioutil.ReadFile(cli.YamlPath)
		utils.CheckError(err, "Unable to read yaml config.")

		fmt.Print("\033[?25l")       // hide cursor
		defer fmt.Print("\033[?25h") // show cursor
		Run(yamlString, cli)

	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().BoolVar(&cgroupsMode, "cgroups", false, "Cgroups Mode")
	runCmd.Flags().BoolVar(&devMode, "dev", false, "Dev Mode")
	runCmd.Flags().BoolVar(&listTagsMode, "list-tags", false, "List Tags")
	runCmd.Flags().BoolVar(&listTasksMode, "list-tasks", false, "List Tasks")

	runCmd.Flags().StringVar(&tags, "tags", "", "A comma delimited list of matching task tags. If a task's tag matches *or if it is not tagged* then it will be executed (also see --only-tags)")
	runCmd.Flags().StringVar(&onlyTags, "only-tags", "", "A comma delimited list of matching task tags. A task will only be executed if it has a matching tag")
}

func get_tasks(task_config *config.Config) []string {
	tasks := []string{}

	for _, tc := range task_config.TaskConfigs {
		parent_tags := []string{}
		for _, parent_tag := range tc.Tags {
			has := false
			for _, _t := range parent_tags {
				if parent_tag == _t {
					has = true
				}
			}
			if !has {
				parent_tags = append(parent_tags, parent_tag)
			}
		}
		for _, pt := range tc.ParallelTasks {
			child_tags := parent_tags
			for _, child_tag := range pt.Tags {
				has := false
				for _, _t := range child_tags {
					if child_tag == _t {
						has = true
					}
				}
				if !has {
					child_tags = append(child_tags, child_tag)
				}
			}

			qty := ``
			if len(pt.ForEach) > 0 {
				qty = fmt.Sprintf(` (%d Times)`, len(pt.ForEach))
			}
			t := fmt.Sprintf(`[%s] %s%s> %s`, tc.Name, pt.Name, qty, strings.Join(child_tags, ","))
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func get_tags(task_config *config.Config) []string {
	tags := []string{}
	for _, tc := range task_config.TaskConfigs {
		for _, t := range tc.Tags {
			has := false
			for _, _t := range tags {
				if t == _t {
					has = true
				}
			}
			if !has {
				tags = append(tags, t)
			}
		}
		for _, pt := range tc.ParallelTasks {
			for _, t := range pt.Tags {
				has := false
				for _, _t := range tags {
					if t == _t {
						has = true
					}
				}
				if !has {
					tags = append(tags, t)
				}
			}
		}
	}
	return tags
}

var shares uint64 = 200
var lim int64 = 20000
var cg_limit1 = &specs.LinuxResources{
	BlockIO: &specs.LinuxBlockIO{},
	CPU: &specs.LinuxCPU{
		Shares: &shares,
		//	Quota:  int64(10000),
		//Cpus: `0`,
	},
	Memory: &specs.LinuxMemory{
		Limit: &lim,
	},
	Pids: &specs.LinuxPids{
		Limit: 1000,
	},
}

func Run(yamlString []byte, cli config.Cli) {
	client, err := runtime.NewClientFromYaml(yamlString, &cli)
	if err != nil {
		utils.ExitWithErrorMessage(err.Error())
	}

	if devMode {
		fmt.Fprintf(os.Stdout, "%s\n", "dev mode")
		//d1()
		os.Exit(0)
	}
	if listTagsMode {
		tags := get_tags(client.Config)
		fmt.Fprintf(os.Stdout, "%s\n", strings.Join(tags, "\n"))
		os.Exit(0)
	}
	if listTasksMode {
		tags := get_tasks(client.Config)
		fmt.Fprintf(os.Stdout, "%s\n", strings.Join(tags, "\n"))
		//pp.Println(client.Config.TaskConfigs)
		os.Exit(0)
	}

	if client.Config.Options.SingleLineDisplay {
		client.AddEventHandler(handler.NewCompressedUI(client.Config))
	} else {
		client.AddEventHandler(handler.NewVerticalUI(client.Config))
	}
	client.AddEventHandler(handler.NewTaskLogger(client.Config))

	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath("/bashful-parent"), cg_limit1)
	if err == nil {
		if control.Add(cgroups.Process{Pid: syscall.Getpid()}) != nil {
			panic(err)
		}
		cmd_uuid := uuid.New()
		cmd_cg, err := control.New(cmd_uuid.String()+`-pidX`, cg_limit1)
		if err != nil {
			panic(err)

		}
		go func() {
			for {
				pids, err := process.Pids()
				if err == nil {
					for _, pid := range pids {
						if false {
							pp.Println(`pid>`, pid)
						}
						if cmd_cg.Add(cgroups.Process{Pid: int(pid)}) != nil {
							//					panic(err)
						}
					}
				}
				stats1, err1 := control.Stat()
				//stats1, err1 := control.Stat(cgroups.IgnoreNotExist)
				if err1 == nil {
					if false {
					}
					pp.Fprintf(os.Stderr, "%s\n", stats1)
				}
				stats, err := control.Stat()
				//stats, err := control.Stat(cgroups.IgnoreNotExist)
				if err == nil {
					pp.Fprintf(os.Stderr, "%s\n", stats)
					if false {
					}
				}
				time.Sleep(3 * time.Second)
			}
		}()
	}

	rand.Seed(time.Now().UnixNano())

	tagInfo := ""
	if len(cli.RunTags) > 0 {
		if cli.ExecuteOnlyMatchedTags {
			tagInfo = " only matching tags: "
		} else {
			tagInfo = " non-tagged and matching tags: "
		}
		tagInfo += strings.Join(cli.RunTags, ", ")
	}

	fmt.Println(utils.Bold("Running " + tagInfo))
	log.LogToMain("Running "+tagInfo, log.StyleMajor)

	failedTasksErr := client.Run()
	log.LogToMain("Complete", log.StyleMajor)

	log.LogToMain("Exiting", "")

	if failedTasksErr != nil {
		utils.Exit(1)
	}
}
