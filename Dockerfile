# Build the manager binary
FROM golang:1.17-alpine3.15 as  builder

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

ENV GOPROXY=https://proxy.golang.com.cn,direct

WORKDIR /workspace

# Copy the go source
COPY / ./

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o example-beego-opensergo

FROM alpine:3.15

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /
COPY --from=builder /workspace/example-beego-opensergo .
COPY --from=builder /workspace/conf ./conf
COPY --from=builder /workspace/start.sh /

ENTRYPOINT ["/start.sh"]
