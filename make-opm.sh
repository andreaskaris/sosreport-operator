#!/bin/bash

cd ~
git clone https://github.com/operator-framework/operator-registry.git
cd operator-registry
make
cp bin/opm /usr/local/bin/opm
