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
	PARENT_CGROUP_PATH := os.Getenv(`PARENT_CGROUP_PATH`)
	TASK_CG_PATH := fmt.Sprintf(`%s/%s`, PARENT_CGROUP_PATH, strings.Split(task.Id.String(), `-`)[0])

	PARENT_CGROUP_PID := os.Getenv(`PARENT_CGROUP_PID`)
	PARENT_CGROUP_UUID := os.Getenv(`PARENT_CGROUP_UUID`)
	task_cgroup, err := v2.LoadManager(BASE_CG_PATH, TASK_CG_PATH)
	utils.CheckError(err, "Could not open task cgroup")
	pp.Fprintf(os.Stderr, `%s`, task_cgroup)
	task_cgroup_controllers, err := task_cgroup.Controllers()
	utils.CheckError(err, "Could not open cgroups controllers")
	p_procs, err := task_cgroup.Procs(true)
	utils.CheckError(err, "Could not open cgroups procs")

	task_procs, err := task_cgroup.Procs(true)
	utils.CheckError(err, "Could not open cgroups procs")

	stats, err := task_cgroup.Stat()
	utils.CheckError(err, "Could not open cgroups stat")

	utils.GetColor(`Controllers`, `xxxxxx`)

	derr := task_cgroup.Delete()
	utils.CheckError(derr, "Could not delete cgroup")
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
		len(task_procs),
		utils.GetColor(`Controllers`, strings.Join(task_cgroup_controllers, `, `)),
		len(task_procs),
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
	cgroup_log(`task_end`, msg)
	if false {
		pp.Fprintf(os.Stderr, `%s`, task_cgroup_controllers)
		pp.Fprintf(os.Stderr, `%s`, p_procs)
		for _, pc := range task_cgroup_controllers {
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
