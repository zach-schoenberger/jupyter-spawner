#!/bin/bash

for s in $(cat /home/jovyan/submission/params.json | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
    export ${s}
done

export PYTHONPATH=/home/jovyan/assessor:/home/jovyan/submission:$PYTHONPATH

ipython /home/jovyan/assessor/pyScriptAssessor.py $@ > /home/jovyan/pyscript.log 2>&1
curl -X POST "http://jupyter-spawner.jhub2.svc.cluster.local/notebook/end/${REQUEST_ID}" -d "@/home/jovyan/pyscript.log"
exit 0