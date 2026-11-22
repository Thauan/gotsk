FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/repro ./cmd/repro

FROM alpine:3.20
# ca-certificates: necessário para falar com a AWS (DynamoDB) por HTTPS.
RUN apk add --no-cache ca-certificates && mkdir -p /data
WORKDIR /app
COPY --from=build /out/repro /app/repro
ENV SQLITE_PATH=/data/gotsk.db
VOLUME ["/data"]
EXPOSE 8080 9090
CMD ["/app/repro"]
