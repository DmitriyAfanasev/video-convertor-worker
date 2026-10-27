FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/server ./cmd/server
COPY internal ./internal
COPY index.html ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/server /app/server
EXPOSE 8080
VOLUME ["/app/data"]
ENV DATA_DIR=/app/data
CMD ["/app/server"]
