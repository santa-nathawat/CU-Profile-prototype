FROM golang:1.27.3-alpine AS source
WORKDIR /app
COPY apps/api .

FROM source AS development
RUN go install github.com/air-verse/air@v1.67.4
CMD ["air", "--build.cmd", "go build -o /tmp/api ./cmd/api", "--build.entrypoint", "/tmp/api", "--build.stop_on_error", "true"]

FROM source AS build
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /api ./cmd/api

FROM source AS seed-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /seed ./cmd/seed

FROM alpine:3.23 AS seed
COPY --from=seed-build /seed /seed
USER 65532:65532
ENTRYPOINT ["/seed"]

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates
COPY --from=build /api /api
USER 65532:65532
ENV PORT=3001
EXPOSE 3001
ENTRYPOINT ["/api"]
