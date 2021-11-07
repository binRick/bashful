#!/usr/bin/env bash
set -e -o pipefail
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source .optparse.sh

EXAMPLE_FILE_PREFIX=ansible-test
EXAMPLE_FILE_DIR="$(pwd)/example"
BASHFUL_BINARY=$(pwd)/bashful
DEFAULT_MODE=run
optparse.define short=f long=file variable=EXAMPLE_FILE_NAME_SUFFIX desc="Example File Suffix" default=file-lifecycle
optparse.define short=V long=verbose variable=VERBOSE_MODE desc="Verbose Mode" default= value=1
optparse.define short=D long=debug variable=DEBUG_MODE desc="Debug Mode" default= value=1
optparse.define short=m long=mode variable=EXEC_MODE desc="Mode- run, list, list-files" default=$DEFAULT_MODE
optparse.define short=P long=preview variable=PREVIEW_MODE desc="Preview Mode" default= value=1
optparse.define short=N long=nodemon variable=NODEMON_MODE desc="Nodemon Mode" default= value=1
optparse.define short=n long=dry-run variable=DRY_RUN_MODE desc="Dry Run Mode" default= value=1
source "$(optparse.build)"

setup_options() {
	ls_example_files_cmd="ls $EXAMPLE_FILE_DIR/$EXAMPLE_FILE_PREFIX-*.yml|xargs -I % basename % .yml|sed \"s|^$EXAMPLE_FILE_PREFIX-||g\""
	_ls_example_files_cmd="ls $EXAMPLE_FILE_DIR/$EXAMPLE_FILE_PREFIX-*.yml|xargs -I % echo %"
	NODEMON_WATCH_FILES="-w $BASHFUL_BINARY -w . -w example"
	NODEMON_WATCH_EXTENSIONS="sh,yaml,j2"
}

of=$(mktemp).yaml
yaml_decode_error_file=$(mktemp).log

cleanup() {
	[[ -f "$of" ]] && unlink "$of"
	[[ -f "$yaml_decode_error_file=" ]] && unlink "$yaml_decode_error_file="
	true
}





run_cmd() (
	local _cmd="$1"
	local of=$(mktemp)
	local ef=$(mktemp)
	local ecf=$(mktemp)
	local pidf=$(mktemp)
	local df=$(mktemp)
	(
		set +e
		echo $$ >$pidf
		eval "$_cmd" >$of 2>$ef
		ec=$?
		echo -e "$ec" >$ecf
		date +%s >$df
		sleep .01
	) &
	sleep .01

	local _pid=$(cat $pidf)
	event="$(ansi --magenta --bold "started")"
	msg="[PID $_pid] [$event] :: $(ansi --cyan --underline "$_cmd")"
	if [[ "$DEBUG_MODE" == "1" ]]; then
		echo >&2 -e "$msg"
	fi
	wait
	local _o=$(cat $of)
	local _e=$(cat $ef)
	local _ec=$(cat $ecf)
	local _ended=$(cat $df)
	ec_color=yellow
	ec_style=
	details=
	[[ "$_ec" == 0 ]] && ec_color=green && ec_style='--italic'
	if [[ "$_ec" -gt 0 ]]; then
		ec_color=red && ec_style='--blink --underline --bold --bg-black'
		details="$(
			cat <<EOF

==================================================================================================================
          $(ansi --cyan --bold --underline "Command")        $(ansi --magenta --italic "$_cmd")
          $(ansi --cyan --bold --underline "Exit Code")      $(ansi --magenta --italic "$_ec")
==================================================================================================================
          $(ansi --cyan --bold --underline "Std Output")        $(ansi --magenta --italic "$of")
==================================================================================================================
$(ansi --yellow --bg-black "$(cat "$of")")
==================================================================================================================
          $(ansi --cyan --bold --underline "Std Error")        $(ansi --magenta --italic "$ef")
==================================================================================================================
$(ansi --yellow --blink --bg-black "$(cat "$ef")")
==================================================================================================================

EOF
		)"

	fi
	event="$(ansi --magenta --bold "ended")"
	msg="[PID $_pid] [$event] :: Exited $(eval ansi --$ec_color $ec_style "$_ec") @ $_ended |$details"
	if [[ "$DEBUG_MODE" == "1" ]]; then
		echo >&2 -e "$msg"
	fi

)

