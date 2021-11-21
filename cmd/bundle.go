package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	guuid "github.com/gofrs/uuid"
	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/runtime"
	"github.com/wagoodman/bashful/utils"
)

// bundleCmd represents the bundle command
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle yaml and referenced resources into a single executable (experimental)",
	Long:  `Bundle yaml and referenced resources into a single executable (experimental)`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		cli := config.Cli{
			YamlPath:             args[0],
			CgroupControllerUUID: guuid.Must(guuid.NewV4()),
		}

		bundlePath := filepath.Base(cli.YamlPath[0:len(cli.YamlPath)-len(filepath.Ext(cli.YamlPath))]) + ".bundle"

		yamlString, err := ioutil.ReadFile(cli.YamlPath)
		utils.CheckError(err, "Unable to read yaml config.")

		fmt.Print("\033[?25l") // hide cursor
		Bundle(yamlString, bundlePath, cli)
	},
}

func init() {
	rootCmd.AddCommand(bundleCmd)
}

func Bundle(yamlString []byte, outputPath string, cli config.Cli) {
	started := time.Now()

	yamlString, err := ioutil.ReadFile(cli.YamlPath)
	utils.CheckError(err, "Unable to read yaml Config.")

	client, err := runtime.NewClientFromYaml(yamlString, &cli)
	if err != nil {
		utils.ExitWithErrorMessage(err.Error())
	}

	doneCh := make(chan struct{})
	bar := progressbar.NewOptions(100,
		progressbar.OptionSetWriter(ansi.NewAnsiStderr()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionFullWidth(),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetRenderBlankState(false),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan][1/3][reset] Bundling %s", outputPath)),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			doneCh <- struct{}{}
		}),
	)
	if true {
		go func() {
			for i := 0; i < 20; i++ {
				bar.Add(1)
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	var do_bundle = func() {
		client.Bundle(cli.YamlPath, outputPath)
		bar.Finish()
	}

	go do_bundle()
	<-doneCh
	bar.Clear()
	fmt.Fprintf(os.Stderr, "====== Bundled %s in %s ==========\n\n", outputPath, time.Since(started))

}
