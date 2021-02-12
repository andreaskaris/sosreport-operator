#/bin/bash

# TODO: auto-determine default interface and IP address for cert generation
IP_ADDRESS="192.168.0.100"

cd /tmp
mkdir CA
cd CA
openssl genrsa -out rootCA.key 4096
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 10240 -out rootCA.crt -subj "/C=CA/ST=Arctica/L=Northpole/O=Acme Inc/OU=DevOps/CN=www.example.com/emailAddress=dev@www.example.com"
sudo cp rootCA.crt  /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust extract

mkdir certificates
cd certificates
cat<<'EOF'>config
[ req ]
distinguished_name = req_distinguished_name
prompt = no
req_extensions = v3_req

[ req_distinguished_name ]
C="DE"
ST="NRW"
L="Dusseldorf"
O="Acme Inc."
CN="${IP_ADDRESS}"
emailAddress="admin@example.com"

[ v3_req ]

#basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
EOF
cat <<EOF>>config
DNS.1 = ${IP_ADDRESS}
IP.1 = ${IP_ADDRESS}
EOF
openssl genrsa -out domain.key 4096
openssl req -new -key domain.key -nodes -out domain.csr -config config

openssl req -in domain.csr -noout -text | grep -i dns
openssl x509 -req -in domain.csr -CA ../rootCA.crt -CAkey ../rootCA.key -CAcreateserial -out domain.crt -days 3650 -sha256 -extensions v3_req -extfile config
openssl x509 -in domain.crt -noout -text | grep IP
openssl verify -verbose domain.crt

openssl verify -CAfile ../rootCA.crt domain.crt

sudo mkdir -p /opt/registry/{auth,certs,data}
sudo cp domain.key  /opt/registry/certs/
sudo cp domain.crt  /opt/registry/certs/

sudo podman run --name mirror-registry \
  -p 5000:5000 -v /opt/registry/data:/var/lib/registry:z      \
  -v /opt/registry/certs:/certs:z      \
  -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt      \
  -e REGISTRY_HTTP_TLS_KEY=/certs/domain.key      \
  -d docker.io/library/registry:2

podman generate systemd --name mirror-registry > /etc/systemd/system/mirror-registry-container.service
systemctl daemon-reload
systemctl enable --now mirror-registry-container

openssl s_client -connect ${IP_ADDRESS}:5000 | cat
curl https://${IP_ADDRESS}:5000/v2/_catalog
