VERSION ?= 1.6.1
IMG ?= maxrocketinternet/k8s-event-logger:$(VERSION)

.PHONY: image

image:
	docker buildx build --platform linux/amd64,linux/arm64 --push -t $(IMG) .