dev_mode() {
	set +e
	(
		run_cmd "ls /1"
		run_cmd "ls /"
	) >/dev/null
	exit
}

#__dev

debug_item() {
	(
		set +e

		local _type="$1"
		local _title="$2"
		local _cmd="$3"
		local _json="type='$_type' title='$_title' cmd='$_cmd'"
		if [[ "$DEBUG_MODE" == 1 ]]; then
			_json="$(eval jo $_json)"
			echo -e "$_json" | jq -C >&2
			msg="$(
				cat <<EOF
$(ansi --magenta "[$_type]") $(ansi --cyan "$_title"): $(ansi --yellow --italic --bg-black "$_cmd")
EOF
			)"
			echo >&2 -e "$msg"
		fi
	)
}

trap cleanup EXIT

ls_example_file_name_suffixes() {
	[[ "$DEBUG_MODE" == 1 ]] && debug_item Command "List Example File Name Suffixes" "$ls_example_files_cmd"
	eval "$ls_example_files_cmd"
}

ls_example_files() {
	[[ "$DEBUG_MODE" == 1 ]] && cmd="$_ls_example_files_cmd" debug_item Command "List Example Files" "$_ls_example_files_cmd"
	eval "$_ls_example_files_cmd"
}

ls_example_file_names() {
	ls_example_files | xargs -I % basename %
}

__ls_example_files() {
	while read -r l; do
		fp="$EXAMPLE_FILE_DIR/$EXAMPLE_FILE_PREFIX-$l.yml"
		if [[ -f "$fp" ]]; then
			echo -e "$fp"
		else
			ansi --red --bold "Example file '$fp' does not exist!"
			exit 1
		fi
	done < <(ls_example_file_name_suffixes)
	true
}

setup_cmds() {
	setup_options
	EXAMPLE_FILE="$EXAMPLE_FILE_DIR/$EXAMPLE_FILE_PREFIX-$EXAMPLE_FILE_NAME_SUFFIX.yml"
	validate_cmd="command cat $EXAMPLE_FILE|yaml2json 2>$yaml_decode_error_file|json2yaml >/dev/null"
	preview_cmd="command cat $EXAMPLE_FILE|yaml2json 2>/dev/null|json2yaml>$of && command bat --pager=never --style=plain --theme=DarkNeon $of"
	cmd="$BASHFUL_BINARY run $EXAMPLE_FILE"
	[[ "$VERBOSE_MODE" == 1 ]] && cmd="$cmd --verbose"
	nodemon_cmd="$(command -v nodemon) -V --signal SIGKILL  -I $NODEMON_WATCH_FILES -e $NODEMON_WATCH_EXTENSIONS -x $(command -v bash) -- -c '$cmd||true;'"
	[[ "$NODEMON_MODE" == 1 ]] && cmd="$nodemon_cmd"
	true
}

validate_example_file() {
	if [[ "$EXAMPLE_FILE" != all ]]; then
		if [[ ! -f "$EXAMPLE_FILE" ]]; then
			ansi --red --bold "[validate] Example file '$EXAMPLE_FILE' does not exist!"
			ls_example_files
			exit 1
		fi
	fi
}

preview_yaml_contents() {
	local _f="$1"
	local _t="$2"
	cat <<EOF

$(ansi --red --bg-white --blink --underline "$_t")
    
$(ansi --red --bold "$(command cat "$_f" | egrep -v 'YAMLLoadWarning')")

EOF

}

validate_yaml() {
	if ! eval "$validate_cmd" 2>/dev/null; then
		contents="$(cat $yaml_decode_error_file | egrep -v 'YAMLLoadWarning')"
		(
			msg="$(
				cat <<EOF

$(ansi --red --bg-white --blink --underline "     YAML Failed to Decode!     ")
    
$(ansi --red --bold "$contents")

EOF
			)"
			echo -e "$msg"
		) >&2
		exit 1
	fi
}

