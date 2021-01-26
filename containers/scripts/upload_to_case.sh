#!/bin/bash

if [ "$CASE_NUMBER" == "" ] ; then
    echo "No case number provided. Cannot upload sosreport to case."
    exit
fi
if [ "$USERNAME" == "" ]; then
    echo "No username provided. Cannot upload sosreport to case."
    exit 1
fi
if [ "$PASSWORD" == "" ]; then
    echo "No password provided. Cannot upload sosreport to case."
    exit 1
fi
mkdir /root/.redhat-support-tool/ 2>/dev/null
cat <<EOF > /root/.redhat-support-tool/redhat-support-tool.conf
[RHHelp]
user = $USERNAME
password = $(/pw_decoder.py encode $USERNAME $PASSWORD)
EOF

if ${OBFUSCATE:-false}; then
    obfuscate="--obfuscate"
fi
case_number="-c $CASE_NUMBER"
support_tool_options="$case_number $obfuscate"

echo "n" | redhat-support-tool addattachment $support_tool_options $sosreport_file
# remove the authentication file
rm -f /root/.redhat-support-tool/redhat-support-tool.conf
