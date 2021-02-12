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

cd /tmp

forward_enabled=$(sysctl net.ipv4.ip_forward)
if [ "$forward_enabled" == "net.ipv4.ip_forward = 0" ]; then
	echo "net.ipv4.ip_forward = 1" | sudo tee /etc/sysctl.d/99-ipforward.conf
	sudo sysctl -p /etc/sysctl.d/99-ipforward.conf
fi

echo "Install dependencies ..."
sudo yum install golang-bin gcc-c++ libvirt-devel
sudo yum install libvirt-devel libvirt-daemon-kvm libvirt-client
sudo systemctl enable --now libvirtd

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
# echo server=/tt.testing/192.168.126.1 | sudo tee /etc/NetworkManager/dnsmasq.d/openshift.conf
sudo systemctl reload NetworkManager

# echo "Clone and build openshift installer ..."
# git clone https://github.com/openshift/installer.git
# cd installer
# TAGS=libvirt hack/build.sh

# Fix some weird bug with DNS resolution
# NetworkManager dnsmasq will listen on 127.0.0.1  but the systemd-resolved resolver will listen on 127.0.0.53 and will
# not forward to 127.0.0.1
# The below fixes this
# https://askubuntu.com/questions/1057953/dns-systemd-resolve-dnsmasq-resolvconf-problems-errors-in-syslog-18-04
sudo sed -i 's/^#DNS=.*/DNS=127.0.0.1/' /etc/systemd/resolved.conf
sudo systemctl restart systemd-resolved

echo "Done." 
echo "It may be required to restart your system now depending on the following output."
sudo ss -lntp | grep 16509
sudo systemctl status libvirtd-tcp.socket
