package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	v2 "github.com/containerd/cgroups/v2"
	"github.com/k0kubun/pp"
	"github.com/wagoodman/bashful/utils"
)

func remove_parent_cgroup() {
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
	pp.Println(io_stat, f_io)

	time.Sleep(10 * time.Minute)

	cg, err := v2.LoadManager(BASE_CG_PATH, PARENT_CGROUP_PATH)
	utils.CheckError(err, "Could not open cgroup")
	derr := cg.Delete()
	utils.CheckError(derr, "Could not delete cgroup")
}
