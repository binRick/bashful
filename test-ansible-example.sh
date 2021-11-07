#!/usr/bin/env bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source .optparse.sh

EXAMPLE_FILE_PREFIX=ansible-test
EXAMPLE_FILE_DIR="$(pwd)/example"
BASHFUL_BINARY=$(pwd)/bashful

optparse.define short=f long=file variable=EXAMPLE_FILE_NAME_SUFFIX= desc="Example File Suffix" default=file-lifecycle
optparse.define short=V long=verbose variable=VERBOSE_MODE desc="Verbose Mode" default= value=1
optparse.define short=D long=debug variable=DEBUG_MODE desc="Debug Mode" default= value=1
optparse.define short=l long=list variable=LIST_EXAMPLE_FILE_NAMES_MODE desc="List Example File Names" default= value=1
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

cmd="$ls_example_files_cmd" debug_cmd
eval "$ls_example_files_cmd"

cmd="$BASHFUL_BINARY run $EXAMPLE_FILE"
[[ "$VERBOSE_MODE" == 1 ]] && cmd="$cmd --verbose"
nodemon_cmd="$(command -v nodemon) -V --signal SIGKILL  -I $NODEMON_WATCH_FILES -e $NODEMON_WATCH_EXTENSIONS -x $(command -v sh) -- -c '$cmd||true; clear'"
[[ "$NODEMON_MODE" == 1 ]] && cmd="$nodemon_cmd"

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

[[ "$PREVIEW_MODE" == 1 ]] && eval "$preview_cmd"
if [[ "$LIST_EXAMPLE_FILE_NAMES_MODE" == "1" ]]; then
	ansi --yellow list
else
  [[ "$DEBUG_MODE" == 1 ]] && debug_cmd
	[[ "$DRY_RUN_MODE" != 1 &&  "$RUN_MODE" == enabled ]] && eval "$cmd"
fi
