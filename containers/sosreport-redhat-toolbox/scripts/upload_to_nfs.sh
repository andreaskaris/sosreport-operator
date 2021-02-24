#!/bin/bash

if [ "$NFS_SHARE" == "" ]; then
	echo "No NFS share provided. Exiting script."
	exit 1
fi

if [ "$NFS_OPTIONS" != "" ] ; then
        nfs_options="-o $NFS_OPTIONS"
fi

mount -t nfs $NFS_SHARE /mnt $nfs_options

if ! $(mount | grep 'on /mnt' | grep -q "$NFS_SHARE"); then
	echo "Could not mount NFS share. Exiting."
	exit 1
fi

echo "Uploading file $sosreport_file to NFS share $NFS_SHARE"
cp $sosreport_file /mnt/.

umount $NFS_SHARE
