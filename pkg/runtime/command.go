package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/utils"
)

func newCommand(taskConfig config.TaskConfig) command {
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
	shell = `/bin/bash`
	env_cmd := fmt.Sprintf(`env`)
	//	pp.Println(taskConfig)
	for k, v := range taskConfig.Env {
		env_cmd = fmt.Sprintf(`%s %s='%s'`, env_cmd, k, v)
	}
	env_cmd = fmt.Sprintf(``)
	exec_cmd := fmt.Sprintf(`%s%s %s; BASHFUL_RC=$?; env >&3; exit $BASHFUL_RC`, sudoCmd, env_cmd, taskConfig.CmdString)
	cmd := exec.Command(shell, "--noprofile", "--norc", "+e", "-c", exec_cmd)

	cmd.Stdin = strings.NewReader(string(sudoPassword) + "\n")

	// Set current working directory; default is empty
	cmd.Dir = taskConfig.CwdString
	env := map[string]string{}

	// allow the child process to provide env vars via a pipe (FD3)
	cmd.ExtraFiles = []*os.File{writeFd}

	// set this command as a process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
