#!/bin/bash

if [ "$FTP_SERVER" == "" ]; then
	echo "No FTP server provided. Exiting script."
	exit 1
fi

if [ "$USERNAME" != "" ] && [ "$PASSWORD" != "" ] ; then
	# https://superuser.com/questions/360966/how-do-i-use-a-bash-variable-string-containing-quotes-in-a-command
	ftp_user_pass=(--user="${USERNAME}" --password="${PASSWORD}")
fi

echo "Uploading file $sosreport_file to FTP server $FTP_SERVER"
lftp "${ftp_user_pass[@]}" -e "put $sosreport_file;quit" $FTP_SERVER
