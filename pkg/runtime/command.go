package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/k0kubun/pp"
	"github.com/noirbizarre/gonja"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/utils"
)

var (
	BASH_TRACE_MODE = os.Getenv(`__BASHFUL_BASH_TRACE_MODE`)
	EXTRACE_MODE    = os.Getenv(`__BASHFUL_EXTRACE_MODE`)
)

type ModifiedCommand struct {
	Name    string
	Src     string
	Dest    string
	Context gonja.Context
	Error   error
	Vars    map[string]string
}

var cmd_counter uint64

type ModifiedCommands map[string]ModifiedCommand

var brief_cmd_mode = false

func newCommand(taskConfig config.TaskConfig) command {
	shell := `bash`

	_shell, err := exec.LookPath("bash")
	utils.CheckError(err, "Could not find bash")
	shell = _shell

	readFd, writeFd, err := os.Pipe()
	utils.CheckError(err, "Could not open env pipe for child shell")

	sudoCmd := ""
	if taskConfig.Sudo {
		sudoCmd = "sudo -nS "
	}
	extrace_args := ``
	extrace_path := ``
	_extrace_path, err := exec.LookPath("extrace")
	utils.CheckError(err, "Could not find extrace")
	extrace_path = _extrace_path
	extrace_log_dir := fmt.Sprintf(`/tmp`)
	atomic.AddUint64(&cmd_counter, 1)
	extrace_log := fmt.Sprintf(`%s/bashful-extrace-%d-%d.log`, extrace_log_dir, syscall.Getpid(), cmd_counter)
	extrace_args = `-Qultd`
	extrace_prefix := ``
	extrace_prefix = fmt.Sprintf(`%s %s -o %s`, extrace_path, extrace_args, extrace_log)

	prefix_exec_cmd := ``
	if BASH_TRACE_MODE == `1` {
		prefix_exec_cmd = strings.Trim(fmt.Sprintf(`
exec 19>>/tmp/bashful-bash-trace-log-$$.log
BASH_XTRACEFD=19 
set -x
`), ` `)
	}

	//pp.Println(taskConfig)
	var modified_commands = ModifiedCommands{
		`CmdString`:       {Src: taskConfig.CmdString},
		`PreCmdString`:    {Src: taskConfig.PreCmdString},
		`PostCmdString`:   {Src: taskConfig.PostCmdString},
		`RescueCmdString`: {Src: taskConfig.RescueCmdString},
		`DebugCmdString`:  {Src: taskConfig.DebugCmdString},
	}
	__rendered_cmds := map[string]string{}
	for mcn, _ := range modified_commands {
		//		pp.Println(taskConfig.ApplyEachVars)
		applied_vars := []map[string]string{
			taskConfig.Vars, taskConfig.Env,
		}
		_, has_all := taskConfig.ApplyEachVars[`*`]
		if has_all {
			applied_vars = append(applied_vars, taskConfig.ApplyEachVars[`*`])
		}
		_, has_cur := taskConfig.ApplyEachVars[taskConfig.CurrentItem]
		if has_cur {
			applied_vars = append(applied_vars, taskConfig.ApplyEachVars[taskConfig.CurrentItem])
		}
		//pp.Println(applied_vars)
		rendered_cmd, err := render_cmd(modified_commands[mcn].Src, applied_vars)
		if err != nil {
			panic(err)
		}
		__rendered_cmds[mcn] = rendered_cmd
	}

	if false {
		pp.Println(__rendered_cmds)
	}

	if len(__rendered_cmds[`CmdString`]) > 0 {
		taskConfig.CmdString = __rendered_cmds[`CmdString`]
	}

	if len(__rendered_cmds[`RescueCmdString`]) > 0 {
		//pp.Println(__rendered_cmds)
		taskConfig.CmdString = fmt.Sprintf(`%s || { %s && %s; }`,
			taskConfig.CmdString,
			__rendered_cmds[`RescueCmdString`],
			taskConfig.CmdString,
		)
	}
	if len(__rendered_cmds[`PreCmdString`]) > 0 {
		taskConfig.CmdString = fmt.Sprintf(`%s; %s`, __rendered_cmds[`PreCmdString`], taskConfig.CmdString)
		///pp.Println(`>         pre cmd:         ================================================================>   `, __rendered_cmds[`PreCmdString`])
	}
	if len(__rendered_cmds[`PostCmdString`]) > 0 {
		//	pp.Println(__rendered_cmds[`PostCmdString`])

		taskConfig.CmdString = fmt.Sprintf(`%s; %s`,
			taskConfig.CmdString,
			__rendered_cmds[`PostCmdString`],
		)
	}

	extrace_exec_cmd := strings.Trim(fmt.Sprintf(`%s %s %s; ec=$?; env >&3; exit $ec;`, sudoCmd, extrace_prefix, taskConfig.CmdString), ` `)
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
	brief_cmd_mode = true
	if brief_cmd_mode {
		exec_cmd = strings.Trim(fmt.Sprintf(`%s %s; ec=$?; env >&3; exit $ec;`, sudoCmd, taskConfig.CmdString), ` `)
	}
	if EXTRACE_MODE == `1` {
		exec_cmd = extrace_exec_cmd
	}
	cmd := exec.Command(shell, "--noprofile", "--norc", "+x", "+e", "-c", exec_cmd)
	cmd.Stdin = strings.NewReader(string(sudoPassword) + "\n")
	cmd.Dir = taskConfig.CwdString
	env := map[string]string{}
	cmd.ExtraFiles = []*os.File{writeFd}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
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
