FROM --platform=${BUILDPLATFORM} golang:1.20.5 as builder
ARG TARGETARCH
ARG TARGETOS
WORKDIR /go/src/github.com/max-rocket-internet/k8s-event-logger
COPY . .
RUN go mod vendor
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o k8s-event-logger &&\
    if ldd 'k8s-event-logger'; then exit 1; fi; # Ensure binary is statically-linked
RUN echo "k8s-event-logger:x:10001:10001::/:/bin/false" > /etc_passwd_to_copy

FROM --platform=${TARGETPLATFORM} scratch
COPY --from=builder /etc_passwd_to_copy /go/src/github.com/max-rocket-internet/k8s-event-logger/k8s-event-logger /
ENV USER=k8s-event-logger
USER 10001
ENTRYPOINT ["/k8s-event-logger"]
