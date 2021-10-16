package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	guuid "github.com/gofrs/uuid"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/utils"
)

var PARENT_CGROUP_NAME = `bashful`
var shares uint64 = 200
var lim int64 = 20000
var cg_limit1 = &specs.LinuxResources{
	BlockIO: &specs.LinuxBlockIO{},
	CPU: &specs.LinuxCPU{
		Shares: &shares,
		//  Quota:  int64(10000),
		//Cpus: `0`,
	},
	Memory: &specs.LinuxMemory{
		Limit: &lim,
	},
	Pids: &specs.LinuxPids{
		Limit: 1000,
	},
}

func newCommand(taskConfig config.TaskConfig) command {
	//if taskConfig.cmd_cg.Add(cgroups.Process{Pid: syscall.Getpid()}) != nil {
	//	panic(err)
	//}

	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}

	readFd, writeFd, err := os.Pipe()
	utils.CheckError(err, "Could not open env pipe for child shell")

	sudoCmd := ""
	if taskConfig.Sudo {
		sudoCmd = "sudo -S "
	}
	shell = `bash`
	pids_max := 30
	cmd_uuid := uuid.New()
	//	pp.Println(cmd_uuid.String(), syscall.Getpid())

	env_cmd := fmt.Sprintf(`env __BASHFUL_PARENT_PID=%d __BASHFUL_PID=$$ __BASHFUL_UUID=%s`, os.Getpid(), cmd_uuid.String())
	mount_cmd := fmt.Sprintf(`mount -t cgroup -o pids,cpu,cpuacct,blkio,memory,net_cls,net_prio none /sys/fs/cgroup/%s 2>/dev/null||true`, cmd_uuid.String())
	limit_cmds := fmt.Sprintf(`echo %d > /sys/fs/cgroup/%s/pids.max`, pids_max, cmd_uuid.String())
	mem_limit_megabytes := 1000
	mem_limit_bytes := mem_limit_megabytes * 1000000

	limit_cmds = fmt.Sprintf(`%s; echo %d > /sys/fs/cgroup/%s/memory.max`, limit_cmds, mem_limit_bytes, cmd_uuid.String())
	limit_cmds = fmt.Sprintf(`%s; echo $$ > /sys/fs/cgroup/%s/cgroup.procs`, limit_cmds, cmd_uuid.String())
	/*
		for k, v := range taskConfig.CgroupLimits {
			if false {
				pp.Println(k, v)
			}
			for _k, _v := range v {
				vf := fmt.Sprintf(`echo %d > /sys/fs/cgroup/%s/%s.%s`, _v, cmd_uuid.String(), k, _k)
				if false {
					pp.Println(_k, _v, vf, v)
				}
				limit_cmds = fmt.Sprintf(`%s; %s`, limit_cmds, vf)
			}
		}
	*/
	prefix_cmd := fmt.Sprintf(`{ date; echo %s; mkdir -p /sys/fs/cgroup/%s; %s; %s; } | tee -a /tmp/prefix-started.log`,
		cmd_uuid.String(),
		cmd_uuid.String(),
		mount_cmd,
		limit_cmds,
	)
	log_cmd := fmt.Sprintf(`jo uuid="%s" cmd="%s" cpu="$(jo -a "$(cat /var/spool/provision/acct/%s/cpu.stat)")"                      io="$(jo -a "$(cat /var/spool/provision/acct/%s/io.stat)")"    mem="$(jo -a "$(cat /var/spool/provision/acct/%s/memory.stat)")"          | jq -Mrc |  tee -a /var/log/provision.acct.log`,
		cmd_uuid.String(),
		taskConfig.CmdString,
		cmd_uuid.String(),
		cmd_uuid.String(),
		cmd_uuid.String(),
	)
	collect_cmd := fmt.Sprintf(`( mkdir -p /var/spool/provision/acct/%s 2>/dev/null||true; cd /sys/fs/cgroup/%s/. && ls *.stat |while read -r l; do cat /sys/fs/cgroup/%s/$l |tee /var/spool/provision/acct/%s/$l; done; )`,
		cmd_uuid.String(),
		cmd_uuid.String(),
		cmd_uuid.String(),
		cmd_uuid.String(),
	)
	suffix_cmd := fmt.Sprintf(`{ %s; }; { %s; }; date | tee -a /tmp/prefix-ended.log`, collect_cmd, log_cmd)
	//	pp.Println(taskConfig)
	for k, v := range taskConfig.Env {
		env_cmd = fmt.Sprintf(`%s %s=%s`, env_cmd, k, v)
	}
	//env_cmd = fmt.Sprintf(``)
	//	prefix_cmd = `echo OK`
	exec_cmd := fmt.Sprintf(`{ %s; }; %s %s %s; export BASHFUL_RC=$?; { %s; }; export -n BASHFUL_RC; env >&3; exit $BASHFUL_RC`,
		prefix_cmd,
		sudoCmd,
		env_cmd, taskConfig.CmdString,
		suffix_cmd,
	)
	if false {
		if taskConfig.SetupTimestamp < 1 {
			taskConfig.SetupTimestamp = int64(time.Now().UnixNano())
			bcg_uuid := guuid.Must(guuid.NewV4())
			parent_cg, err := cgroups.New(cgroups.V1, cgroups.StaticPath(fmt.Sprintf("/%s/%s", strings.Split(PARENT_CGROUP_NAME, `-`)[0], strings.Split(bcg_uuid.String(), `-`)[0])), cg_limit1)
			if err == nil {
				taskConfig.BCG = config.BashfulCgroup{
					ParentUUID:      bcg_uuid,
					ParentResources: cg_limit1,
					TaskCgroups:     map[string]cgroups.Cgroup{},
					CommandCgroups:  map[string]cgroups.Cgroup{},
					ParentCgroup:    parent_cg,
					CgroupIDs:       []string{},
				}
			}
		}

	}

	//	if taskConfig.BCG.ParentCgroup.Add(cgroups.Process{Pid: syscall.Getpid()}) != nil {
	//		fmt.Fprintf(os.Stderr, "NEW COMMAND>> Addded PID %d\n", syscall.Getpid())
	//	}
	taskConfig.CgroupsEnabled = false
	if taskConfig.CgroupsEnabled {
		exec_cmd = fmt.Sprintf(`%s %s %s; BASHFUL_RC=$?; env >&3; exit $BASHFUL_RC;`,
			sudoCmd,
			env_cmd, taskConfig.CmdString,
		)
	}
	cmd := exec.Command(shell, "--noprofile", "--norc", "+e", "-c", exec_cmd)
	///cmd := exec.Command(shell, "-c", exec_cmd)

	cmd.Stdin = strings.NewReader(string(sudoPassword) + "\n")

	// Set current working directory; default is empty
	cmd.Dir = taskConfig.CwdString
	env := map[string]string{}

	// allow the child process to provide env vars via a pipe (FD3)
	cmd.ExtraFiles = []*os.File{writeFd}

	// set this command as a process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		//Cloneflags: syscall.CLONE_NEWPID,
		//		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		// | syscall.CLONE_NEWUSER
	}

	return command{
		Environment:      env,
		ReturnCode:       -1,
		EnvReadFile:      readFd,
		Cmd:              cmd,
		EstimatedRuntime: time.Duration(-1),
		errorBuffer:      bytes.NewBufferString(""),
	}
}

func (cmd *command) addEstimatedRuntime(duration time.Duration) {
	cmd.EstimatedRuntime = duration
}
