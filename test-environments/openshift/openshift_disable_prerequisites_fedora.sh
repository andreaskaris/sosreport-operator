#!/bin/bash

os_ver=$(cat /etc/redhat-release)
if [ "$os_ver" != "Fedora release 33 (Thirty Three)" ]; then
	echo "Unsupported OS version."
	exit 1
fi

stat /dev/kvm
if [ $? -ne 0 ] ; then
	echo "Enable KVM before continuing".
	exit 1
fi

echo "Stop wide open libvirtd-tcp.socket"
sudo systemctl stop libvirtd-tcp.socket
sudo systemctl disable libvirtd-tcp.socket
sudo sed -i 's/auth_tcp = "none"/#auth_tcp = "none"/' /etc/libvirt/libvirtd.conf
sudo systemctl restart libvirtd

echo "Unconfigure dnsmasq ..."
sudo rm -f /etc/NetworkManager/conf.d/openshift.conf
sudo systemctl reload NetworkManager

sudo firewall-cmd --zone=libvirt --remove-service=libvirt --permanent
sudo firewall-cmd --zone=libvirt --remove-service=libvirt
sudo firewall-cmd --remove-rich-rule "rule service name="libvirt" reject" --permanent
sudo firewall-cmd --remove-rich-rule "rule service name="libvirt" reject"

