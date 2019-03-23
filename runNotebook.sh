#!/bin/bash

curl -X POST "http://localhost:8888/notebook/run?uid=zach&adr=localhost&prt=80&frc=true" -F "pyscript=@ipCrossdevice.py" -v