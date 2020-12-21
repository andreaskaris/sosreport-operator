#!/bin/bash -x

# Variables:
# CASE_NUMBER - Case number for portal
# RH_USERNAME - RH username for portal
# RH_PASSWORD - RH password for portal
# UPLOAD_SOSREPORT - Upload sosreport to case
# DEBUG - Be more verbose
# SIMULATION_MODE - If simulation mode is on, create a sosreport from the container instead of the host file system
# OBFUSCATE - Obfuscate the attachment by running it through soscleaner to remove hostnames and IPs

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

sosreport_file=$(grep 'tar.xz' /tmp/log.txt  | awk '{print $1}')

if ${UPLOAD_SOSREPORT:-false} ; then
    if [ "$CASE_NUMBER" == "" ] ; then
	echo "No case number provided. Cannot upload sosreport to case."
	exit
    fi
    if [ "$RH_USERNAME" == "" ]; then
	echo "No username provided. Cannot upload sosreport to case."
	exit 1
    fi
    if [ "$RH_PASSWORD" == "" ]; then
	echo "No password provided. Cannot upload sosreport to case."
	exit 1
    fi
    cat <<EOF > /root/.redhat-support-tool/redhat-support-tool.conf
[RHHelp]
user = $RH_USERNAME
password = $(/pw_decoder.py encode $RH_USERNAME $RH_PASSWORD)
EOF
    if ${OBFUSCATE:-false}; then
    	obfuscate="--obfuscate"
    fi
    case_number="-c $CASE_NUMBER"
    support_tool_options="$case_number $description $obfuscate"
    
    redhat-support-tool addattachment $support_tool_options $sosreport_file
fi