validate() {
	validate_example_file
	validate_yaml
}

dev_mode1() {
	preview_yaml_contents "/tmp/passwd" "title"
	exit 2
}

re_exec() {
	local me="${BASH_SOURCE[0]}"
	local args=
	[[ "$DRY_RUN_MODE" == 1 ]] && args+=" --dry-run"
	[[ "$PREVIEW_MODE" == 1 ]] && args+=" --preview"
	[[ "$DEBUG_MODE" == 1 ]] && args+=" --debug"
	[[ "$VERBOSE_MODE" == 1 ]] && args+=" --verbose"
	local c="$me --file $fs --mode $EXEC_MODE $args"

	local _cmd="$me "
}

check_multiple() {
	setup_options
	#set -x
	if echo -e "$EXAMPLE_FILE_NAME_SUFFIX" | egrep -q ',|all'; then
		local re_exec_cmds=()
		local re_exec_cmd="$(command -v multiview)  -p -c $EXAMPLE_FILE_PREFIX"
		RE_EXEC_MODES="$(echo -e "$EXAMPLE_FILE_NAME_SUFFIX" | tr ',' '\n' | egrep -v '^$')"
		#debug_item "ls_example_files_cmd" "ls_example_files_cmd" "$ls_example_files_cmd"
		#debug_item "RE_EXEC_MODES" "RE_EXEC_MODES" "$RE_EXEC_MODES"
		all_loaded=0
		while read -r _m; do
			if [[ "$_m" != "" && "$all_loaded" == 0 && "$_m" == all ]]; then
				while read -r _am; do
					if [[ "$_am" != "" ]]; then
						RE_EXEC_MODES="$RE_EXEC_MODES\n$_am"
					fi
				done < <(ls_example_file_name_suffixes)
				all_loaded=1
			fi
		done < <(echo -e "$RE_EXEC_MODES")
		RE_EXEC_MODES="$(echo -e "$RE_EXEC_MODES" | egrep -v '^all$')"
		#debug_item "RE_EXEC_MODES" "RE_EXEC_MODES" "$RE_EXEC_MODES"
		while read -r fs; do
			if [[ "$fs" != "" ]]; then
				local me="${BASH_SOURCE[0]}"
				local args=
				[[ "$DRY_RUN_MODE" == 1 ]] && args+=" --dry-run"
				[[ "$PREVIEW_MODE" == 1 ]] && args+=" --preview"
				[[ "$DEBUG_MODE" == 1 ]] && args+=" --debug"
				[[ "$VERBOSE_MODE" == 1 ]] && args+=" --verbose"
				lf=$(mktemp)
				local c="$me --file $fs --mode $EXEC_MODE $args"
				local re_exec_cmd+=" [ $c ]"
				re_exec_cmds+=("$c")
			fi
		done < <(echo -e "$RE_EXEC_MODES")
		(
			for _cmd in "${re_exec_cmds[@]}"; do
				debug_item "Command" "Re Exec Command" "$_cmd"
				if [[ "$IS_EXITING" == 1 ]]; then continue; fi
				ansi >&2 --yellow --bg-black "$_cmd"
				set +e
				eval "$_cmd"
        ec=$?
			done
		)
		exit
	fi
}

main() {
	setup_cmds
	check_multiple
	case "$EXEC_MODE" in
	d | dev) dev_mode ;;
	D) dev_mode1 ;;
	validate) validate ;;
	run)
		[[ "$PREVIEW_MODE" == 1 ]] && eval "$preview_cmd"
		[[ "$DEBUG_MODE" == 1 ]] && debug_item Command "Bashful Run" "$cmd"
		if [[ "$DRY_RUN_MODE" != 1 ]]; then
			validate
			eval "$cmd"
		fi
		;;
	names | file-names | list-file-names)
		ls_example_file_names
		;;
	ls | list)
		ls_example_file_name_suffixes
		;;
	files | list-files)
		ls_example_files
		;;
	*)
		ansi --red --bold "Unhandled Mode '$EXEC_MODE'"
		exit 1
		;;
	esac
	exit
}

main
