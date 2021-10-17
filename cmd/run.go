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

	v2 "github.com/containerd/cgroups/v2"

	"github.com/containerd/cgroups"
	mapset "github.com/deckarep/golang-set"
	"github.com/google/gops/agent"
	"github.com/google/uuid"
	"github.com/k0kubun/pp"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/shirou/gopsutil/process"

	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	guuid "github.com/gofrs/uuid"
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

const BASE_CG_PATH = `/sys/fs/cgroup`

var DEBUG_CG = false
var CGROUPS_MODE = get_cg_mode()
var parent_cgroup, bfcg *v2.Manager
var BASHFUL_CGROUP_NAME = `bashful`
var PARENT_CGROUP_UUID = strings.Split(guuid.Must(guuid.NewV4()).String(), `-`)[0]
var BASHFUL_CGROUP_PATH = fmt.Sprintf(`/%s`, BASHFUL_CGROUP_NAME)
var PARENT_CGROUP_PATH = fmt.Sprintf(`%s/%s`, BASHFUL_CGROUP_PATH, PARENT_CGROUP_UUID)
var GOPS_ENABLED = false
var CG_VER = 0

var swap_max int64 = 2048 * 1000 * 1000
var mem_max int64 = 1024 * 1000 * 1000
var proc_max int64 = 500
var BashfulResources = v2.Resources{
	Pids: &v2.Pids{
		Max: proc_max,
	},
	Memory: &v2.Memory{
		Max:  &mem_max,
		Swap: &swap_max,
	},
	IO: &v2.IO{},
}

func gops_init() {
	if GOPS_ENABLED {
		go func() {
			for {
				if err := agent.Listen(agent.Options{
					ShutdownCleanup: true, // automatically closes on os.Interrupt
				}); err != nil {
					fmt.Errorf(`gops err> %s`, err)
				}
				time.Sleep(time.Hour)
			}
		}()
	}
}

func get_cg_mode() string {
	cg_mode := cgroups.Mode()
	switch cg_mode {
	case cgroups.Legacy:
		CG_VER = 1
		return "legacy"
	case cgroups.Hybrid:
		return fmt.Sprintf("hybrid")
	case cgroups.Unified:
		CG_VER = 2
		return fmt.Sprintf("unified")
	case cgroups.Unavailable:
		return fmt.Sprintf("cgroups unavailable")
	}
	return ``
}

