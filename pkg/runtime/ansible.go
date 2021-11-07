package runtime

import (
	"context"
	"fmt"

	"github.com/apenella/go-ansible/pkg/adhoc"
	"github.com/apenella/go-ansible/pkg/options"
)

func init() {
	ap()
}
func ap() {
	ansibleConnectionOptions := &options.AnsibleConnectionOptions{
		Connection: "local",
	}

	ansibleAdhocOptions := &adhoc.AnsibleAdhocOptions{
		Inventory:  "localhost,",
		ModuleName: "ping",
	}

	adhoc := &adhoc.AnsibleAdhocCmd{
		Pattern:           "all",
		Options:           ansibleAdhocOptions,
		ConnectionOptions: ansibleConnectionOptions,
		StdoutCallback:    "yaml",
	}

	fmt.Println(adhoc.String())

	err := adhoc.Run(context.TODO())
	if err != nil {
		panic(err)
	}
}
