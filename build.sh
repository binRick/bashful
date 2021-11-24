#!/bin/bash
set -e
cd $(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
BV=5.1
BL=$(pwd)/bash-loadables
BASH_LOADABLES_DIR=$BL/bash-$BV/examples/loadables

if [[ ! -f ./submodules/bash-loadable-ansi-color/build.sh ]]; then
  rm -rf ./submodules/bash-loadable-ansi-color
  git clone git@github.com:binRick/bash-loadable-ansi-color.git ./submodules/bash-loadable-ansi-color
  git pull --recurse-submodules
fi

[[ -d "$BL" ]] || mkdir -p "$BL"
command -v bison >/dev/null || dnf -y install bison

if [[ ! -f $BL/bash-$BV/bash ]]; then
  (
    cd $BL/.
    tar zxf ../src/bash-$BV.tar.gz
    mv bash-bash-$BV bash-$BV
    cd bash-$BV
    { ./configure &&  make; } |  pv -l -N "Compiling Bash v$BV"  >/dev/null
  )
fi

if [[ ! -f $BL/color.so ]]; then
  ./submodules/bash-loadable-ansi-color/build.sh
  rsync submodules/bash-loadable-ansi-color/src/.libs/color.so $BL/.
fi

./GET_BASH_LOADABLES.sh build_modules|tr '\n' ' '
tf=$(mktemp)
./GET_BASH_LOADABLES.sh get_built_modules > $tf
cmd="rsync --files-from=$tf $BASH_LOADABLES_DIR/. $BL/. -v"
eval "$cmd"
unlink $tf

./compile.sh

if command -v rsync; then
  if [[ -d ~/.local/bin ]]; then
  	rsync bashful ~/.local/bin/bashful
  fi
  if uname -s |grep -i darwin; then
    echo darwin
  else
  if command -v bashful; then
  	rsync bashful $(command -v bashful)
  fi
	rsync bashful /usr/bin/bashful||true
	[[ -d ~/vpntech-haproxy-container/files ]] && rsync bashful ~/vpntech-haproxy-container/files/bashful
	[[ -d /opt/vpntech-binaries/x86_64 ]] && rsync bashful /opt/vpntech-binaries/x86_64/bashful
  fi
else
	cp bashful /usr/bin/bashful
fi

true
