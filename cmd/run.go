package cmd

import (
	"fmt"
	"os"

	mapset "github.com/deckarep/golang-set"
	"github.com/shirou/gopsutil/process"

	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/log"
	"github.com/wagoodman/bashful/pkg/runtime"
	"github.com/wagoodman/bashful/pkg/runtime/handler"
	"github.com/wagoodman/bashful/utils"
)

var DEBUG_MODE = (os.Getenv(`DEBUG_MODE`) == `1`)

// todo: put these in a cli struct instance instead, then most logic can be in the cli struct
var tags, onlyTags, skipTags string
var verboseMode bool
var listTagsMode bool
var devMode bool
var execHostname string
var DEFAULT_EVENTS_LOG_FILE = `/var/log/bashful-events.log`
var statsMode bool
var statsModeDefault bool = false
var cgroupsMode bool
var listTasksMode bool
var eventLogMode bool
var dryRunMode bool
var found_pids = []int32{}
var eventLogFile string

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the given yaml file with bashful",
	Long:  `Execute the given yaml file with bashful`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if verboseMode {
			os.Setenv(`__BASHFUL_VERBOSE_MODE`, `true`)
		}
		os.Setenv(`__BASHFUL_EXEC_HOSTNAME`, execHostname)
		if tags != "" && onlyTags != "" {
			utils.ExitWithErrorMessage("Options 'tags' and 'only-tags' are mutually exclusive.")
		}
		cli := config.Cli{
			YamlPath: args[0],
			BashfulCgroup: config.BashfulCgroup{
				CgroupIDs: []string{},
			},
		}

		if len(args) > 1 {
			cli.Args = args[1:]
		} else {
			cli.Args = []string{}
		}

		for _, value := range strings.Split(tags, ",") {
			if value != "" {
				cli.RunTags = append(cli.RunTags, value)
			}
		}

		for _, value := range strings.Split(onlyTags, ",") {
			if value != "" {
				cli.ExecuteOnlyMatchedTags = true
				cli.RunTags = append(cli.RunTags, value)
			}
		}

		nl := []string{}
		for _, tt := range cli.RunTags {
			do_skip := false
			for _, value := range strings.Split(skipTags, ",") {
				if tt == value {
					do_skip = true
				}
				if !do_skip {
					has := false
					for _, nk := range nl {
						if nk == tt {
							has = true
						}
					}
					if !has {
						nl = append(nl, tt)
					}
				}
			}
		}

		//		pp.Println(`running tags: `, len(cli.RunTags), `=>`, len(nl))
		cli.RunTags = nl

		// todo: make this a function for CLI (addTag or something)
		cli.RunTagSet = mapset.NewSet()
		for _, tag := range cli.RunTags {
			cli.RunTagSet.Add(tag)
		}

		yamlString, err := ioutil.ReadFile(cli.YamlPath)
		utils.CheckError(err, "Unable to read yaml config.")

		fmt.Print("\033[?25l")       // hide cursor
		defer fmt.Print("\033[?25h") // show cursor
		Run(yamlString, cli)

	},
}

