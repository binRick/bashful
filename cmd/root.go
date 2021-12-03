package cmd

import (
	"fmt"
	"os"

	color "github.com/wayneashleyberry/truecolor/pkg/color"

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

	color.Color(186, 218, 85).Println("Hello, World!")
	color.Black().Background(186, 218, 85).Println("Hello, World!")
	color.White().Underline().Print("Hello, World!\n")
	color.White().Dim().Println("Hello, World!")
	color.White().Italic().Println("Hello, World!")
	color.White().Bold().Println("Hello, World!")
	color.Color(255, 165, 00).Printf("Hello, %s!\n", "World")

}

func initBashful() {
	runtime.Setup()
}
