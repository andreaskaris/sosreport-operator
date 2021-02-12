#!/bin/bash

echo "SNC installer from https://github.com/code-ready/snc"
cat <<"EOF"
OpenShift installer will create 2 VMs. It is sometimes useful to ssh inside the VMs. Add the following lines in your ~/.ssh/config file. You can then do ssh master and ssh bootstrap.

Host master
    Hostname 192.168.126.11
    User core
    IdentityFile <directory_to_cloned_repo>/id_ecdsa_crc
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null

Host bootstrap
    Hostname 192.168.126.10
    User core
    IdentityFile <directory_to_cloned_repo>/id_ecdsa_crc
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
EOF

# use OKD if no valid pull secret is provided
if ! [ -f /tmp/pull_secret.json ]; then
	echo "Using OKD upstream project instead of OpenShift"
	cat << EOF > /tmp/pull_secret.json
{"auths":{"fake":{"auth": "Zm9vOmJhcgo="}}}
EOF

	# Set environment for OKD build
	export OKD_VERSION=4.6.0-0.okd-2021-01-23-132511
fi       
export OPENSHIFT_PULL_SECRET_PATH="/tmp/pull_secret.json"

cd /tmp
git clone https://github.com/code-ready/snc.git
cd snc
./snc.sh
