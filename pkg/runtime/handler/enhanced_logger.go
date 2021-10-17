package handler

import (
	"log"
	"os"
	"strings"

	"github.com/nolleh/caption_json_formatter"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/runtime"
	"github.com/wagoodman/bashful/utils"

	//  logrus "github.com/sirupsen/logrus"
	logrus "github.com/sirupsen/logrus"
)

var (
	logger = NewLogger()
)

// this was just a (successful) experiment :) needs to be reworked
func NewLogger() *logrus.Logger {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)

	logger := logrus.New()
	logger.Level = logrus.TraceLevel
	logger.SetOutput(os.Stderr)

	//	logger.SetFormatter(&caption_json_formatter.Formatter{PrettyPrint: true, CustomCaption: "TEST123"})
	logger.SetFormatter(&logrus.JSONFormatter{})

	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		ForceColors:   true,
		FullTimestamp: true,
	})
	logger.SetReportCaller(false)

	return logger
}

func Log() *caption_json_formatter.Entry {
	//	logrus.SetFormatter(&logrus.JSONFormatter{})
	return &caption_json_formatter.Entry{Entry: logrus.NewEntry(logger)}
}

type EnhancedLogger struct {
	logFile *os.File
	config  *config.Config
}

func NewEnhancedLogger(config *config.Config, log_path string) *EnhancedLogger {
	f, err := os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}
	logger.SetOutput(f)
	return &EnhancedLogger{
		logFile: f,
		config:  config,
	}
}

func (handler *EnhancedLogger) AddRuntimeData(data *runtime.TaskStatistics) {
}

func (handler *EnhancedLogger) Register(task *runtime.Task) {
	//	fmt.Fprintf(os.Stderr, pp.Sprintf(`%s`, task))
	logger.WithFields(logrus.Fields{
		"name": task.Config.Name,
	}).Info(utils.GetColor(`Task`, `Task Start`))
}

func (handler *EnhancedLogger) Unregister(task *runtime.Task) {
	et := `UNKNOWN`
	color := `Unregister`
	bf := logrus.Fields{
		"name": task.Config.Name,
	}

	if task.Command.Cmd.Process != nil && task.Command.Cmd.Process.Pid > 0 {
		et = `Process End`
		bf["pid"] = task.Command.Cmd.Process.Pid
		if task.Command.ReturnCode > -1 {
			bf["exit_code"] = task.Command.ReturnCode
			if task.Command.ReturnCode == 0 {
				color = `UnregisterOK`
			} else {
				color = `UnregisterFailed`
			}
		}
	} else {
		et = `Task Group End`
	}
	if task.Config.CmdString != `` {
		bf["cmd"] = task.Config.CmdString
	}
	logger.WithFields(bf).Info(utils.GetColor(color, et))
}

var FL = true

func (handler *EnhancedLogger) OnEvent(task *runtime.Task, e runtime.TaskEvent) {
	bf := logrus.Fields{
		"name": e.Task.Config.Name,
		"tags": strings.Join(e.Task.Config.Tags, `,`),
	}

	et := `UNKNOWN`
	color := `TaskEnd`
	if e.Complete {
		bf[`exit_code`] = e.ReturnCode
		if e.ReturnCode == 0 {
			color = `TaskEndOK`
			et = `Process Success`
		} else {
			color = `TaskEndFailed`
			et = `Process Fail`
		}
	} else {
		if len(e.Stdout) > 0 || len(e.Stderr) > 0 {
			et = `Process Update`
			color = `TaskUpdate`
		} else {
			et = `Process Start`
			color = `TaskStart`
		}
		if task.Command.Cmd.Process != nil && task.Command.Cmd.Process.Pid > 0 {
			bf["pid"] = task.Command.Cmd.Process.Pid
		}
		bf[`stdout_bytes`] = len(e.Stdout)
		bf[`stderr_bytes`] = len(e.Stderr)
	}

	logger.WithFields(bf).Info(utils.GetColor(color, et))

}

func (handler *EnhancedLogger) Close() {
	handler.logFile.Close()
}
