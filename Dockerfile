FROM --platform=$BUILDPLATFORM golang:1.27 AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ENV CGO_ENABLED=0
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/tfimpact ./cmd/tfimpact

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source="https://github.com/infraedgecloud-tools/tfimpact" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.description="Explain what a Terraform or OpenTofu plan will do to your AWS resources" \
      org.opencontainers.image.vendor="InfraEdge"
COPY --from=build /out/tfimpact /tfimpact
ENTRYPOINT ["/tfimpact"]
