package cmd

import (
	"os"

	v2 "github.com/containerd/cgroups/v2"
	"github.com/wagoodman/bashful/utils"
)

func remove_parent_cgroup() {

	PARENT_CGROUP_PATH := os.Getenv(`PARENT_CGROUP_PATH`)
	cg, err := v2.LoadManager(BASE_CG_PATH, PARENT_CGROUP_PATH)
	utils.CheckError(err, "Could not open cgroup")
	derr := cg.Delete()
	utils.CheckError(derr, "Could not delete cgroup")

}
