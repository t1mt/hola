# Building stage
FROM golang:1.14-alpine3.11 as builder

WORKDIR /go/src/github.com/t1mt/hola

# Source code, building tools and dependences
COPY . /go/src/github.com/t1mt/hola

ENV CGO_ENABLED 0
ENV GOOS linux
ENV GO111MODULE=on

ENV TIMEZONE "Asia/Shanghai"

RUN make go.build.linux_amd64.consumer
RUN mv linux/amd64/consumer /go/bin

# Production stage
FROM alpine:3.9

WORKDIR /go/bin

# copy the go binaries from the building stage
COPY --from=builder /go/bin /go/bin

EXPOSE 5555
ENTRYPOINT ["./consumer"]
