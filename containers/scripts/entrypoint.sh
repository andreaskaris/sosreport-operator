#!/bin/bash

# Variables:
# CASE_NUMBER - Case number for portal
# RH_USERNAME - RH username for portal
# RH_PASSWORD - RH password for portal
# UPLOAD_SOSREPORT - Upload sosreport to case
# UPLOAD_METHOD - case|ftp|nfs
# NFS - NFS share configuration
# FTS - FTP connection string
# DEBUG - Be more verbose
# SIMULATION_MODE - If simulation mode is on, create a sosreport from the container instead of the host file system
# OBFUSCATE - Obfuscate the attachment by running it through soscleaner to remove hostnames and IPs

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

if [ "$CASE_NUMBER" != "" ]; then
	ticket_number="--ticket-number $CASE_NUMBER"
fi
if ${DEBUG:-false} ; then
	verbose="-v"
fi
# When using kind, the host base image is Ubuntu, so the "real sosreport"
# will not work
# If simulation mode is on, create a sosreport from the container instead, just to 
# have something
simulation_mode="--sysroot /host"
if ${SIMULATION_MODE:-false}; then
	simulation_mode="--tmp-dir /host/var/tmp"
fi
options="$ticket_number $verbose $simulation_mode"
sosreport --batch  -k crio.all=on -k crio.logs=on $options | tee -a /tmp/log.txt

export sosreport_file=$(grep 'tar.xz' /tmp/log.txt  | awk '{print $1}')

if ${UPLOAD_SOSREPORT:-false} ; then
  if [ "$UPLOAD_METHOD" == "case" ]; then
	  ${DIR}/upload_to_case.sh
  fi
fi