func init() {
	gops_init()
	rootCmd.AddCommand(runCmd)

	//  RUN OPTIONS
	runCmd.Flags().BoolVarP(&cgroupsMode, "cgroups", "c", false, "Cgroups Mode")
	runCmd.Flags().BoolVarP(&devMode, "dev-mode", "d", false, "Dev Mode")
	runCmd.Flags().BoolVarP(&statsMode, "stats-mode", "s", statsModeDefault, "Stats Mode")
	runCmd.Flags().BoolVarP(&dryRunMode, "dry-run", "n", false, "Dry Run")
	runCmd.Flags().BoolVarP(&eventLogMode, "log-events", "E", false, "Log Events")
	runCmd.Flags().StringVarP(&eventLogFile, "log-events-file", "F", DEFAULT_EVENTS_LOG_FILE, "Log Events File")

	//  MODES
	runCmd.Flags().BoolVarP(&listTagsMode, "list-tags", "T", false, "List Tags")
	runCmd.Flags().BoolVarP(&listTasksMode, "list-tasks", "l", false, "List Tasks")

	//  TAGS
	runCmd.Flags().StringVar(&tags, "untagged-and-tags", "", "A comma delimited list of matching task tags. If a task's tag matches *or if it is not tagged* then it will be executed (also see --only-tags)")
	runCmd.Flags().StringVarP(&onlyTags, "only-tags", "t", "", "A comma delimited list of matching task tags. A task will only be executed if it has a matching tag")
	runCmd.Flags().StringVarP(&skipTags, "skip-tags", "x", "", "A comma delimited list of task tags to skip. A task will only be executed if it does not have a matching tag")
	runCmd.Flags().BoolVarP(&verboseMode, "verbose", "v", false, "Verbose Mode")
	runCmd.Flags().StringVarP(&execHostname, "host", "H", `localhost`, "Hostname to apply")
}

func Run(yamlString []byte, cli config.Cli) {
	client, err := runtime.NewClientFromYaml(yamlString, &cli)
	if err != nil {
		utils.ExitWithErrorMessage(err.Error())
	}

	if devMode {
		fmt.Fprintf(os.Stdout, "%s\n", "dev mode")
		//d1()
		os.Exit(0)
	}
	if listTagsMode {
		tags := get_tags(client.Config)
		fmt.Fprintf(os.Stdout, "%s\n", strings.Join(tags, "\n"))
		os.Exit(0)
	}
	if listTasksMode {
		tags := get_tasks(client.Config)
		fmt.Fprintf(os.Stdout, "%s\n", strings.Join(tags, "\n"))
		os.Exit(0)
	}
	sm := os.Getenv(`STATS_MODE`)
	if !statsMode {
		if sm == `1` || sm == `true` || sm == `yes` {
			statsMode = true
		}
	}
	if statsMode {
		fmt.Fprintf(os.Stderr, "%s\n", `Running in Stats Mode!`)
	}

	if client.Config.Options.SingleLineDisplay {
		client.AddEventHandler(handler.NewCompressedUI(client.Config))
	} else {
		client.AddEventHandler(handler.NewVerticalUI(client.Config))
	}
	client.AddEventHandler(handler.NewTaskLogger(client.Config))
	//client.AddEventHandler(handler.NewSimpleLogger(client.Config))
	if eventLogMode {
		client.AddEventHandler(handler.NewEnhancedLogger(client.Config, eventLogFile))
	}

	if false {
		go func() {
			for {
				pids, err := process.Pids()
				if err == nil {
					for _, pid := range pids {
						has := false
						for _, p := range found_pids {
							if p == pid {
								has = true
							}
						}
						if !has {
							found_pids = append(found_pids, pid)
							//						panic(err)
							//				}
						}
						if false {
						}
						fmt.Fprintf(os.Stderr, "ADDING pid> %s to %d PIDs\n", pid, len(found_pids))
						//						//					panic(err)
						//				}
					}
				}
				time.Sleep(3 * time.Second)
			}
		}()
	}
	rand.Seed(time.Now().UnixNano())

	tagInfo := ""
	if len(cli.RunTags) > 0 {
		if cli.ExecuteOnlyMatchedTags {
			tagInfo = " only matching tags: "
		} else {
			tagInfo = " non-tagged and matching tags: "
		}
		tagInfo += strings.Join(cli.RunTags, ",")
	}

	fmt.Println(utils.Bold("Running " + tagInfo))
	log.LogToMain("Running "+tagInfo, log.StyleMajor)

	failedTasksErr := client.Run()
	log.LogToMain("Complete", log.StyleMajor)

	log.LogToMain("Exiting", "")

	//remove_parent_cgroup()

	if failedTasksErr != nil {
		utils.Exit(1)
	}
}
