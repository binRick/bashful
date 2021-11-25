#!/usr/bin/env bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
MODE="${1:-main}"
MODULES_ENABLED=${MODULES_ENABLED:-1}

BV=5.1
LOADED_MODULE_FILES="color.so print ln cut basename uname unlink tee sleep seq rm rmdir realpath print printenv mktemp mkdir id head dirname base64 timehistory.so id"
BD=$(pwd)
BL=$BD/bash-loadables
SM=$BD/submodules
BASH_DIR=$SM/bash-$BV
BASH_BIN=$BASH_DIR/bash
DIR=$BASH_DIR/examples/loadables
[[ -d "$BL" ]] || mkdir -p "$BL"
[[ -d "$SM" ]] || mkdir -p "$SM"

create_bash_script_prefix() {
	F=$BL/script.sh
	cat <<EOF >$F
#!$BASH_BIN
set -eou pipefail
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
MODULES_ENABLED="\${MODULES_ENABLED:-$MODULES_ENABLED}"

{
  echo -e "OK- Bash V$BASH_VERSION";
  echo -e "    OS $OSTYPE|$MACHTYPE";
  echo -e "    PROMPT_COMMAND $PROMPT_COMMAND";
} >&2

get_loaded_modules(){
  LOADED="\$(enable -p | sort -u|grep '^enable [a-z]'|cut -d' ' -f2|tr ' ' '\n' |grep -v '^$')"
  LOADABLES="\$(echo -e "\$LOADED"|tr '\n' ' ')"
  QTY="\$(echo -e "\$LOADED"|wc -l)"
  echo -e "    Loaded \$QTY Loadables: \$LOADABLES"
}

load_modules(){
  while read -r m; do load_module "\$m" "\$(basename \$m .so)"; done < <(echo -e "$LOADED_MODULE_FILES"|tr ' ' '\n')
}

load_module(){
  m="\$1"; n="\$2"
  CMD="enable -f $BL/\$m \$n"
  ansi --yellow "\$CMD"
  eval "\$CMD"
}

if [[ "$MODULES_ENABLED" == 1 ]]; then
  {
    get_loaded_modules;
    load_modules;
    get_loaded_modules;
  } >&2
fi

if command -v color; then
  color fg green
  color --italic
fi
echo Timehistory:
timehistory -j|jq -Mrc '.[].filename'

echo OK


EOF
	chmod +x $F
}

coproc_dev() {
	coproc cp0 (while :; do
		read -r input
		echo ">>${input}"
	done)
	coproc cp1 {
		date
		#    while read -r input; do echo ">>${input}"; done
	}

	echo "The cp1 coprocess array: ${cp1[@]}"

	echo "The PID of the cp1 coprocess is ${cp1_PID}"

	read -r output <&"${cp1[0]}"
	echo "The output of the cp1 coprocess is ${output}"

	for x in $(seq 1 10); do
		(echo "hello world $x" >&"${cp0[1]}")
		read -r output <&"${cp0[0]}"
		echo "The output of the cp0 coprocess is ${output}"
	done

	exit
}

benchmarks() {
	#qty=5000 benchmark_module /usr/bin/uname "uname -a" $DIR/uname uname "uname -a"
	#qty=1000 benchmark_module /usr/bin/dirname "dirname /etc" $DIR/dirname dirname "dirname /etc"
	#qty=1000 benchmark_module /usr/bin/mktemp mktemp $DIR/mktemp mktemp mktemp
	#qty=5000 benchmark_module /usr/bin/tee 'tee <<< OK' $DIR/tee tee
	qty=5000 benchmark_module /usr/bin/base64 'base64 <<< 13dfe6a8-ec81-4712-9bb8-bea3466dcfbb' $DIR/base64
}

benchmark_module() {
	qty="${qty:-10}"
	b="$1"
	b_cmd="$2"
	f="$3"
	m="${4:-$(basename $f)}"
	m_cmd="${5:-$b_cmd}"
	msg="qty:$qty, "
	time (
		for x in $(seq 1 $qty); do
			eval "$b_cmd"
		done
	) | pv -l >/dev/null
	time (
		enable -f $f $m
		for x in $(seq 1 $qty); do
			eval "$m_cmd"
		done
	) | pv -l >/dev/null
}

get_cur_modules() { enable | sort -u; }

get_built_modules() {
	while read -r m; do
		[[ -f "$DIR/$m" ]] && echo -e "$m"
	done < <(get_modules)
}

build_modules() (
	cd $DIR
	make
)

get_built_modules_with_so() (
	get_built_modules | with_so
)
get_modules() (
	cd $DIR
	ls *.c | xargs -I % basename % .c | sort -u
)

with_so() {
	while read -r f; do echo -e "$f.so"; done
}

get_modules_with_path() {
	while read -r m; do
		echo -e "$DIR/$m"
	done < <(get_built_modules)
}

get_load_modules_cmds() {
	set +e
	while read -r m; do
		local cmd="enable -f $DIR/$m $m"
		if eval "$cmd" 2>/dev/null; then
			ansi >&2 -n --green "$m "
		else
			ansi >&2 -n --red "$m "
		fi
	done < <(get_built_modules)
	echo
}

get_loaded() {
	now_modules="$(get_cur_modules)"
	comm -23 \
		<(echo -e "$now_modules") \
		<(echo -e "$cur_modules") | cut -d' ' -f2 | grep -v '^$' | tr '\n' ' '
}

main() {
	cur_modules="$(get_cur_modules)"
	get_load_modules_cmds >&2
	LOADED="$(get_loaded)"
	qty="$(echo -e "$LOADED" | tr ' ' '\n' | wc -l)"
	msg="$(ansi --magenta --bold "Loaded $qty Modules"): $(ansi --cyan --bold "$LOADED")"
	echo -e "$msg"
}

compile_base64() (
	cd $DIR
	gcc -fPIC -DHAVE_CONFIG_H -DSHELL -g -O2 -Wno-parentheses -Wno-format-security -I. -I.. -I../.. -I../../lib -I../../builtins -I. -I../../include -I/root/bashful/submodules/bash-5.1 -I/root/bashful/submodules/bash-5.1/lib -I/root/bashful/submodules/bash-5.1/builtins -c -o base64.o base64.c
	gcc -shared -Wl,-soname,base64 -o base64 base64.o
	rsync base64 $BL/.
)

eval "$MODE"
