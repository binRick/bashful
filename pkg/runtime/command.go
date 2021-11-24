package runtime

import (
	"bytes"
	"encoding/base64"
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
	BASH_TRACE_MODE       = os.Getenv(`__BASHFUL_BASH_TRACE_MODE`)
	EXTRACE_MODE          = os.Getenv(`__BASHFUL_EXTRACE_MODE`)
	BASHFUL_EXEC_HOSTNAME = os.Getenv(`__BASHFUL_EXEC_HOSTNAME`)
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

var (
	extrace_args           = `-Qultd`
	SET_TIMEHISTORY_ENV    = fmt.Sprintf(`export TIMEHISTORY_LIMIT=%d`, 200)
	TIMEHISTORY_ENABLED    = (os.Getenv(`TIMEHISTORY_ENABLED`) == `1`)
	RESET_TIMEHISTORY_CMD  = `command timehistory -R >/dev/null`
	brief_cmd_mode         = false
	concurrent_lib         = ``
	events                 = ``
	events_encoded         = ``
	concurrent_lib_encoded = ``
)

func init() {
	TIMEHISTORY_ENABLED = false
	events_encoded = `ZWNobyBvawo=` //utils.ENCODED_BASH_EVENTS
	/*
		_concurrent_lib, err := ioutil.ReadFile(`./bash_utils/concurrent.lib.sh`)
		if err != nil {
			panic(err)
		}
	*/
	_concurrent_lib := []byte(`ZWNobyBvawo=`)
	concurrent_lib = fmt.Sprintf(`%s`, _concurrent_lib)
	concurrent_lib_encoded = base64.StdEncoding.EncodeToString([]byte(concurrent_lib))
}

func newCommand(taskConfig config.TaskConfig) command {
	BASHFUL_EXEC_HOSTNAME = os.Getenv(`__BASHFUL_EXEC_HOSTNAME`)
	if BASHFUL_EXEC_HOSTNAME == `` {
		BASHFUL_EXEC_HOSTNAME = `localhost`
	}
	shell := `bash`

	_extrace_path, err := exec.LookPath("extrace")
	utils.CheckError(err, "Could not find extrace")
	_shell, err := exec.LookPath(shell)
	utils.CheckError(err, "Could not find shell")
	_sudo, err := exec.LookPath(`sudo`)
	utils.CheckError(err, "Could not find sudo")
	shell = _shell

	readFd, writeFd, err := os.Pipe()
	utils.CheckError(err, "Could not open env pipe for child shell")

	sudoCmd := ""
	if taskConfig.Sudo {
		sudoCmd = fmt.Sprintf("%s -nS ", _sudo)
	}

	extrace_args := ``
	extrace_path := ``
	extrace_path = _extrace_path
	extrace_log_dir := fmt.Sprintf(`/tmp`)
	atomic.AddUint64(&cmd_counter, 1)
	extrace_log := fmt.Sprintf(`%s/bashful-extrace-%d-%d.log`, extrace_log_dir, syscall.Getpid(), cmd_counter)
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

	if taskConfig.Concurrent != nil {
		concurrent_cmd := ``
		ccs := []string{}
		fxns := []string{}
		reqs := []string{}
		for k := range taskConfig.Concurrent {
			C := taskConfig.Concurrent[k]
			C.Name = strings.Replace(C.Name, ` `, `_`, -1)
			if C.StdoutLogFile == `` {
				C.StdoutLogFile = `/dev/null`
				C.StdoutLogFile = `/proc/self/fd/1`
			}
			if C.StderrLogFile == `` {
				C.StderrLogFile = `/dev/null`
				C.StderrLogFile = `/proc/self/fd/2`
			}
			sub_cmd := fmt.Sprintf(`{ %s; } > %s 2> %s`,
				C.Command,
				C.StdoutLogFile,
				C.StderrLogFile,
			)
			fxns = append(fxns, fmt.Sprintf("eval_%s(){\n %s;\n}",
				C.Name,
				sub_cmd,
			))
			ccs = append(ccs, fmt.Sprintf("    - \"%s\" eval_%s",
				C.Name,
				C.Name,
			))
			for _, req := range C.Requires {
				reqs = append(reqs, fmt.Sprintf("    --require \"%s\"       --before \"%s\"",
					req,
					C.Name,
				))
			}
		}
		concurrent_cmd = fmt.Sprintf("%s\n#  Concurrent Functions", concurrent_cmd)
		for _, l := range fxns {
			concurrent_cmd = fmt.Sprintf("%s\n%s", concurrent_cmd, l)
		}
		concurrent_cmd = fmt.Sprintf(`%s

run_concurrents() {
  local concurrent_args=(`, concurrent_cmd)
		for _, _cc := range ccs {
			concurrent_cmd = fmt.Sprintf("%s\n%s", concurrent_cmd, _cc)
		}
		for _, l := range reqs {
			concurrent_cmd = fmt.Sprintf("%s\n%s", concurrent_cmd, l)
		}
		concurrent_cmd = fmt.Sprintf(`%s
  )
  concurrent "${concurrent_args[@]}"
}
`,
			concurrent_cmd,
		)

		//bash-5.1#  export TIMEHISTORY_FORMAT='%(time:%s)\t%(pid)\t%(cpu)\t%(status)/%Tt/%Tx\t\t%(sys_time_us)/%(user_time_us)\t\t\t%(inblock)/%(oublock)\t%(maxrss)\t\t\t\t\t\t%(args)' && timehistory
		// exclude:            /concurrent.lib.sh.          bashful/.logs/       ["date","+%F@%T"]}        ["base64","-d"]
		prefix_cmd := `echo`
		suffix_cmd := `echo`
		if TIMEHISTORY_ENABLED {
			if len(taskConfig.TimehistoryJsonLogFile) > 0 {
				prefix_cmd = fmt.Sprintf(`%s && command -v timehistory >/dev/null || { enable -f ./bash_utils/libtimehistory_bash.so timehistory; } && %s`,
					SET_TIMEHISTORY_ENV,
					RESET_TIMEHISTORY_CMD,
				)
				if false {
					prefix_cmd = fmt.Sprintf(`%s && mycallback() { event on event1 echo "nested!" >> /tmp/ed; } && event on "event1" mycallback && event fire "event1"`,
						prefix_cmd,
					)
				}
				s1 := fmt.Sprintf(`enable -d timehistory||true`)
				suffix_cmd = fmt.Sprintf(`command -v timehistory >/dev/null && { timehistory -j >> %s; %s; }`, taskConfig.TimehistoryJsonLogFile, s1)
			}
		}
		if DEBUG_MODE {
			fmt.Fprintf(os.Stderr, "%s\n", concurrent_cmd)
		}
		concurrent_cmd = fmt.Sprintf(`( eval "$(echo %s|base64 -d)" && eval "$(echo %s|base64 -d)" && %s && eval "$(echo %s|base64 -d)" && run_concurrents; %s; ) >> /tmp/t1 2>&1`,
			events_encoded,
			concurrent_lib_encoded,
			prefix_cmd,
			base64.StdEncoding.EncodeToString([]byte(concurrent_cmd)),
			suffix_cmd,
		)
		taskConfig.CmdString = fmt.Sprintf(`( %s ) && %s`, concurrent_cmd, taskConfig.CmdString)
	}
	if len(taskConfig.CmdGenerator) > 0 {
		modified_cmd := taskConfig.CmdString
		modified_OrigCmdString := strings.Replace(taskConfig.OrigCmdString, taskConfig.ReplicaReplaceString, `${CMD_GENERATED_ITEM}`, -1)
		module_hosts := []string{`localhost`}
		adhoc := NewAdhoc(`shell`, map[string]interface{}{`cmd`: taskConfig.CmdGenerator}, module_hosts)
		_cmd, err := adhoc.Command()
		if err != nil {
			panic(err)
		}
		_adhoc_cmd := strings.Join(_cmd, ` `)
		_adhoc_cmd = strings.Replace(_adhoc_cmd, `--one-line`, ``, -1)
		ANSIBLE_STDOUT_EXTRACTOR := fmt.Sprintf(`command jq '.plays[0].tasks[0].hosts.localhost.stdout' -Mrc`)
		ANSIBLE_ENV := fmt.Sprintf(`ANSIBLE_STDOUT_CALLBACK=json ANSIBLE_LOAD_CALLBACK_PLUGINS=1`)
		_adhoc_cmd = fmt.Sprintf(`ansible localhost, --inventory %s, --limit localhost, --module-name shell  --connection local -a "%s"`,
			strings.Join(module_hosts, `,`),
			taskConfig.CmdGenerator,
		)
		_adhoc_cmd = fmt.Sprintf(`env %s %s | %s`, ANSIBLE_ENV, _adhoc_cmd, ANSIBLE_STDOUT_EXTRACTOR)
		modified_cmd = fmt.Sprintf(`while read -r CMD_GENERATED_ITEM; do ORIGINAL_ITEM="%s" && %s; done < <(%s)`,
			taskConfig.CurrentItem,
			modified_OrigCmdString,
			_adhoc_cmd,
		)
		if DEBUG_MODE {
			pp.Println(taskConfig)
			pp.Println(`generator:`, taskConfig.CmdGenerator)
			pp.Println(`adhoc`, adhoc)
			pp.Println(`orig cmd`, taskConfig.OrigCmdString)
			pp.Println(`orig generator`, taskConfig.OrigCmdGenerator)
			pp.Println(`modified generator`, modified_cmd)
			pp.Println(`modified cmd`, taskConfig.CmdString)
			fmt.Println(modified_cmd)
		}
		taskConfig.CmdString = modified_cmd
	}

	if len(taskConfig.Ansible) > 0 {
		if VERBOSE_MODE {
			pp.Println(taskConfig.Ansible)
		}
		for module_name, module_args := range taskConfig.Ansible {
			_, has_options := module_args[`options`]
			_, has_args := module_args[`args`]
			if has_options && has_args {

				module_hosts := []string{`localhost`}
				if len(BASHFUL_EXEC_HOSTNAME) > 0 {
					module_hosts = []string{
						BASHFUL_EXEC_HOSTNAME,
					}
				}

				_, has_enabled := module_args[`options`][`enabled`]
				if has_enabled {
					adhoc := NewAdhoc(module_name, module_args[`args`], module_hosts)
					orig_cmd := taskConfig.CmdString
					modified_cmd := orig_cmd
					_cmd, err := adhoc.Command()
					if err != nil {
						panic(err)
					}
					_adhoc_cmd := strings.Join(_cmd, ` `)
					_, has_before_cmd := module_args[`options`][`before-command`]
					_, has_after_cmd := module_args[`options`][`after-command`]
					if module_args[`options`][`before-command`].(bool) {
						modified_cmd = fmt.Sprintf(`%s && %s`, _adhoc_cmd, modified_cmd)
					}
					if has_after_cmd {
						modified_cmd = fmt.Sprintf(`%s && %s`,
							modified_cmd,
							_adhoc_cmd,
						)
					}
					taskConfig.CmdString = modified_cmd
					if VERBOSE_MODE {
						fmt.Fprintf(os.Stderr, `
Hostname:       %s

Cmd Before:     %s
Cmd After:      %s

Ansible Module:              %s
Ansible Cmd:                 %s
Ansible Module Args:         %s
Ansible Module Options:      %s
%d Ansible Module Hosts:     %s

Has Enabled:    %v
Enabled:        %v
Has Before cmd: %v
Before cmd:     %v
Has After cmd:  %v
After cmd:      %v

`,
							BASHFUL_EXEC_HOSTNAME,
							orig_cmd,
							modified_cmd,
							module_name,
							_adhoc_cmd,
							pp.Sprintf(`%s`, module_args[`args`]),
							pp.Sprintf(`%s`, module_args[`options`]),
							len(module_hosts),
							pp.Sprintf(`%s`, module_hosts),

							has_enabled, module_args[`options`][`enabled`],
							has_before_cmd,

							module_args[`options`][`before-command`],

							has_after_cmd,
							module_args[`options`][`after-command`],
						)
					}
				}
			}
		}
	}

	var modified_commands = ModifiedCommands{
		`CmdString`:       {Src: taskConfig.CmdString},
		`PreCmdString`:    {Src: taskConfig.PreCmdString},
		`PostCmdString`:   {Src: taskConfig.PostCmdString},
		`RescueCmdString`: {Src: taskConfig.RescueCmdString},
		`DebugCmdString`:  {Src: taskConfig.DebugCmdString},
	}

	__rendered_cmds := map[string]string{}
	for mcn, _ := range modified_commands {
		applied_vars := []map[string]string{
			taskConfig.Vars, taskConfig.Env,
		}
		if VERBOSE_MODE {
			pp.Fprintf(os.Stderr, "\nTask Config:\n%s\n\n", taskConfig)
			pp.Fprintf(os.Stderr, "\nglobal_task_vars:\n%s\n\n", global_task_vars)
		}
		_, has_all := taskConfig.ApplyEachVars[`*`]
		if has_all {
			applied_vars = append(applied_vars, taskConfig.ApplyEachVars[`*`])
		}
		_, has_cur := taskConfig.ApplyEachVars[taskConfig.CurrentItem]
		if has_cur {
			applied_vars = append(applied_vars, taskConfig.ApplyEachVars[taskConfig.CurrentItem])
		}

		if VERBOSE_MODE {
			fmt.Fprintf(os.Stderr, "\nApplied Vars:\n%s\n\n", pp.Sprintf(`%s`, applied_vars))
		}
		rendered_cmd, err := render_cmd(modified_commands[mcn].Src, applied_vars)
		if err != nil {
			panic(err)
		}
		__rendered_cmds[mcn] = rendered_cmd
	}

	if VERBOSE_MODE {
		pp.Fprintf(os.Stderr, "Rendered Commands: %s", __rendered_cmds)
	}

	if len(__rendered_cmds[`CmdString`]) > 0 {
		taskConfig.CmdString = __rendered_cmds[`CmdString`]
	}

	if len(__rendered_cmds[`RescueCmdString`]) > 0 {
		taskConfig.CmdString = fmt.Sprintf(`%s || { %s && %s; }`,
			taskConfig.CmdString,
			__rendered_cmds[`RescueCmdString`],
			taskConfig.CmdString,
		)
	}
	if len(__rendered_cmds[`PreCmdString`]) > 0 {
		taskConfig.CmdString = fmt.Sprintf(`%s; %s`, __rendered_cmds[`PreCmdString`], taskConfig.CmdString)
		//pp.Println(`>         pre cmd:         ================================================================>   `, __rendered_cmds[`PreCmdString`])
	}
	if len(__rendered_cmds[`PostCmdString`]) > 0 {
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
