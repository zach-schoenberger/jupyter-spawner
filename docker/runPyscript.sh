#!/bin/bash

for s in $(cat /home/jovyan/config/params.json | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
    export ${s}
done

ipython /home/jovyan/config/pyScript.py $@ > /home/jovyan/pyscript.log 2>&1
curl -X POST "http://jupyter-spawner.jhub.svc.cluster.local/notebook/end/${REQUEST_ID}" -d "@/home/jovyan/pyscript.log"
exit 0