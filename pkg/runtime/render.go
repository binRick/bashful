package runtime

import "github.com/noirbizarre/gonja"

func render_cmd_vars_logic(cmd string, vars_list []map[string]string) (rendered_cmd string) {
	return rendered_cmd
}

func render_cmd(cmd string, vars_list []map[string]string) (rendered_cmd string, render_error error) {
	ctx := gonja.Context{}
	for _, vars := range vars_list {
		for ek, ev := range vars {
			ctx[ek] = ev
		}
	}
	tpl, err := gonja.FromString(cmd)
	if err != nil {
		return ``, err
	}
	return tpl.Execute(ctx)
}
