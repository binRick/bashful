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

var (
	BASH_TRACE_MODE = os.Getenv(`__BASHFUL_BASH_TRACE_MODE`)
	EXTRACE_MODE    = os.Getenv(`__BASHFUL_EXTRACE_MODE`)
)

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
	extrace_args := ``
	extrace_path := ``
	if EXTRACE_MODE == `1` {
		_extrace_path, err := exec.LookPath("extrace")
		utils.CheckError(err, "Could not find extrace")
		extrace_path = _extrace_path
		extrace_log := `/tmp/bashful-extrace-$$.log`
		extrace_args = `-Qfultd`
		sudoCmd = fmt.Sprintf(`%s %s`, sudoCmd, fmt.Sprintf(`%s %s -o %s`, extrace_path, extrace_args, extrace_log))
	}

	prefix_exec_cmd := ``
	if BASH_TRACE_MODE == `1` {
		prefix_exec_cmd = strings.Trim(fmt.Sprintf(`
exec 19>>/tmp/bashful-bash-trace-log-$$.log
BASH_XTRACEFD=19 
set -x
`), ` `)
	}
	exec_cmd := strings.Trim(fmt.Sprintf(`%s
eval "$(cat <<EOF
%s %s
EOF
)"
BASHFUL_RC=$? 
env >&3 
exit $BASHFUL_RC
`,
		prefix_exec_cmd,
		sudoCmd,
		taskConfig.CmdString,
	), ` `)
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
