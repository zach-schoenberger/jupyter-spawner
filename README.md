# JupyterSpawner
## Build
To build, checkout this repository and in the checked out directory run
```bash
go install
```
## Build Docker Image
To build the `JupyterSpawner` docker image, in the checked out directory run
```bash
./build.sh
```
It is recommended that you update the image and tag info for your specific organization.

## Build Assessor Docker Image
The assessor docker image is the image used to run and assess the submitted python scripts. To build the assessor docker container, from the checked out directory run
```bash
./docker/build.sh
```
It is recommended that you update the Dockerhub image and tag info for your specific organization.
This new image and tag should be used in the `jspawner.yml` as follows:
```yaml
job:
    Image: "{organization}/{repo}:{version}"
```

## Create Assessor
The `assessor` python script is the script that is run inside of the `assessor` docker container. 
It should be updated to validate the submitted python scripts appropriately. The `ASSIGNMENT` environment variable inside the container
as it is running is the value of the `asnmt` parameter sent by the `submitter.py` script. This allows you to determine how the submission should
be assessed.