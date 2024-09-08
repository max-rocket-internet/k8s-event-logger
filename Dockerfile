FROM --platform=${BUILDPLATFORM} golang:1.23 as builder
ARG TARGETARCH
ARG TARGETOS
WORKDIR /go/src/github.com/max-rocket-internet/k8s-event-logger
COPY . .
RUN go mod vendor
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o k8s-event-logger &&\
    if ldd 'k8s-event-logger'; then exit 1; fi; # Ensure binary is statically-linked

FROM --platform=${TARGETPLATFORM} scratch
COPY --from=builder /go/src/github.com/max-rocket-internet/k8s-event-logger/k8s-event-logger /
USER 10001
ENTRYPOINT ["/k8s-event-logger"]
