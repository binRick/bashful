#!/usr/bin/env bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source .optparse.sh

EXAMPLE_FILE_PREFIX=ansible-test
EXAMPLE_FILE_DIR="$(pwd)/example"
BASHFUL_BINARY=$(pwd)/bashful
DEFAULT_MODE=run
optparse.define short=f long=file variable=EXAMPLE_FILE_NAME_SUFFIX= desc="Example File Suffix" default=file-lifecycle
optparse.define short=V long=verbose variable=VERBOSE_MODE desc="Verbose Mode" default= value=1
optparse.define short=D long=debug variable=DEBUG_MODE desc="Debug Mode" default= value=1
optparse.define short=m long=mode variable=EXEC_MODE desc="Mode- run, list, list-files" default=$DEFAULT_MODE
optparse.define short=P long=preview variable=PREVIEW_MODE desc="Preview Mode" default= value=1
optparse.define short=N long=nodemon variable=NODEMON_MODE desc="Nodemon Mode" default= value=1
optparse.define short=n long=nodemon variable=DRY_RUN_MODE desc="Dry Run Mode" default= value=1
optparse.define short=R long=run-mode variable=RUN_MODE desc="Set Run Mode (default enabled)" default=enabled
source "$(optparse.build)"

of=$(mktemp).yaml
yaml_decode_error_file=$(mktemp).log
ls_example_files_cmd="ls $EXAMPLE_FILE_DIR/$EXAMPLE_FILE_PREFIX-*.yml|xargs -I % basename % .yml|sed \"s|^$EXAMPLE_FILE_PREFIX-||g\""

cleanup() {
	[[ -f "$of" ]] && unlink "$of"
	[[ -f "$yaml_decode_error_file=" ]] && unlink "$yaml_decode_error_file="
	true
}

debug_cmd() {
	msg="$(
		cat <<EOF
$(ansi --cyan "Command:") $(ansi --yellow --italic --bg-black "$cmd")
EOF
	)"
	echo >&2 -e "$msg"

}

trap cleanup EXIT

EXAMPLE_FILE="$EXAMPLE_FILE_DIR/$EXAMPLE_FILE_PREFIX-$EXAMPLE_FILE_NAME_SUFFIX.yml"
validate_cmd="command cat $EXAMPLE_FILE|yaml2json 2>$yaml_decode_error_file|json2yaml >/dev/null"
preview_cmd="command cat $EXAMPLE_FILE|yaml2json 2>/dev/null|json2yaml>$of && command bat --pager=never --style=plain --theme=DarkNeon $of"
NODEMON_WATCH_FILES="-w $BASHFUL_BINARY -w . -w example"
NODEMON_WATCH_EXTENSIONS="sh,yaml,j2"

ls_example_file_name_suffixes() {
	[[ "$DEBUG_MODE" == 1 ]] && cmd="$ls_example_files_cmd" debug_cmd
	eval "$ls_example_files_cmd"
}

ls_example_files() {
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

cmd="$BASHFUL_BINARY run $EXAMPLE_FILE"
if [[ ! -f "$EXAMPLE_FILE" ]]; then
	ansi --red --bold "Example file '$EXAMPLE_FILE' does not exist!"
	ls_example_files
	exit 1
fi
[[ "$VERBOSE_MODE" == 1 ]] && cmd="$cmd --verbose"
nodemon_cmd="$(command -v nodemon) -V --signal SIGKILL  -I $NODEMON_WATCH_FILES -e $NODEMON_WATCH_EXTENSIONS -x $(command -v bash) -- -c '$cmd||true; reset;'"
[[ "$NODEMON_MODE" == 1 ]] && cmd="$nodemon_cmd"

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

main() {
	case "$EXEC_MODE" in
	run)
		[[ "$PREVIEW_MODE" == 1 ]] && eval "$preview_cmd"
		[[ "$DEBUG_MODE" == 1 ]] && debug_cmd
		if [[ "$DRY_RUN_MODE" != 1 && "$RUN_MODE" == enabled ]]; then
			validate_yaml

			eval "$cmd"
		fi
		;;
	list)
		ls_example_file_name_suffixes
		;;
	list-files)
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
