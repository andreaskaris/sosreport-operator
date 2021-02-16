#!/bin/bash

# Variables:
# UPLOAD_METHOD - case|ftp|nfs
# USERNAME - username for portal or FTP
# PASSWORD - password for portal or FTP
# CASE_NUMBER - Case number for portal
# NFS_SHARE - NFS share configuration
# NFS_OPTIONS - NFS mount options
# FTP_SERVER - FTP connection string
# DEBUG - Be more verbose
# SIMULATION_MODE - If simulation mode is on, create a sosreport from the container instead of the host file system
# OBFUSCATE - Obfuscate the attachment by running it through soscleaner to remove hostnames and IPs

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

PV_DIR="/pv"

if [ "$CASE_NUMBER" != "" ]; then
	ticket_number="--ticket-number $CASE_NUMBER"
fi
if [ "$DEBUG" == "true" ] ; then
	verbose="-v"
fi
# When using kind, the host base image is Ubuntu, so the "real sosreport"
# will not work
# If simulation mode is on, create a sosreport from the container instead, just to 
# have something
simulation_mode="--sysroot /host"
if [ "$SIMULATION_MODE" == "true" ]; then
	simulation_mode="--tmp-dir /host/var/tmp"
fi
options="$ticket_number $verbose $simulation_mode"
sosreport --batch  -k crio.all=on -k crio.logs=on $options | tee /tmp/log.txt

tmp_sosreport_file=$(grep 'tar.xz' /tmp/log.txt  | awk '{print $1}')
sosreport_basename=$(basename $tmp_sosreport_file)
export sosreport_file=$PV_DIR/$sosreport_basename
echo "Moving file $tmp_sosreport_file to PV $sosreport_file"
mv $tmp_sosreport_file $sosreport_file

if [ "$UPLOAD_METHOD" == "case" ]; then
	${DIR}/upload_to_case.sh
elif [ "$UPLOAD_METHOD" == "nfs" ]; then
	${DIR}/upload_to_nfs.sh
elif [ "$UPLOAD_METHOD" == "ftp" ]; then
	${DIR}/upload_to_ftp.sh
fi
