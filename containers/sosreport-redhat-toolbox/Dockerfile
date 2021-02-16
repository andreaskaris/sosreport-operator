FROM registry.redhat.io/rhel8/support-tools
RUN yum install sos -y
RUN yum install redhat-support-tool -y
RUN yum install python2 -y
RUN yum install nfs-utils -y
RUN yum install lftp -y
RUN yum install sssd -y
RUN mkdir /etc/modules-load.d
Run echo -e "tcp_diag\naf_packet_diag\nunix_diag\nudp_diag\nnetlink_diag\ninet_diag\n" > /etc/modprobe.d/diag.conf
Run echo -e "\nmodprobe tcp_diag\nmodprobe af_packet_diag\nmodprobe unix_diag\nmodprobe udp_diag\nmmodprobe netlink_diag\nmodprobe inet_diag\n" >> /etc/rc.local
COPY scripts /scripts
