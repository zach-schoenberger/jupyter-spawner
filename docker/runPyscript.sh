#!/bin/bash

set -e

for s in $(cat /home/jovyan/config/params.json | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
    export ${s}
done

python /home/jovyan/config/pyScript.py $@

if [[ "$?" -eq "1" ]]; then
    curl http://jupyter-spawner/notebook/end/${REQUEST_ID}
fi

exit 0