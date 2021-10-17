package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	v2 "github.com/containerd/cgroups/v2"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/utils"
)

var parent_cgroup, bfcg *v2.Manager

func newCommand(taskConfig config.TaskConfig) command {
	shell := `bash`

	_shell, err := exec.LookPath("bash")
	utils.CheckError(err, "Could not find bash")
	shell = _shell
	readFd, writeFd, err := os.Pipe()
	utils.CheckError(err, "Could not open env pipe for child shell")

	if false {
		//readPidFd, writePidFd, err := os.Pipe()
		//utils.CheckError(err, "Could not open pid pipe for child shell")
	}

	sudoCmd := ""
	if taskConfig.Sudo {
		sudoCmd = "sudo -nS "
	}
	echo_pid_fd_cmd := ``
	exec_cmd := fmt.Sprintf(`%s %s %s; BASHFUL_RC=$?; env >&3; exit $BASHFUL_RC`,
		echo_pid_fd_cmd,
		sudoCmd,
		taskConfig.CmdString,
	)
	cmd := exec.Command(shell, "--noprofile", "--norc", "+e", "-c", exec_cmd)
	cmd.Stdin = strings.NewReader(string(sudoPassword) + "\n")

	// Set current working directory; default is empty
	cmd.Dir = taskConfig.CwdString
	env := map[string]string{}

	// allow the child process to provide env vars via a pipe (FD3)
	cmd.ExtraFiles = []*os.File{writeFd}

	// set this command as a process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return command{
		Environment: env,
		ReturnCode:  -1,
		EnvReadFile: readFd,
		//	PidReadFile:      readPidFd,
		Cmd:              cmd,
		EstimatedRuntime: time.Duration(-1),
		errorBuffer:      bytes.NewBufferString(""),
	}
}

func (cmd *command) addEstimatedRuntime(duration time.Duration) {
	cmd.EstimatedRuntime = duration
}
