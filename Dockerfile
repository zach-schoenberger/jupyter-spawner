#FROM golang:1.11.4-alpine3.8 as build-env
FROM golang:1.11
LABEL maintainer="Zach Schoenberger <zschoenb@gmail.com>"

RUN apt-get update -y && apt-get upgrade -y
RUN apt-get install -y python3 ipython python3-pip
RUN pip3 install jupyter

WORKDIR $GOPATH/src/github.com/zach-schoenberger/jupyter-spawner
ENV GO111MODULE=on
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go install -v ./...

EXPOSE 8888

CMD ["jupyter-spawner"]