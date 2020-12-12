#!/bin/bash

sosreport --batch  -k crio.all=on -k crio.logs=on --tmp-dir /host/var/tmp/
