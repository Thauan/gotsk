FROM golang:1.24-alpine AS build
WORKDIR /src
# O repositório é multi-módulo: cmd/repro usa replace para os módulos locais,
# então o contexto inteiro precisa estar presente no build.
COPY . .
WORKDIR /src/cmd/repro
RUN CGO_ENABLED=0 go build -o /out/repro .

FROM alpine:3.20
# ca-certificates: necessário para falar com a AWS (DynamoDB) por HTTPS.
RUN apk add --no-cache ca-certificates && mkdir -p /data
WORKDIR /app
COPY --from=build /out/repro /app/repro
ENV SQLITE_PATH=/data/gotsk.db
VOLUME ["/data"]
EXPOSE 8080 9090
CMD ["/app/repro"]
