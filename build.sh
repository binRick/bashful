#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
BV=5.1
BD=$(pwd)
BL=$BD/bash-loadables
BASH_LOADABLES_DIR=$BL/bash-$BV/examples/loadables
[[ -d "$BL" ]] || mkdir -p $BL
command -v bison >/dev/null || dnf -y install bison
BINRICK_GITHUB_REPOS="bash-loadable-ansi-color bash-loadable-time-utils bash-loadable-wireguard"



################################################################################################
##                               Time History                                                 ##
################################################################################################
if [[ ! -d ./submodules/timehistory-bash ]]; then
  git clone git@github.com:binRick/timehistory-bash ./submodules/timehistory-bash
fi
if [[ ! -f ./submodules/timehistory-bash/target/release/libtimehistory_bash.so ]]; then
  (
    cd ./submodules/timehistory-bash
    command -v cargo || dnf -y install cargo
    cargo build --release
  )
fi
rsync ./submodules/timehistory-bash/target/release/libtimehistory_bash.so $BL/timehistory.so
################################################################################################


################################################################################################
##                               Wireguard                                                    ##
################################################################################################
REPO=bash-loadable-wireguard
MODULE=wg
[[ -d $BD/submodules/$REPO ]] || (cd $BD/submodules/. && git clone git@github.com:binRick/$REPO.git)
(cd $BD/submodules/$REPO && git pull --recurse-submodules)
(cd $BD/submodules/$REPO/. && ./build.sh)
rsync $BD/submodules/$REPO/src/.libs/$MODULE.so $BL/.
################################################################################################


################################################################################################
##                               Time Utils                                                   ##
################################################################################################
REPO=bash-loadable-time-utils
MODULE=ts
[[ -d $BD/submodules/$REPO ]] || (cd $BD/submodules/. && git clone git@github.com:binRick/$REPO.git)
(cd $BD/submodules/$REPO && git pull --recurse-submodules)
(cd $BD/submodules/$REPO/. && ./build.sh)
rsync $BD/submodules/$REPO/src/.libs/$MODULE.so $BL/.
################################################################################################


################################################################################################
##                               Ansi Color                                                   ##
################################################################################################
REPO=bash-loadable-ansi-color
MODULE=color
[[ -d $BD/submodules/$REPO ]] || (cd $BD/submodules/. && git clone git@github.com:binRick/$REPO.git)
(cd $BD/submodules/$REPO && git pull --recurse-submodules)
(cd $BD/submodules/$REPO/. && ./build.sh)
rsync $BD/submodules/$REPO/src/.libs/$MODULE.so $BL/.
################################################################################################


################################################################################################
##                           Bash Binary                                                      ##
################################################################################################
if [[ ! -f $BL/bash-$BV/bash ]]; then
  (
    cd $BL/.
    tar zxf ../src/bash-$BV.tar.gz
    [[ -d bash-bash-$BV ]] && mv bash-bash-$BV bash-$BV
    cd bash-$BV
    { ./configure &&  make; } |  pv -l -N "Compiling Bash v$BV"  >/dev/null
  )
fi
################################################################################################



################################################################################################
##                           Summarize Loadables                                              ##
################################################################################################
./GET_BASH_LOADABLES.sh build_modules|tr '\n' ' '
./GET_BASH_LOADABLES.sh compile_base64
tf=$(mktemp)
./GET_BASH_LOADABLES.sh get_built_modules > $tf
cmd="rsync --files-from=$tf $BASH_LOADABLES_DIR/. $BL/. -v"
eval "$cmd"
unlink $tf
################################################################################################



################################################################################################
##                           Compile Bashful                                                  ##
################################################################################################
./compile.sh
if command -v rsync >/dev/null; then
  if [[ -d ~/.local/bin ]]; then
  	rsync bashful ~/.local/bin/bashful
  fi
  if uname -s |grep -qi darwin; then
    echo darwin
  else
  if command -v bashful >/dev/null; then
  	rsync bashful $(command -v bashful)
  fi
	rsync bashful /usr/bin/bashful||true
	[[ -d ~/vpntech-haproxy-container/files ]] && rsync bashful ~/vpntech-haproxy-container/files/bashful
	[[ -d /opt/vpntech-binaries/x86_64 ]] && rsync bashful /opt/vpntech-binaries/x86_64/bashful
  fi
else
	cp bashful /usr/bin/bashful
fi
################################################################################################
