package runtime

import (
	"fmt"
	"os"
	"strings"
)

const CGROUP_LOG_FILE = `/var/log/bashful-cgroups.log`

func cgroup_log(name string, msg string) {
	if false {
		logged_msg := strings.Trim(fmt.Sprintf(`%s`, msg), "\n\r\t")
		log_file_stat, err := os.Stat(CGROUP_LOG_FILE)
		if os.IsNotExist(err) {
			f1, err := os.Create(CGROUP_LOG_FILE)
			if err != nil {
				panic(err)
			} else {
				f1.Close()
			}
		} else {
			fmt.Fprintf(os.Stderr, "Logging %d Bytes to %d Byte Log file %d\n", len(logged_msg), log_file_stat.Size, CGROUP_LOG_FILE)
		}
		f, err := os.OpenFile(CGROUP_LOG_FILE, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}

		defer f.Close()
		fmt.Fprintf(f, "%s", logged_msg)
	}
}
