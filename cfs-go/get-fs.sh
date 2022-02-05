#!/bin/bash
# this script uses Docker to get a simple ubuntu filesystem for the container to use
docker run -d --rm --name ubuntufs ubuntu:20.04 sleep 1000
docker export ubuntufs -o ubuntufs.tar
docker stop ubuntufs
mkdir -p /container/ubuntu-fs
tar xf ubuntufs.tar -C /container/ubuntu-fs
