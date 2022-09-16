IMG ?= maxrocketinternet/k8s-event-logger
TAG ?= 1.6
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: image

image:
	docker buildx build --platform $(PLATFORMS) --push -t $(IMG):$(TAG) .
