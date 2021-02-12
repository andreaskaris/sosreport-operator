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

echo "Create wide open libvirtd-tcp.socket and lock down through firewall rules"
sudo systemctl restart libvirtd-tcp.socket
sudo systemctl enable libvirtd-tcp.socket
sudo sed -i 's/#auth_tcp.*/auth_tcp = "none"/' /etc/libvirt/libvirtd.conf
sudo systemctl restart libvirtd
sudo firewall-cmd --add-rich-rule "rule service name="libvirt" reject"
sudo firewall-cmd --add-rich-rule "rule service name="libvirt" reject" --permanent
sudo firewall-cmd --zone=libvirt --add-service=libvirt
sudo firewall-cmd --zone=libvirt --add-service=libvirt --permanent

echo "Configure dnsmasq ..."
echo -e "[main]\ndns=dnsmasq" | sudo tee /etc/NetworkManager/conf.d/openshift.conf
echo server=/tt.testing/192.168.126.1 | sudo tee /etc/NetworkManager/dnsmasq.d/openshift.conf
sudo systemctl reload NetworkManager

