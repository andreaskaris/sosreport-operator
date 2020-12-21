FROM centos:centos8
RUN yum install sos -y
RUN yum install redhat-support-tool -y
RUN yum install python2 -y
COPY pw_decoder.py /pw_decoder.py
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /pw_decoder.py
RUN chmod +x /entrypoint.sh
