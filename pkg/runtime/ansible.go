package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	ansible_adhoc "github.com/apenella/go-ansible/pkg/adhoc"
	ansible_options "github.com/apenella/go-ansible/pkg/options"
	guuid "github.com/gofrs/uuid"
	"github.com/wagoodman/bashful/utils"
)

var ad_hoc_tree_dir_prefix = `/tmp/bashful-go-ansible`
var VERBOSE_MODE = (os.Getenv(`VERBOSE_MODE`) == `true`)

var DEFAULT_ENV = map[string]string{
	`ANSIBLE_DEPRECATION_WARNINGS`: `False`, `ANSIBLE_FORCE_COLOR`: `True`, `ANSIBLE_ANY_ERRORS_FATAL`: `True`, `ANSIBLE_DISPLAY_ARGS_TO_STDOUT`: `False`,
}

func NewAdhoc(module_name string, module_args map[string]interface{}, module_hosts []string) *ansible_adhoc.AnsibleAdhocCmd {
	U := guuid.Must(guuid.NewV4())
	tree_path := fmt.Sprintf(`%s/%s/%d/%s.json`, ad_hoc_tree_dir_prefix, module_name, syscall.Getpid(), strings.Split(U.String(), `-`)[0])
	EnsureFileDir(tree_path)
	py3, err := exec.LookPath("python3")
	if err != nil {
		panic(err)
	}

	module_args_encoded, _ := json.Marshal(module_args)
	mhl := fmt.Sprintf(`%s,`, strings.Join(module_hosts, `,`))
	adhoc := &ansible_adhoc.AnsibleAdhocCmd{
		///	Binary:  ap,
		Pattern: mhl,
		ConnectionOptions: &ansible_options.AnsibleConnectionOptions{
			Connection:    "local",
			SSHCommonArgs: "",
			SSHExtraArgs:  "",
			PrivateKey:    "",
			Timeout:       5,
			//			User: "root",
		},
		Options: &ansible_adhoc.AnsibleAdhocOptions{
			ModuleName: module_name,
			Inventory:  mhl,
			Limit:      mhl,
			Tree:       tree_path,
			Verbose:    VERBOSE_MODE,
			OneLine:    true,
		},
		StdoutCallback: "oneline",
	}
	kv := ``
	ap, err := exec.LookPath("ansible")
	if err != nil {
		if strings.Contains(strings.ToLower(fmt.Sprintf(`%s`, err)), `executable file not found in path`) {
			utils.CheckError(err, `Ansible not found in path!`)
		}
		panic(err)
	}
	if false {
		adhoc.Binary = ap
	}
	os.Setenv(`ANSIBLE_PYTHON_INTERPRETER`, `auto_silent`)
	if len(module_hosts) == 1 && module_hosts[0] == `localhost` {
		os.Setenv(`ANSIBLE_PYTHON_INTERPRETER`, py3)
	} else {
		adhoc.ConnectionOptions = &ansible_options.AnsibleConnectionOptions{
			Connection:    "ssh",
			SSHCommonArgs: "",
			SSHExtraArgs:  "",
			PrivateKey:    "",
			Timeout:       5,
			User:          "root",
		}

	}
	if len(module_args) > 0 && string(module_args_encoded) != `null` {
		for kk, vv := range module_args {
			kv = fmt.Sprintf(`%s %s`, kv, fmt.Sprintf(`%s="%s"`, kk, fmt.Sprintf(`%v`, vv)))
		}
	}
	kv = strings.Trim(kv, ` `)
	if len(kv) > 0 {
		adhoc.Options.Args = fmt.Sprintf(`%s`, kv)
	}

	_, hasval := module_args[`val`]
	if hasval {
		adhoc.Options.Args = fmt.Sprintf(`'%s'`, module_args[`val`])
	}
	if VERBOSE_MODE {
		fmt.Fprintf(os.Stderr, "\n\n%s\n\n", fmt.Sprintf(`%s`, adhoc.String()))
	}
	//	fmt.Fprintf(os.Stderr, "\n\n%s\n\n", pp.Sprintf(`%s`, adhoc))
	for k, v := range DEFAULT_ENV {
		os.Setenv(k, v)
	}
	return adhoc
}