func cg_init() {

	os.Setenv(`PARENT_CGROUP_PID`, fmt.Sprintf("%d", syscall.Getpid()))
	os.Setenv(`CGROUPS_MODE`, CGROUPS_MODE)
	os.Setenv(`PARENT_CGROUP_UUID`, PARENT_CGROUP_UUID)
	os.Setenv(`PARENT_CGROUP_PATH`, PARENT_CGROUP_PATH)
	os.Setenv(`CGROUPS_BASE_CG_PATH`, BASE_CG_PATH)
	os.Setenv(`BASHFUL_CGROUP_PATH`, BASHFUL_CGROUP_PATH)

	if false {
		if CG_VER == 2 {
			_bfcg, err := v2.LoadManager(BASE_CG_PATH, BASHFUL_CGROUP_PATH)
			if err != nil {
				_, err := v2.NewManager(BASE_CG_PATH, BASHFUL_CGROUP_PATH, &v2.Resources{})
				if err != nil {
					panic(err)
				}
				_bfcg, err := v2.LoadManager(BASE_CG_PATH, BASHFUL_CGROUP_PATH)
				if err != nil {
					panic(err)
				}
				bfcg = _bfcg
			} else {
				bfcg = _bfcg
			}

			root_controllers, err := bfcg.RootControllers()
			if err != nil {
				panic(err)
			}

			_parent_cgroup, err := v2.NewManager(BASE_CG_PATH, PARENT_CGROUP_PATH, &BashfulResources)
			if err != nil {
				panic(err)
			}
			parent_cgroup = _parent_cgroup
			if err := parent_cgroup.ToggleControllers(root_controllers, v2.Enable); err != nil {
				panic(err)
			}
			if false {
				parent_controllers, err := parent_cgroup.Controllers()
				if err != nil {
					panic(err)
				}
				stats, err := parent_cgroup.Stat()
				if err != nil {
					panic(err)
				}

				_, err = bfcg.Procs(true)
				if err != nil {
					panic(err)
				}

				p_procs, err := parent_cgroup.Procs(true)
				if err != nil {
					panic(err)
				}
				if DEBUG_CG {
					pp.Println(stats)
					fmt.Printf("<ROOT>    %s  %d Root Controllers: %s\n", len(root_controllers), root_controllers)
					fmt.Printf("<PARENT>  %s %d Procs| %d Parent Controllers: %s\n", PARENT_CGROUP_PATH, len(p_procs), len(parent_controllers), parent_controllers)
				}
			}
		}
	}
}

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
		parent_cg_uuid := guuid.Must(guuid.NewV4())
		cli := config.Cli{
			YamlPath: args[0],
			BashfulCgroup: config.BashfulCgroup{
				ParentUUID:      parent_cg_uuid,
				ParentResources: cg_limit1,
				TaskCgroups:     map[string]cgroups.Cgroup{},
				CommandCgroups:  map[string]cgroups.Cgroup{},
				//ParentCgroup:    parent_cg,
				CgroupIDs: []string{},
			},
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
	gops_init()
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().BoolVar(&cgroupsMode, "cgroups", false, "Cgroups Mode")
	runCmd.Flags().BoolVar(&devMode, "dev", false, "Dev Mode")
	runCmd.Flags().BoolVar(&listTagsMode, "list-tags", false, "List Tags")
	runCmd.Flags().BoolVar(&listTasksMode, "list-tasks", false, "List Tasks")

	runCmd.Flags().StringVar(&tags, "tags", "", "A comma delimited list of matching task tags. If a task's tag matches *or if it is not tagged* then it will be executed (also see --only-tags)")
	runCmd.Flags().StringVar(&onlyTags, "only-tags", "", "A comma delimited list of matching task tags. A task will only be executed if it has a matching tag")
	if false {
		cg_init()
	}
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

var found_pids = []int32{}
var DEBUG_BF = false

func Run(yamlString []byte, cli config.Cli) {
	if DEBUG_BF {
		pp.Fprintf(os.Stderr, "RUN> %s %d\n", uuid.New().String(), syscall.Getpid())
	}
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
		os.Exit(0)
	}

	if client.Config.Options.SingleLineDisplay {
		client.AddEventHandler(handler.NewCompressedUI(client.Config))
	} else {
		client.AddEventHandler(handler.NewVerticalUI(client.Config))
	}
	client.AddEventHandler(handler.NewTaskLogger(client.Config))
	//client.AddEventHandler(handler.NewSimpleLogger(client.Config))
	client.AddEventHandler(handler.NewEnhancedLogger(client.Config))

	if false {
		go func() {
			for {
				pids, err := process.Pids()
				if err == nil {
					for _, pid := range pids {
						has := false
						for _, p := range found_pids {
							if p == pid {
								has = true
							}
						}
						if !has {
							found_pids = append(found_pids, pid)
							//						if cli.BashfulCgroup.ParentCgroup.Add(cgroups.Process{Pid: int(pid)}) != nil {
							//						panic(err)
							//				}
						}
						if false {
						}
						fmt.Fprintf(os.Stderr, "ADDING pid> %s to %d PIDs\n", pid, len(found_pids))
						//						if cli.BashfulCgroup.ParentCgroup.Add(cgroups.Process{Pid: int(pid)}) != nil {
						//						//					panic(err)
						//				}
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
		tagInfo += strings.Join(cli.RunTags, ",")
	}

	fmt.Println(utils.Bold("Running " + tagInfo))
	log.LogToMain("Running "+tagInfo, log.StyleMajor)

	failedTasksErr := client.Run()
	log.LogToMain("Complete", log.StyleMajor)

	log.LogToMain("Exiting", "")

	remove_parent_cgroup()

	if failedTasksErr != nil {
		utils.Exit(1)
	}
}
