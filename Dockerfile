FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/repro ./cmd/repro

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/repro /app/repro
# Os templates são lidos por caminho relativo ("web-ui/templates"),
# então o diretório de execução precisa conter essa pasta.
COPY web-ui/templates /app/web-ui/templates
EXPOSE 8080 9090
CMD ["/app/repro"]
