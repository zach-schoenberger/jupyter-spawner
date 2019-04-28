#!/bin/bash
function help() {
    echo "Usage: createAssessor.sh {config file}"
}

if [[ "x${1}" == "x" ]]; then
    help
    exit 1
fi

FILE=$1

kubectl --namespace jhub create configmap assessor --from-literal pyScriptAssessor.py=$( cat ${FILE} | base64 )
