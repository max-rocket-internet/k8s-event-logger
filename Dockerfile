FROM golang:1.12.1
WORKDIR /go/src/github.com/deliveryhero/k8s-event-logger
COPY main.go .
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
RUN adduser --disabled-login --no-create-home --disabled-password --system --uid 101 non-root
FROM alpine:3.9.3
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY --from=0 /go/src/github.com/deliveryhero/k8s-event-logger/main k8s-event-logger
USER 101
ENV USER non-root
CMD ["/k8s-event-logger"]
