#!/bin/bash
# See:
# https://operator-framework.github.io/community-operators/testing-operators/

VERSION=$1

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

cd ~ 
git clone https://github.com/operator-framework/community-operators.git
cd community-operators/ 
git branch sosreport-operator-${VERSION}
git checkout sosreport-operator-${VERSION}
rm -Rf community-operators/sosreport-operator/$VERSION
mkdir -p community-operators/sosreport-operator/$VERSION
cp -a $DIR/bundle/* community-operators/sosreport-operator/$VERSION/.
