FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot \
    && GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/bot /usr/local/bin/bot
COPY --from=build /out/goose /usr/local/bin/goose
COPY --from=build --chown=app:app /src/migrations /app/migrations
USER app
ENTRYPOINT ["/usr/local/bin/bot"]
