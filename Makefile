IMG ?= maxrocketinternet/k8s-event-logger
TAG ?= 2.0
PLATFORMS ?= linux/amd64,linux/arm64
BUILDXDRIVER ?= docker-container
WITHSBOM ?= true

.DEFAULT_GOAL := image

.PHONY: all
all: binfmt buildxbuilder image

.PHONY: binfmt
binfmt:
	docker run --privileged --rm tonistiigi/binfmt --install all

.PHONY: buildxbuilder
buildxbuilder:
	docker buildx create --name k8s-event-logger-builder --driver $(BUILDXDRIVER) --platform $(PLATFORMS) --bootstrap

.PHONY: image
image:
	docker buildx build --builder k8s-event-logger-builder --platform $(PLATFORMS) --sbom=$(WITHSBOM) --push -t $(IMG):$(TAG) .

.PHONY: clean
clean:
	-docker rmi $(IMG):$(TAG)
	-docker buildx rm k8s-event-logger-builder
