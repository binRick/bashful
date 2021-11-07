package handler

import (
	"syscall"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/k0kubun/pp"
	gopsutil_net "github.com/shirou/gopsutil/v3/net"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	color "github.com/mgutz/ansi"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/tj/go-spin"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/runtime"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/jotframe"
	terminaldimensions "github.com/wayneashleyberry/terminal-dimensions"
)

var DEBUG_BF = false

type VerticalUI struct {
	lock        sync.Mutex
	config      *config.Config
	data        map[uuid.UUID]*display
	spinner     *spin.Spinner
	ticker      *time.Ticker
	startTime   time.Time
	runtimeData *runtime.TaskStatistics
	frame       *jotframe.FixedFrame
}

// display represents all non-Config items that control how the task line should be printed to the screen
type display struct {
	Task *runtime.Task

	// Template is the single-line string template that should be used to display the TaskStatus of a single task
	Template *template.Template

	// Index is the row within a screen frame to print the task template
	Index int

	// Values holds all template values that represent the task TaskStatus
	Values lineInfo

	line *jotframe.Line
}

var VERBOSE_MODE = (os.Getenv(`__BASHFUL_VERBOSE_MODE`) == `true`)

type summary struct {
	Status  string
	Percent string
	Msg     string
	Runtime string
	Eta     string
	Split   string
	Steps   string
	Errors  string
}

// lineInfo represents all template values that represent the task status
type lineInfo struct {
	// Status is the current pending/running/error/success status of the command
	Status string

	// Title is the display name to use for the task
	Title string

	// Msg may show any arbitrary string to the screen (such as stdout or stderr values)
	Msg string

	// Prefix is used to place the spinner or bullet characters before the title
	Prefix string

	// Eta is the displayed estimated time to completion based on the current time
	Eta string

	// Split can be used to "float" values to the right hand side of the screen when printing a single line
	Split string
}

