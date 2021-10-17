package runtime

import (
	"fmt"
	"os"
	"strings"

	v2 "github.com/containerd/cgroups/v2"
	"github.com/k0kubun/pp"
	"github.com/wagoodman/bashful/utils"
)

const BASE_CG_PATH = `/sys/fs/cgroup`

func cgroup_task_ended(task *Task) {

	PARENT_CGROUP_PID := os.Getenv(`PARENT_CGROUP_PID`)
	PARENT_CGROUP_UUID := os.Getenv(`PARENT_CGROUP_UUID`)
	PARENT_CGROUP_PATH := os.Getenv(`PARENT_CGROUP_PATH`)
	parent_cgroup, err := v2.LoadManager(BASE_CG_PATH, PARENT_CGROUP_PATH)
	utils.CheckError(err, "Could not open cgroups stat")

	parent_controllers, err := parent_cgroup.Controllers()
	utils.CheckError(err, "Could not open cgroups controllers")
	p_procs, err := parent_cgroup.Procs(true)
	utils.CheckError(err, "Could not open cgroups procs")

	stats, err := parent_cgroup.Stat()
	utils.CheckError(err, "Could not open cgroups stat")
	utils.GetColor(`Controllers`, `xxxxxx`)

	msg := fmt.Sprintf(`+++++++++++++++++++++++++++++++++++++++++
PID %s exited %d in %s

Task:
	ID:                          %s

Command:
	Cmd:                         %s
	Exec Path:                   %s
	Exec:                        %s
  # Env:                       %s

Cgroup:                        %s
	%d Controllers:              %s
	Current Procs:               %d
	Pids
		Limit:                     %d
	CPU
		Usage:                     %d
		Throttled:                 %d
	Memory
		Usage:                     %d/%d
		Mem+Swap Usage:            %d/%d

Process Usage: 
	System CPU Time:             %s
	User   CPU Time:             %s
	Success?                     %v

Environment
	PARENT_CGROUP_PID:              %s
	PARENT_CGROUP_UUID:             %s
	PARENT_CGROUP_PATH:             %s

+++++++++++++++++++++++++++++++++++++++++
`,
		//info
		utils.GetColor(`Pid`, fmt.Sprintf(`%d`, task.Command.Cmd.Process.Pid)),
		utils.GetColor(`ExitCode`, fmt.Sprintf(`%d`, task.Command.ReturnCode)),
		task.Command.StopTime.Sub(task.Command.StartTime),

		//task
		utils.GetColor(`Task`, strings.Split(task.Id.String(), `-`)[0]),

		//command
		utils.GetColor(`Cmd`, task.Config.CmdString),
		utils.GetColor(`Path`, task.Command.Cmd.Path),
		utils.GetColor(`Command`, strings.Join(task.Command.Cmd.Args, ` `)),
		utils.GetColor(`Environment`, fmt.Sprintf(`%d`, len(task.Command.Cmd.Env))),

		//cgroup
		task.CGPath,
		len(parent_controllers),
		utils.GetColor(`Controllers`, strings.Join(parent_controllers, `, `)),
		len(p_procs),
		stats.Pids.Limit,
		stats.CPU.UsageUsec, stats.CPU.ThrottledUsec,
		stats.Memory.Usage, stats.Memory.UsageLimit, stats.Memory.SwapUsage, stats.Memory.SwapLimit,

		//pid
		task.Command.Cmd.ProcessState.SystemTime(),
		task.Command.Cmd.ProcessState.UserTime(),
		task.Command.Cmd.ProcessState.Success(),

		//env
		PARENT_CGROUP_PID, PARENT_CGROUP_UUID, PARENT_CGROUP_PATH,
	)
	if false {
		pp.Fprintf(os.Stderr, `%s`, parent_controllers)
		pp.Fprintf(os.Stderr, `%s`, p_procs)
		for _, pc := range parent_controllers {
			pp.Fprintf(os.Stderr, `%s`, pc)
		}
		//	pp.Fprintf(os.Stderr, `%s`, stats)
		//pp.Fprintf(os.Stderr, `%s`, stats.Io)
		fmt.Fprintf(os.Stderr, pp.Sprintf(`%s`, task.Command.Cmd.ProcessState))
		//json.NewEncoder(os.Stderr).Encode(stats)
	}
	if DEBUG_CG_END {
		fmt.Fprintf(os.Stderr, utils.Bold(msg))
	}
}
