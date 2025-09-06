FROM golang:1.23-alpine AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY backend/ ./backend/
WORKDIR /src/backend
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod tidy && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags='-s -w' -o /out/server ./cmd/server

FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=build /out/server /app/server
COPY terminogrid/ /ui/
EXPOSE 8080
ENTRYPOINT ["/app/server"]
