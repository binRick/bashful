package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/noirbizarre/gonja"
)

func render_cmd_vars_logic(cmd string, vars_list []map[string]string) (rendered_cmd string) {
	return rendered_cmd
}

func render_when(when string, vars_list []map[string]string) (when_result bool, when_error error) {
	rendered_when, err := render_cmd(when, vars_list)
	when_error = err
	exitCode := -1
	syntax_exit_code := -1
	var outbuf, errbuf bytes.Buffer
	var sob, seb bytes.Buffer

	run_cmd := fmt.Sprintf("command test %s", rendered_when)
	syntax_check_cmd := exec.Command("bash", "--noprofile", "--norc", "-n", "-c", run_cmd)
	syntax_check_cmd.Stdout = &sob
	syntax_check_cmd.Stderr = &seb

	bash_cmd := exec.Command("bash", "--noprofile", "--norc", "-c", run_cmd)
	bash_cmd.Stdout = &outbuf
	bash_cmd.Stderr = &errbuf

	syntax_check_cmd_err := syntax_check_cmd.Run()
	_os := outbuf.String()
	_es := errbuf.String()

	if syntax_check_cmd_err != nil {
		if exitError, ok := syntax_check_cmd_err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			syntax_exit_code = ws.ExitStatus()
		} else {
			syntax_exit_code = 1
			if _es == "" {
				_es = syntax_check_cmd_err.Error()
			}
		}
	} else {
		ws := syntax_check_cmd.ProcessState.Sys().(syscall.WaitStatus)
		syntax_exit_code = ws.ExitStatus()
	}

	cmd_err := bash_cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()
	if cmd_err != nil {
		if exitError, ok := cmd_err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			exitCode = 1
			if stderr == "" {
				stderr = cmd_err.Error()
			}
		}
	} else {
		ws := bash_cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}

	when_result = (exitCode == 0 && syntax_exit_code == 0)

	msg := fmt.Sprintf(` **************  WHEN ***************
when:                                %s
vars_list                            %s
rendered_when                        %s
when_result                          %v
when_error                           %s

syntax out %s
syntax err %s
syntax_exit_code %d
exitCode %d

run_cmd												%s

run_cmd out %s
run_cmd err %s

`,
		when,
		vars_list,
		rendered_when,
		when_result,
		when_error,
		_os,
		_es,
		syntax_exit_code,
		exitCode,
		run_cmd,
		stdout,
		stderr,
	)
	if VERBOSE_MODE {
		fmt.Fprintf(os.Stderr, "%s\n", msg)
	}
	return when_result, when_error
}

func render_cmd(cmd string, vars_list []map[string]string) (rendered_cmd string, render_error error) {
	ctx := gonja.Context{}
	for _, vars := range vars_list {
		for ek, ev := range vars {
			ctx[ek] = ev
		}
	}
	tpl, err := gonja.FromString(cmd)
	if err != nil {
		return ``, err
	}
	return tpl.Execute(ctx)
}
