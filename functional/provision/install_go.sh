#!/bin/bash -e

USER_ID=${SUDO_UID:-$(id -u)}
HOME=$(getent passwd "${USER_ID}" | cut -d: -f6)

export GOROOT=${HOME}/go
export PATH=${HOME}/go/bin:${PATH}

gover=1.5.3
gotar=go${gover}.linux-amd64.tar.gz
if [ ! -f ${HOME}/${gotar} ]; then
  # Remove unfinished archive when you press Ctrl+C
  trap "rm -f ${HOME}/${gotar}" INT TERM
  wget --no-verbose https://storage.googleapis.com/golang/${gotar} -P ${HOME}
fi
tar -xf ${HOME}/${gotar} -C ${HOME}
