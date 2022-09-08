FROM --platform=${BUILDPLATFORM} golang:1.18.3 as builder
ARG TARGETARCH
ARG TARGETOS
WORKDIR /go/src/github.com/max-rocket-internet/k8s-event-logger
COPY . .
RUN go get
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o k8s-event-logger
RUN adduser --disabled-login --no-create-home --disabled-password --system --uid 101 non-root

FROM --platform=${TARGETPLATFORM} alpine:3.15.4
RUN addgroup -S non-root && adduser -S -G non-root non-root
USER 101
ENV USER non-root
COPY --from=builder /go/src/github.com/max-rocket-internet/k8s-event-logger/k8s-event-logger k8s-event-logger
CMD ["/k8s-event-logger"]
