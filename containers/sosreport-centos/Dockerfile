FROM centos:centos8
RUN yum install sos -y
RUN yum install redhat-support-tool -y
RUN yum install python2 -y
RUN yum install nfs-utils -y
RUN yum install lftp -y
RUN yum install sssd -y
RUN yum update -y
Run echo -e "tcp_diag\naf_packet_diag\nunix_diag\nudp_diag\nnetlink_diag\ninet_diag\n" > /etc/modules-load.d/diag.conf
COPY scripts /scripts
