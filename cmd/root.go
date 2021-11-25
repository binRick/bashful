package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wagoodman/bashful/pkg/runtime"
)

var cachePath string
var bashBinary string
var bashLoadables bool
var statsMode bool
var statsModeDefault = false

var rootCmd = &cobra.Command{
	Use:   "bashful",
	Short: "Takes a yaml file containing commands and bash snippits and executes each command while showing a simple (vertical) progress bar",
	Long:  `Takes a yaml file containing commands and bash snippits and executes each command while showing a simple (vertical) progress bar`,
}

func Execute() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initBashful)
	rootCmd.PersistentFlags().StringVar(&cachePath, "cache-path", "", "The path where cached files will be stored. By default '$(pwd)/.bashful' is used")
	rootCmd.PersistentFlags().BoolVar(&bashLoadables, "bash-loadables", false, "Enable Bash Loadables")
	rootCmd.PersistentFlags().StringVar(&bashBinary, "bash-binary", `/bin/bash`, "Bash Binary")
	//	rootCmd.PersistentFlags().BoolVar(&statsMode, "stats", false, "Enable Stats Mode")
	rootCmd.PersistentFlags().BoolVarP(&statsMode, "stats-mode", "s", statsModeDefault, "Stats Mode")
}

func initBashful() {
	runtime.Setup()
}
