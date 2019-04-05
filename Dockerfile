#FROM golang:1.11.4-alpine3.8 as build-env
FROM golang:1.11
LABEL maintainer="Zach Schoenberger <zschoenb@gmail.com>"

WORKDIR $GOPATH/src/github.com/zach-schoenberger/jupyter-spawner
ENV GO111MODULE=on
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go install -v ./...

EXPOSE 8080

CMD ["jupyter-spawner"]