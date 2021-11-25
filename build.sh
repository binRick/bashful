#!/bin/bash
set -eou pipefail
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
BD=$(pwd)
ARGS="${@:-}"

################################################################################################
##                               Setup                                                        ##
################################################################################################
do_setup() {
	set -e
	export BASHFUL_BUILD_SCRIPT=$$
	ANSIBLE_BINARY_DISTRO=fedora35
	BV=5.1
	BL=$BD/bash-loadables
	BB=$BD/bash-bin
	SM=$BD/submodules
	BASH_LOADABLES_DIR=$SM/bash-$BV/examples/loadables
	for d in $BB $BL $SM; do [[ -d "$d" ]] || mkdir -p "$d"; done
	command -v bison >/dev/null || dnf -y install bison
	rpm -qa bash-devel || dnf -y install bash-devel
}
################################################################################################

################################################################################################
##                               Time History                                                 ##
################################################################################################
build_timehistory() (
	if [[ ! -d $SM/timehistory-bash ]]; then
		cd $SM/. && git clone git@github.com:binRick/timehistory-bash.git
	fi
	if [[ ! -f $SM/timehistory-bash/target/release/libtimehistory_bash.so ]]; then
		(
			cd $SM/timehistory-bash
			command -v cargo || dnf -y install cargo
			cargo build --release
		)
	fi
	rsync $SM/timehistory-bash/target/release/libtimehistory_bash.so $BL/timehistory.so
)
################################################################################################

################################################################################################
##                               Wireguard                                                    ##
################################################################################################
build_wg() (
	REPO=bash-loadable-wireguard
	MODULE=wg
	[[ -d $BD/submodules/$REPO ]] || (cd $BD/submodules/. && git clone git@github.com:binRick/$REPO.git)
	(cd $BD/submodules/$REPO && git pull --recurse-submodules)
	(cd $BD/submodules/$REPO/. && ./build.sh)
	rsync $BD/submodules/$REPO/src/.libs/$MODULE.so $BL/.
)
################################################################################################

################################################################################################
##                               Time Utils                                                   ##
################################################################################################
build_ts() (
	REPO=bash-loadable-time-utils
	MODULE=ts
	[[ -d $BD/submodules/$REPO ]] || (cd $BD/submodules/. && git clone git@github.com:binRick/$REPO.git)
	(cd $BD/submodules/$REPO && git pull --recurse-submodules)
	(cd $BD/submodules/$REPO/. && ./build.sh)
	rsync $BD/submodules/$REPO/src/.libs/$MODULE.so $BL/.
)
################################################################################################

################################################################################################
##                               Ansi Color                                                   ##
################################################################################################
build_ansi() (
	REPO=bash-loadable-ansi-color
	MODULE=color
	[[ -d $BD/submodules/$REPO ]] || (cd $BD/submodules/. && git clone git@github.com:binRick/$REPO.git)
	(cd $BD/submodules/$REPO && git pull --recurse-submodules)
	(cd $BD/submodules/$REPO/. && ./build.sh)
	rsync $BD/submodules/$REPO/src/.libs/$MODULE.so $BL/.
)
################################################################################################

################################################################################################
##                           Bash                                                             ##
################################################################################################
build_bash() (
	if [[ ! -f $BD/submodules/bash-$BV/bash ]]; then
		cd $BD/submodules/.
		tar zxf $BD/src/bash-$BV.tar.gz
		cd $BD/submodules/bash-$BV
		{ ./configure && make; } | pv -l -N "Compiling Bash v$BV" >/dev/null
	fi
	rsync $SM/bash-$BV/bash $BB/bash
)
################################################################################################

################################################################################################
##                           Bash Example Builtins                                            ##
################################################################################################
build_bash_example_builtins() (
	./GET_BASH_LOADABLES.sh build_modules | tr '\n' ' '
)
################################################################################################
##                           Base64 Builtin                                                   ##
################################################################################################
compile_base64_builtin() (
	./GET_BASH_LOADABLES.sh compile_base64
)
################################################################################################
##                           Copy Bash Example Builtins                                       ##
################################################################################################
copy_bash_example_builtins() {
	tf=$(mktemp)
	./GET_BASH_LOADABLES.sh get_built_modules >$tf
	cmd="rsync --files-from=$tf $BASH_LOADABLES_DIR/. $BL/. -v"
	eval "$cmd"
	unlink $tf
}
################################################################################################

################################################################################################
##                           Normalize Loadable Module File Names                             ##
################################################################################################
normalize_module_file_names() (
	while read -r m; do
		echo -e "$m" | grep -q '\.so$|\.' && continue
		local dest=$(dirname $m)/$(basename $m).so
		cmd="mv -f $m $dest"
		msg="$(echo -e "moving $m to $dest.....=>\n                $cmd")"
		ansi >&2 --yellow "$msg"
		eval "$cmd"
	done < <(find $BL -type f)
)
################################################################################################

################################################################################################
##                           Compile Bashful                                                  ##
################################################################################################
compile_bashful() (
	cd $BD/.
	./compile.sh
	if command -v rsync >/dev/null; then
		if [[ -d ~/.local/bin ]]; then
			rsync bashful ~/.local/bin/bashful
		fi
		if uname -s | grep -qi darwin; then
			echo darwin
		else
			if command -v bashful >/dev/null; then
				rsync bashful $(command -v bashful)
			fi
			rsync bashful /usr/bin/bashful || true
			[[ -d ~/vpntech-haproxy-container/files ]] && rsync bashful ~/vpntech-haproxy-container/files/bashful
			[[ -d /opt/vpntech-binaries/x86_64 ]] && rsync bashful /opt/vpntech-binaries/x86_64/bashful
		fi
	else
		cp bashful /usr/bin/bashful
	fi
)
################################################################################################

################################################################################################
##                           Ansible Binaries                                                 ##
################################################################################################
compile_ansible() (
	REPO=pyinstaller-ansible-playbook
	cd $BD/submodules/.
	[[ -d $BD/submodules/$REPO ]] || git clone git@github.com:binRick/$REPO.git
	cd ./$REPO
	git reset --hard
	git pull --recurse-submodules
	cmd="cd $BD/submodules/$REPO/. && cat distros.yaml && ./bf.sh $ANSIBLE_BINARY_DISTRO"
  ansi --yellow --italic "$cmd"
  eval "$cmd"
	rsync -arv $BD/submodules/$REPO/binaries/* $BB/.
)
################################################################################################

common_main() {
	do_setup
}

do_main() {
	(
		build_bash
		build_bash_example_builtins
		compile_base64_builtin
		copy_bash_example_builtins
		wait
	) &
	(
		build_timehistory &
		build_ansi &
		build_ts &
		build_wg &
		wait
	)
	(
		compile_ansible &
		wait
	) &
	wait
	compile_bashful
}

main() {
	common_main
	if [[ "${1:-}" == "" ]]; then
		do_main
	else
		eval "$1"
	fi
}

main "$ARGS"
