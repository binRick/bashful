package cmd

import (
	"fmt"
	"time"

	"github.com/google/gops/agent"
)

var GOPS_ENABLED = false
var CG_VER = 0

func gops_init() {
	if GOPS_ENABLED {
		go func() {
			for {
				if err := agent.Listen(agent.Options{
					ShutdownCleanup: true, // automatically closes on os.Interrupt
				}); err != nil {
					fmt.Errorf(`gops err> %s`, err)
				}
				time.Sleep(time.Hour)
			}
		}()
	}
}
