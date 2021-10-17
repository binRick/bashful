package cmd

func d1() {
	/*
		var max int64 = 1000
		resources := specs.LinuxResources{}
		resources.Pids = &specs.LinuxPids{}
		resources.Pids.Limit = int64(max)
	*/
	//control, err := cgroups.New(cgroups.Systemd, cgroups.Slice("test.slice", "chronyd1"), &resources)
	///////control, err := cgroups.New(cgroups.Systemd, cgroups.StaticPath("/sys/fs/cgroup/foo"), &resources)

	//	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath("test"), &specs.LinuxResources{})
	/*
		if err != nil {
			fmt.Println(err)
			return
		}
		if control == nil {
			fmt.Println("control is nil")
			return
		}
		for _, s := range cgroups.Subsystems() {
			if false {
				pp.Println(s)
			}

		}
	*/

}
