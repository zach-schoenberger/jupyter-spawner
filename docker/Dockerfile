FROM jupyter/pyspark-notebook:latest
USER root
RUN apt-get update
RUN apt-get -y install jq curl
RUN mkdir -p /home/jovyan/bin
ADD runPyscript.sh /home/jovyan/bin

CMD ["/bin/bash", "/home/jovyan/bin/runPyscript.sh"]