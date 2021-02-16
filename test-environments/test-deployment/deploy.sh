#!/bin/bash

KUBECTL=""

which kubectl 2>/dev/null
if [ $? -eq 0 ]; then
	KUBECTL="kubectl"
fi
which oc 2>/dev/null
if [ $? -eq 0 ]; then
	KUBECTL="oc"
fi
if [ "$KUBECTL" == "" ]; then
	echo "Cannot find oc or kubectl command"
	exit 1
fi

which yq 2>/dev/null
if ! [ $? -eq 0 ]; then
	echo "Installing yq in /usr/local/bin"
	curl -L -o /usr/local/bin/yq https://github.com/mikefarah/yq/releases/download/v4.5.1/yq_linux_386
	chmod +x /usr/local/bin/yq
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cp ${DIR}/sosreport-pvc.yaml /tmp
cp ${DIR}/sosreport-deployment.yaml /tmp

STORAGE_CLASS=$(kubectl get storageclass | grep default | awk '{print $1}')
yq -i e '.spec.storageClassName = "'$STORAGE_CLASS'"' /tmp/sosreport-pvc.yaml

if [ "$SOSREPORT_IMG" != "" ]; then
	yq -i e '.spec.template.spec.containers[0].image = "'$SOSREPORT_IMG'"' /tmp/sosreport-deployment.yaml
fi

$KUBECTL delete -f /tmp/sosreport-deployment.yaml 2>/dev/null
$KUBECTL delete -f /tmp/sosreport-pvc.yaml 2>/dev/null
$KUBECTL apply -f /tmp/sosreport-pvc.yaml
$KUBECTL apply -f /tmp/sosreport-deployment.yaml


echo "In order to delete this deployment, run:"
echo "$KUBECTL delete -f /tmp/sosreport-deployment.yaml"
echo "$KUBECTL delete -f /tmp/sosreport-pvc.yaml"
