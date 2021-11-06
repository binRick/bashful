package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	guuid "github.com/gofrs/uuid"
	"github.com/k0kubun/pp"
	"github.com/spf13/cobra"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/runtime"
	"github.com/wagoodman/bashful/utils"
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Info yaml and referenced resources into a single executable (experimental)",
	Long:  `Info yaml and referenced resources into a single executable (experimental)`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		cli := config.Cli{
			YamlPath:             args[0],
			CgroupControllerUUID: guuid.Must(guuid.NewV4()),
		}

		infoPath := filepath.Base(cli.YamlPath[0:len(cli.YamlPath)-len(filepath.Ext(cli.YamlPath))]) + ".info"

		yamlString, err := ioutil.ReadFile(cli.YamlPath)
		utils.CheckError(err, "Unable to read yaml config.")

		fmt.Print("\033[?25l") // hide cursor
		Info(yamlString, infoPath, cli)
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func Info(yamlString []byte, outputPath string, cli config.Cli) {

	yamlString, err := ioutil.ReadFile(cli.YamlPath)
	utils.CheckError(err, "Unable to read yaml Config.")

	client, err := runtime.NewClientFromYaml(yamlString, &cli)
	if err != nil {
		utils.ExitWithErrorMessage(err.Error())
	}

	fmt.Println(utils.Bold("Bundling " + cli.YamlPath + " to " + outputPath))
	pp.Println(client)

	//client.Info(cli.YamlPath, outputPath)

}

func d1() {
	/*
		var max int64 = 1000
		resources := specs.LinuxResources{}
		resources.Pids = &specs.LinuxPids{}
		resources.Pids.Limit = int64(max)
	*/

	/*
		if err != nil {
			fmt.Println(err)
			return
		}
		if control == nil {
			fmt.Println("control is nil")
			return
		}
			if false {
				pp.Println(s)
			}

		}
	*/

}
