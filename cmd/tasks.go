package cmd

import (
	"fmt"
	"strings"

	"github.com/wagoodman/bashful/pkg/config"
)

func get_tasks(task_config *config.Config) []string {
	tasks := []string{}
	for _, tc := range task_config.TaskConfigs {
		parent_tags := []string{}
		for _, parent_tag := range tc.Tags {
			has := false
			for _, _t := range parent_tags {
				if parent_tag == _t {
					has = true
				}
			}
			if !has {
				parent_tags = append(parent_tags, parent_tag)
			}
		}
		for _, pt := range tc.ParallelTasks {
			child_tags := parent_tags
			for _, child_tag := range pt.Tags {
				has := false
				for _, _t := range child_tags {
					if child_tag == _t {
						has = true
					}
				}
				if !has {
					child_tags = append(child_tags, child_tag)
				}
			}
			qty := ``
			if len(pt.ForEach) > 0 {
				qty = fmt.Sprintf(` (%d Times)`, len(pt.ForEach))
			}
			t := fmt.Sprintf(`[%s] %s%s> %s`, tc.Name, pt.Name, qty, strings.Join(child_tags, ","))
			tasks = append(tasks, t)
		}
	}
	return tasks
}