var (
	summaryTemplate, _ = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)

	// lineDefaultTemplate is the string template used to display the TaskStatus values of a single task with no children
	lineDefaultTemplate, _ = template.New("default line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)

	// lineParallelTemplate is the string template used to display the TaskStatus values of a task that is the child of another task
	lineParallelTemplate, _ = template.New("parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} ├─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)

	// lineLastParallelTemplate is the string template used to display the TaskStatus values of a task that is the LAST child of another task
	lineLastParallelTemplate, _ = template.New("last parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} └─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
)

func NewVerticalUI(cfg *config.Config) *VerticalUI {

	updateInterval := 150 * time.Millisecond
	if cfg.Options.UpdateInterval > 150 {
		updateInterval = time.Duration(cfg.Options.UpdateInterval) * time.Millisecond
	}

	handler := &VerticalUI{
		data:      make(map[uuid.UUID]*display, 0),
		spinner:   spin.New(),
		ticker:    time.NewTicker(updateInterval),
		startTime: time.Now(),
		config:    cfg,
	}

	go handler.spinnerHandler()

	return handler
}

func (handler *VerticalUI) AddRuntimeData(data *runtime.TaskStatistics) {
	handler.runtimeData = data
}

func load_env(task *runtime.Task) {
	PARENT_CGROUP_PID := os.Getenv(`PARENT_CGROUP_PID`)
	PARENT_CGROUP_UUID := os.Getenv(`PARENT_CGROUP_UUID`)
	PARENT_CGROUP_PATH := os.Getenv(`PARENT_CGROUP_PATH`)
	if DEBUG_BF {
		fmt.Fprintf(os.Stderr, "NEW TASK>> tags=%s children_qty=%d envs=%d parallel_tasks_qty=%d cmd=%s task=%s path=%s uuid=%s pid=%d parent_pid=%s task_config_parent_uuid=%s args=\"%s\"\n",
			strings.Join(task.Config.Tags, `,`),
			len(task.Children),
			len(task.Config.Env),
			len(task.Config.ParallelTasks),
			task.Command.Cmd.Path,
			task.Id,
			PARENT_CGROUP_PATH, PARENT_CGROUP_UUID, syscall.Getpid(), PARENT_CGROUP_PID,
			task.Config.BCG.ParentUUID.String(),
			strings.Join(task.Command.Cmd.Args, ` `),
		)
	}
}

func (handler *VerticalUI) spinnerHandler() {

	for {
		select {

		case <-handler.ticker.C:
			handler.lock.Lock()

			handler.spinner.Next()
			for _, displayData := range handler.data {
				task := displayData.Task

				if task.Config.CmdString != "" {
					if !task.Completed && task.Started {
						displayData.Values.Prefix = handler.spinner.Current()
						displayData.Values.Eta = handler.CurrentEta(task)
					}
					handler.displayTask(task)
				}

				for _, subTask := range task.Children {
					childDisplayData := handler.data[subTask.Id]
					if !subTask.Completed && subTask.Started {
						childDisplayData.Values.Prefix = handler.spinner.Current()
						childDisplayData.Values.Eta = handler.CurrentEta(subTask)
					}
					handler.displayTask(subTask)
				}

				// update the summary line
				if handler.config.Options.ShowSummaryFooter {
					renderedFooter := handler.footer(runtime.StatusPending, "")
					io.WriteString(handler.frame.Footer(), renderedFooter)
				}
			}
			handler.lock.Unlock()
		}

	}
}

// todo: move footer logic based on jotframe requirements
func (handler *VerticalUI) Close() {
	// todo: remove config references
	if handler.config.Options.ShowSummaryFooter {
		// todo: add footer update via Executor stats
		message := ""

		handler.frame.Footer().Open()
		if len(handler.runtimeData.Failed) > 0 {
			if handler.config.Options.LogPath != "" {
				message = utils.Bold(" See log for details (" + handler.config.Options.LogPath + ")")
			}

			renderedFooter := handler.footer(runtime.StatusError, message)
			handler.frame.Footer().WriteStringAndClose(renderedFooter)
		} else {
			renderedFooter := handler.footer(runtime.StatusSuccess, message)
			handler.frame.Footer().WriteStringAndClose(renderedFooter)
		}
		handler.frame.Footer().Close()
	}
	handler.frame.Close()
}

func (handler *VerticalUI) Unregister(task *runtime.Task) {
	if _, ok := handler.data[task.Id]; !ok {
		// ignore data that have already been unregistered
		return
	}
	handler.lock.Lock()
	defer handler.lock.Unlock()

	if len(task.Children) > 0 {

		displayData := handler.data[task.Id]

		hasHeader := len(task.Children) > 0
		collapseSection := task.Config.CollapseOnCompletion && hasHeader && task.FailedChildren == 0
		if VERBOSE_MODE {

			collapseSection = false
		}

		// complete the proc group TaskStatus
		if hasHeader {
			handler.frame.Header().Open()
			var message bytes.Buffer
			collapseSummary := ""
			if collapseSection {
				collapseSummary = utils.Purple(" (" + strconv.Itoa(len(task.Children)) + " Tasks hidden)")
			}
			displayData.Template.Execute(&message, lineInfo{Status: handler.TaskStatusColor(task.Status, "i"), Title: task.Config.Name + collapseSummary, Prefix: handler.config.Options.BulletChar})

			handler.frame.Header().WriteStringAndClose(message.String())
		}

		// collapse sections or parallel Tasks...
		if collapseSection {
			// todo: enhance jotframe to take care of this
			length := len(handler.frame.Lines())
			for idx := 0; idx < length; idx++ {
				handler.frame.Remove(handler.frame.Lines()[0])
			}
		}
	}

	delete(handler.data, task.Id)
}

func (handler *VerticalUI) doRegister(task *runtime.Task) {
	if _, ok := handler.data[task.Id]; ok {
		// ignore data that have already been registered
		return
	}

	hasParentCmd := task.Config.CmdString != ""
	hasHeader := len(task.Children) > 0
	numTasks := len(task.Children)
	if hasParentCmd {
		numTasks++
	}

	// we should overwrite the footer of the last frame when creating a new frame (kinda hacky... todo: replace this)
	isFirst := handler.frame == nil
	if handler.frame != nil {
		handler.frame.Close()
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panic occurred:", err)
		}
	}()
	handler.frame = jotframe.NewFixedFrame(0, hasHeader, handler.config.Options.ShowSummaryFooter, false)
	if !isFirst && handler.config.Options.ShowSummaryFooter {
		handler.frame.Move(-1)
	}

	var line *jotframe.Line
	if hasParentCmd {
		line, _ = handler.frame.Append()
		// todo: check err
	}

	handler.data[task.Id] = &display{
		Template: lineDefaultTemplate,
		Index:    0,
		Task:     task,
		line:     line,
	}
	for idx, subTask := range task.Children {
		line, _ := handler.frame.Append()
		// todo: check err
		handler.data[subTask.Id] = &display{
			Template: lineParallelTemplate,
			Index:    idx + 1,
			Task:     subTask,
			line:     line,
		}
		if idx == len(task.Children)-1 {
			handler.data[subTask.Id].Template = lineLastParallelTemplate
		}
	}

	displayData := handler.data[task.Id]

	// initialize each line in the frame
	if hasHeader {
		var message bytes.Buffer
		lineObj := lineInfo{Status: handler.TaskStatusColor(runtime.StatusRunning, "i"), Title: task.Config.Name, Msg: "", Prefix: handler.config.Options.BulletChar}
		displayData.Template.Execute(&message, lineObj)

		io.WriteString(handler.frame.Header(), message.String())
	}

	if hasParentCmd {
		displayData.Values = lineInfo{Status: handler.TaskStatusColor(runtime.StatusPending, "i"), Title: task.Config.Name}
		handler.displayTask(task)
	}

	for line := 0; line < len(task.Children); line++ {
		childDisplayData := handler.data[task.Children[line].Id]
		childDisplayData.Values = lineInfo{Status: handler.TaskStatusColor(runtime.StatusPending, "i"), Title: task.Children[line].Config.Name}
		handler.displayTask(task.Children[line])
	}
}

func (handler *VerticalUI) Register(task *runtime.Task) {
	handler.lock.Lock()
	load_env(task)
	defer handler.lock.Unlock()
	handler.doRegister(task)
}

func (handler *VerticalUI) OnEvent(task *runtime.Task, e runtime.TaskEvent) {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	eventTask := e.Task

	if !eventTask.Config.ShowTaskOutput {
		e.Stderr = ""
		e.Stdout = ""
	}
	eventDisplayData := handler.data[e.Task.Id]

	if e.Stderr != "" {
		eventDisplayData.Values = lineInfo{
			Status: handler.TaskStatusColor(e.Status, "i"),
			Title:  eventTask.Config.Name,
			Msg:    e.Stderr,
			Prefix: handler.spinner.Current(),
			Eta:    handler.CurrentEta(eventTask),
		}
	} else {
		//pp.Println(e, handler, eventTask)
		if VERBOSE_MODE {
			pp.Println(e.Status)
			pp.Println(eventTask.Config.Name)
			pp.Println(e.Stdout)
			pp.Println(handler.spinner.Current())
			pp.Println(handler.CurrentEta(eventTask))
		}
		eventDisplayData.Values = lineInfo{
			Status: handler.TaskStatusColor(e.Status, "i"),
			Title:  eventTask.Config.Name,
			Msg:    e.Stdout,
			Prefix: handler.spinner.Current(),
			Eta:    handler.CurrentEta(eventTask),
		}
	}

	handler.displayTask(eventTask)

	// update the summary line
	if handler.config.Options.ShowSummaryFooter {
		renderedFooter := handler.footer(runtime.StatusPending, "")
		handler.frame.Footer().WriteString(renderedFooter)
	}
}

const DIMENSIONS_MIN_DURATION = time.Duration(100 * time.Millisecond)

var last_dimensions_check_ts int64 = 0
var last_server_hostname_check int64 = 0
var cached_terminalWidth uint = 0
var cached_server_hostname string = ""
var last_host_info_check int64 = 0
var cached_host_info = host.InfoStat{}
var last_mem_check int64 = 0
var cached_mem = mem.VirtualMemoryStat{}
var cached_misc = load.MiscStat{}
var cached_io = map[string]disk.IOCountersStat{}
var cached_usage = disk.UsageStat{}
var cached_conns = []gopsutil_net.ConnectionStat{}
var cached_conn_stats = []gopsutil_net.ProtoCountersStat{}
var cached_ints = gopsutil_net.InterfaceStatList{}
var cur_estab_qty int64 = 0
var created_since_started int64 = 0
var created_at_start int64 = 0
var starting_io_sums = map[string]int64{
	`reads`:       0,
	`writes`:      0,
	`read_bytes`:  0,
	`write_bytes`: 0,
}
var cached_io_sums = map[string]int64{
	`reads`:       0,
	`writes`:      0,
	`read_bytes`:  0,
	`write_bytes`: 0,
}
var first_run = true
var cg_limit1 = &specs.LinuxResources{
	BlockIO: &specs.LinuxBlockIO{},
	CPU:     &specs.LinuxCPU{},
	Memory:  &specs.LinuxMemory{},
	Pids: &specs.LinuxPids{
		Limit: 1000,
	},
}

func (handler *VerticalUI) displayTask(task *runtime.Task) {
	now := time.Now()

	// todo: error handling
	if _, ok := handler.data[task.Id]; !ok {
		return
	}

	duration3, err := time.ParseDuration("-100ms")
	if err == nil {
		then3 := now.Add(duration3)
		if last_mem_check < then3.UnixNano() {
			last_host_info_check = now.UnixNano()
			_host_info, err := host.Info()
			if err == nil {
				cached_host_info = *_host_info
			}
			_misc, err := load.Misc()
			if err == nil {
				cached_misc = *_misc
				if created_at_start == 0 {
					created_at_start = int64(cached_misc.ProcsCreated)
				}
				created_since_started = int64(cached_misc.ProcsCreated) - created_at_start
			}
			_io, err := disk.IOCounters()
			if err == nil {
				cached_io = _io
				cached_io_sums["write_bytes"] = 0
				cached_io_sums["read_bytes"] = 0
				cached_io_sums["writes"] = 0
				cached_io_sums["reads"] = 0
				for io_n, kv := range cached_io {
					if false {
						pp.Println(io_n, kv)
					}
					if first_run {
						starting_io_sums["write_bytes"] += int64(kv.WriteBytes)
						starting_io_sums["read_bytes"] += int64(kv.ReadBytes)
						starting_io_sums["reads"] += int64(kv.ReadCount)
						starting_io_sums["writes"] += int64(kv.WriteCount)
					} else {
						cached_io_sums["write_bytes"] += int64(kv.WriteBytes) - starting_io_sums["write_bytes"]
						cached_io_sums["read_bytes"] += int64(kv.ReadBytes) - starting_io_sums["read_bytes"]
						cached_io_sums["writes"] += int64(kv.WriteCount) - starting_io_sums["writes"]
						cached_io_sums["reads"] += int64(kv.ReadCount) - starting_io_sums["reads"]
					}
				}
				if first_run {
					first_run = false
				}
			}
			_ints, err := gopsutil_net.Interfaces()
			if err == nil {
				cached_ints = _ints
			}

			_conns, err := gopsutil_net.Connections(`inet`)
			if err == nil {
				cached_conns = _conns
			}
			_conn_stats, err := gopsutil_net.ProtoCounters([]string{})
			if err == nil {
				cached_conn_stats = _conn_stats
				for _, cs := range cached_conn_stats {
					if cs.Protocol == `tcp` {
						cur_estab_qty = cs.Stats["CurrEstab"]
					}
				}
			}
			_usage, err := disk.Usage("/")
			if err == nil {
				cached_usage = *_usage
			}
		}
	}

	duration2, err := time.ParseDuration("-100ms")
	if err == nil {
		then2 := now.Add(duration2)
		if last_mem_check < then2.UnixNano() {
			_mem, err := mem.VirtualMemory()
			if err == nil {
				cached_mem = *_mem
				last_mem_check = now.UnixNano()
			}
		}
	}
	duration1, err := time.ParseDuration("-5000ms")
	if err == nil {
		then1 := time.Now().Add(duration1)
		if last_server_hostname_check < then1.UnixNano() {
			_server_hostname, err := os.Hostname()
			if err == nil {
				cached_server_hostname = _server_hostname
				last_server_hostname_check = now.UnixNano()
			}
		}
	}
	duration, err := time.ParseDuration("-1000ms")
	if err == nil {
		then := now.Add(duration)
		if last_dimensions_check_ts < then.UnixNano() {
			_cached_terminalWidth, err := terminaldimensions.Width()
			if err == nil {
				cached_terminalWidth = _cached_terminalWidth
				last_dimensions_check_ts = now.UnixNano()
			}
		}
	}
	if cached_terminalWidth > 0 {
		displayData := handler.data[task.Id]
		renderedLine := handler.renderTask(task, int(cached_terminalWidth))
		io.WriteString(displayData.line, renderedLine)
	}
}

func (handler *VerticalUI) footer(status runtime.TaskStatus, message string) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string
	if handler.config.Options.ShowSummaryTimes {
		duration := time.Since(handler.startTime)
		hn := fmt.Sprintf(`%s`, cached_server_hostname)
		usage_str := fmt.Sprintf("/ %d%% Used",
			uint(cached_usage.UsedPercent),
		)
		if uint(cached_usage.UsedPercent) < 10 {
			usage_str = ``
		}
		procs_str := fmt.Sprintf("%d/%d Procs, %d Blocked, %d Running, %d Created",
			cached_misc.ProcsTotal,
			cached_host_info.Procs,
			cached_misc.ProcsBlocked,
			cached_misc.ProcsRunning,
			created_since_started,
		)
		conns_str := fmt.Sprintf("%d Conns, %d TCPs",
			len(cached_conns),
			cur_estab_qty,
		)
		io_str := fmt.Sprintf("%d/%sr, %d/%sw",
			cached_io_sums["reads"]-starting_io_sums["reads"],
			humanize.Bytes(uint64(cached_io_sums["read_bytes"]-starting_io_sums["read_bytes"])),
			cached_io_sums["writes"]-starting_io_sums["writes"],
			humanize.Bytes(uint64(cached_io_sums["write_bytes"]-starting_io_sums["write_bytes"])),
		)
		mem_str := fmt.Sprintf("%s/%s Free",
			humanize.Bytes(uint64(cached_mem.Free)),
			humanize.Bytes(uint64(cached_mem.Total)),
		)
		if uint(cached_mem.UsedPercent) < 5 {
			mem_str = ``
		}
		if strings.Contains(hn, `.`) {
			hn = strings.Split(hn, `.`)[0]
		}
		io_str = ``
		durString = fmt.Sprintf("[%s]|%s|%s|%s|%s|Mem:%s|[%s] Runtime",
			hn,
			io_str,
			conns_str,
			usage_str,
			procs_str,
			mem_str,
			utils.FormatDuration(duration),
		)

		terminalWidth, _ := terminaldimensions.Width()
		maxMessageWidth := uint(terminalWidth) - uint(utils.VisualLength(durString))
		if uint(utils.VisualLength(durString)) > maxMessageWidth-3 {
			durString = utils.TrimToVisualLength(durString, int(maxMessageWidth-3)) + "..."
		}

		totalEta := time.Duration(handler.config.TotalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", utils.FormatDuration(remainingEta))
		if strings.Contains(utils.FormatDuration(remainingEta), "Unknown") {
			etaString = ``
		}
		if strings.Contains(utils.FormatDuration(remainingEta), "Overdue") {
			etaString = ``
		}
	}

	if len(handler.runtimeData.Completed) == handler.runtimeData.Total {
		etaString = ""
	}

	if handler.config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", len(handler.runtimeData.Completed), handler.runtimeData.Total)
	}

	if handler.config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", len(handler.runtimeData.Failed))
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(len(handler.runtimeData.Completed)) * float64(100)) / float64(handler.runtimeData.Total)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)
	percentStr = color.Color(percentStr, "default+b")

	summaryTemplate.Execute(&tpl, summary{Status: handler.TaskStatusColor(status, "i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString, Msg: message})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminaldimensions.Width()
	splitWidth := int(terminalWidth) - utils.VisualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, summary{Status: handler.TaskStatusColor(status, "i"), Percent: percentStr, Runtime: utils.Bold(durString), Eta: utils.Bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: utils.Bold(stepString), Errors: utils.Bold(errorString), Msg: message})

	return tpl.String()
}

// String represents the task status and command output in a single line
func (handler *VerticalUI) renderTask(task *runtime.Task, terminalWidth int) string {
	displayData := handler.data[task.Id]

	if task.Completed {
		displayData.Values.Eta = ""
		if task.Command.ReturnCode != 0 && !task.Config.IgnoreFailure {
			displayData.Values.Msg = utils.Red("Exited with error (" + strconv.Itoa(task.Command.ReturnCode) + ")")
		}
	}

	// set the name
	if task.Config.Name == "" {
		if len(task.Config.CmdString) > 25 {
			task.Config.Name = task.Config.CmdString[:22] + "..."
		} else {
			task.Config.Name = task.Config.CmdString
		}
	}

	// display
	var message bytes.Buffer

	// get a string with the summary line without a split gap or message
	displayData.Values.Split = ""
	originalMessage := displayData.Values.Msg
	displayData.Values.Msg = ""
	displayData.Template.Execute(&message, displayData.Values)

	// calculate the max width of the message and trim it
	maxMessageWidth := terminalWidth - utils.VisualLength(message.String())
	displayData.Values.Msg = originalMessage
	if utils.VisualLength(displayData.Values.Msg) > maxMessageWidth {
		displayData.Values.Msg = utils.TrimToVisualLength(displayData.Values.Msg, maxMessageWidth-3) + "..."
	}

	// calculate a space buffer to push the eta to the right
	message.Reset()
	displayData.Template.Execute(&message, displayData.Values)
	splitWidth := terminalWidth - utils.VisualLength(message.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	message.Reset()

	// override the current spinner to empty or a handler.config.Options.BulletChar
	if (!task.Started || task.Completed) && len(task.Children) == 0 && displayData.Template == lineDefaultTemplate {
		displayData.Values.Prefix = handler.config.Options.BulletChar
	} else if task.Completed {
		displayData.Values.Prefix = ""
	}

	displayData.Values.Split = strings.Repeat(" ", splitWidth)
	displayData.Template.Execute(&message, displayData.Values)

	return message.String()
}

// TaskStatusColor returns the ansi color value represented by the given TaskStatus
func (handler *VerticalUI) TaskStatusColor(status runtime.TaskStatus, attributes string) string {
	switch status {
	case runtime.StatusRunning:
		return color.ColorCode(strconv.Itoa(handler.config.Options.ColorRunning) + "+" + attributes)

	case runtime.StatusPending:
		return color.ColorCode(strconv.Itoa(handler.config.Options.ColorPending) + "+" + attributes)

	case runtime.StatusSuccess:
		return color.ColorCode(strconv.Itoa(handler.config.Options.ColorSuccess) + "+" + attributes)

	case runtime.StatusError:
		return color.ColorCode(strconv.Itoa(handler.config.Options.ColorError) + "+" + attributes)

	}
	return "INVALID COMMAND STATUS"
}

// CurrentEta returns a formatted string indicating a countdown until command completion
func (handler *VerticalUI) CurrentEta(task *runtime.Task) string {
	var eta, etaValue string

	if task.Options.ShowTaskEta {
		running := time.Since(task.Command.StartTime)
		etaValue = "Unknown!"
		if task.Command.EstimatedRuntime > 0 {
			etaValue = utils.FormatDuration(time.Duration(task.Command.EstimatedRuntime.Seconds()-running.Seconds()) * time.Second)
		}
		eta = fmt.Sprintf(utils.Bold("[%s]"), etaValue)
	}
	return eta
}
