package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/containerd/cgroups"
	v2 "github.com/containerd/cgroups/v2"
	guuid "github.com/gofrs/uuid"
	"github.com/k0kubun/pp"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/wagoodman/bashful/utils"
)

var DEBUG_BF = false
var DEBUG_CG = false
var CGROUPS_MODE = get_cg_mode()
var parent_cgroup, bfcg *v2.Manager
var BASHFUL_CGROUP_NAME = `bashful`
var PARENT_CGROUP_UUID = strings.Split(guuid.Must(guuid.NewV4()).String(), `-`)[0]
var BASHFUL_CGROUP_PATH = fmt.Sprintf(`/%s`, BASHFUL_CGROUP_NAME)
var PARENT_CGROUP_PATH = fmt.Sprintf(`%s/%s`, BASHFUL_CGROUP_PATH, PARENT_CGROUP_UUID)
var GOPS_ENABLED = false
var CG_VER = 0

var swap_max int64 = 2048 * 1000 * 1000
var mem_max int64 = 1024 * 1000 * 1000
var proc_max int64 = 500
var BashfulResources = v2.Resources{
	Pids: &v2.Pids{
		Max: proc_max,
	},
	Memory: &v2.Memory{
		Max:  &mem_max,
		Swap: &swap_max,
	},
	IO: &v2.IO{},
}

const BASE_CG_PATH = `/sys/fs/cgroup`

var shares uint64 = 200
var lim int64 = 20000
var cg_limit1 = &specs.LinuxResources{
	BlockIO: &specs.LinuxBlockIO{},
	CPU: &specs.LinuxCPU{
		Shares: &shares,
	},
	Memory: &specs.LinuxMemory{
		Limit: &lim,
	},
	Pids: &specs.LinuxPids{
		Limit: 1000,
	},
}

func get_cg_mode() string {
	cg_mode := cgroups.Mode()
	switch cg_mode {
	case cgroups.Legacy:
		CG_VER = 1
		return "legacy"
	case cgroups.Hybrid:
		return fmt.Sprintf("hybrid")
	case cgroups.Unified:
		CG_VER = 2
		return fmt.Sprintf("unified")
	case cgroups.Unavailable:
		return fmt.Sprintf("cgroups unavailable")
	}
	return ``
}

func cg_init() {

	os.Setenv(`PARENT_CGROUP_PID`, fmt.Sprintf("%d", syscall.Getpid()))
	os.Setenv(`CGROUPS_MODE`, CGROUPS_MODE)
	os.Setenv(`PARENT_CGROUP_UUID`, PARENT_CGROUP_UUID)
	os.Setenv(`PARENT_CGROUP_PATH`, PARENT_CGROUP_PATH)
	os.Setenv(`CGROUPS_BASE_CG_PATH`, BASE_CG_PATH)
	os.Setenv(`BASHFUL_CGROUP_PATH`, BASHFUL_CGROUP_PATH)

	if false {
		if CG_VER == 2 {
			_bfcg, err := v2.LoadManager(BASE_CG_PATH, BASHFUL_CGROUP_PATH)
			if err != nil {
				_, err := v2.NewManager(BASE_CG_PATH, BASHFUL_CGROUP_PATH, &v2.Resources{})
				if err != nil {
					panic(err)
				}
				_bfcg, err := v2.LoadManager(BASE_CG_PATH, BASHFUL_CGROUP_PATH)
				if err != nil {
					panic(err)
				}
				bfcg = _bfcg
			} else {
				bfcg = _bfcg
			}

			root_controllers, err := bfcg.RootControllers()
			if err != nil {
				panic(err)
			}

			_parent_cgroup, err := v2.NewManager(BASE_CG_PATH, PARENT_CGROUP_PATH, &BashfulResources)
			if err != nil {
				panic(err)
			}
			parent_cgroup = _parent_cgroup
			if err := parent_cgroup.ToggleControllers(root_controllers, v2.Enable); err != nil {
				panic(err)
			}
			if false {
				parent_controllers, err := parent_cgroup.Controllers()
				if err != nil {
					panic(err)
				}
				stats, err := parent_cgroup.Stat()
				if err != nil {
					panic(err)
				}

				_, err = bfcg.Procs(true)
				if err != nil {
					panic(err)
				}

				p_procs, err := parent_cgroup.Procs(true)
				if err != nil {
					panic(err)
				}
				if DEBUG_CG {
					pp.Println(stats)
					fmt.Printf("<ROOT>    %s  %d Root Controllers: %s\n", len(root_controllers), root_controllers)
					fmt.Printf("<PARENT>  %s %d Procs| %d Parent Controllers: %s\n", PARENT_CGROUP_PATH, len(p_procs), len(parent_controllers), parent_controllers)
				}
			}
		}
	}
}

func remove_parent_cgroup() {
	if false {
		PARENT_CGROUP_PATH := os.Getenv(`PARENT_CGROUP_PATH`)
		BASE_CG_PATH := os.Getenv(`CGROUPS_BASE_CG_PATH`)

		cgf := fmt.Sprintf(`%s/%s`, BASE_CG_PATH, PARENT_CGROUP_PATH)
		f_io := fmt.Sprintf(`%s/%s`, cgf, `io.stat`)

		_, err := os.Stat(cgf)
		utils.CheckError(err, "Could not open PARENT_CGROUP_PATH")

		_, err = os.Stat(f_io)
		utils.CheckError(err, "Could not open io.stat")

		io_stat, err := ioutil.ReadFile(f_io)
		utils.CheckError(err, "Could not read io.stat")

		fmt.Fprintf(os.Stdout, `%s:%s`, f_io, pp.Sprintf(`%s`, string(io_stat)))
		//	time.Sleep(10 * time.Minute)

		cg, err := v2.LoadManager(BASE_CG_PATH, PARENT_CGROUP_PATH)
		utils.CheckError(err, "Could not open cgroup")
		derr := cg.Delete()
		utils.CheckError(derr, "Could not delete cgroup")
	}
}
