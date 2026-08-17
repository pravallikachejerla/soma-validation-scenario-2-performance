# Multi-stage Dockerfile for the Go pricing service.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /out/api ./cmd/api
RUN go build -o /out/migrate ./cmd/migrate
RUN go build -o /out/seed ./cmd/seed
RUN go build -o /out/worker ./cmd/worker
RUN go build -o /out/benchmark ./cmd/benchmark

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/seed /app/seed
COPY --from=build /out/worker /app/worker
COPY --from=build /out/benchmark /app/benchmark
COPY migrations /app/migrations
EXPOSE 8080
CMD ["/app/api"]
