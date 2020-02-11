FROM golang:1.12
ENV GO111MODULE=on
WORKDIR /go/src/github.com/lannparty/k8s-cluster-update-controller
COPY ./go.mod ./go.sum ./
COPY cmd cmd
COPY pkg pkg
COPY internal internal
RUN go build cmd/k8s-cluster-update-controller.go

FROM golang:1.12
RUN apt update && apt install ca-certificates -y
COPY --from=0 /go/src/github.com/lannparty/k8s-cluster-update-controller/k8s-cluster-update-controller /usr/local/bin/k8s-cluster-update-controller
ENTRYPOINT /usr/local/bin/k8s-cluster-update-controller
