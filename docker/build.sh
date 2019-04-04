#!/bin/bash

docker build -t zschoenb/jhub-tester:1.0 .
docker push zschoenb/jhub-tester:1.0
