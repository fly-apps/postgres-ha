#!/bin/bash

set -e

# Setup for semaphore ci

function switch_to_go16() {
	sudo ln -fs /usr/local/golang/1.6.0/go/bin/go /usr/local/bin/go
	# remove other Go version from path
	export PATH=`echo $PATH | sed -e 's|:/usr/local/golang/.*/go/bin||'`
	# setup GOROOT
	export GOROOT="/usr/local/golang/1.6.0/go"
	# add new go installation to PATH
	export PATH="$PATH:/usr/local/golang/1.6.0/go/bin"
}


if [ "${CI}" != "true" -o "${SEMAPHORE}" != "true" ]; then
	echo "not on semaphoreci"
	exit 1
fi

# Install and start etcd
mkdir etcd
pushd etcd
curl -L https://github.com/coreos/etcd/releases/download/v2.2.1/etcd-v2.2.1-linux-amd64.tar.gz -o etcd-v2.2.1-linux-amd64.tar.gz
tar xzvf etcd-v2.2.1-linux-amd64.tar.gz
popd

# Install and start etcd
mkdir consul
pushd consul
curl -L https://releases.hashicorp.com/consul/0.6.3/consul_0.6.3_linux_amd64.zip -o consul_0.6.3_linux_amd64.zip
unzip consul_0.6.3_linux_amd64.zip
popd

# Install go 1.6
# TODO(sgotti) remove when semaphore image will provide it
curl -L  https://storage.googleapis.com/golang/go1.6.linux-amd64.tar.gz -o /tmp/go1.6.linux-amd64.tar.gz
sudo mkdir /usr/local/golang/1.6.0
pushd /usr/local/golang/1.6.0
sudo tar xvfz /tmp/go1.6.linux-amd64.tar.gz
popd

switch_to_go16

# Precompile stdlib with cgo disable to speedup builds
sudo -E CGO_ENABLED=0 go install -a -installsuffix cgo std

# Run tests
export ETCD_BIN="${PWD}/etcd/etcd-v2.2.1-linux-amd64/etcd"
export CONSUL_BIN="${PWD}/consul/consul"
export PATH=/usr/lib/postgresql/9.4/bin/:$PATH ; INTEGRATION=1 ./test
