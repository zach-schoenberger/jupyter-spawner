#!/bin/bash

set -e

for s in $(cat /home/jovyan/config/params.json | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
    export ${s}
done

python /home/jovyan/config/pyScript.pyc $@ > /home/jovyan/pyscript.log &>1

if [[ "$?" -eq "1" ]]; then
    curl -X POST http://jupyter-spawner/notebook/end/${REQUEST_ID} -d "@/home/jovyan/pyscript.log"
fi

exit 0