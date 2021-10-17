package cmd

import "github.com/wagoodman/bashful/pkg/config"

func get_tags(task_config *config.Config) []string {
	tags := []string{}
	for _, tc := range task_config.TaskConfigs {
		for _, t := range tc.Tags {
			has := false
			for _, _t := range tags {
				if t == _t {
					has = true
				}
			}
			if !has {
				tags = append(tags, t)
			}
		}
		for _, pt := range tc.ParallelTasks {
			for _, t := range pt.Tags {
				has := false
				for _, _t := range tags {
					if t == _t {
						has = true
					}
				}
				if !has {
					tags = append(tags, t)
				}
			}
		}
	}
	return tags
}
