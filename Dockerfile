# syntax=docker/dockerfile:1

# ---------- Etapa 1: build ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cachear dependencias: solo se re-descargan si cambian go.mod/go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código y compilar
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mesamusic-api .

# ---------- Etapa 2: runtime ----------
FROM alpine:3.20

# certificados TLS necesarios para llamar a la API de YouTube
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/mesamusic-api .

# Render inyecta PORT en runtime; el valor por defecto es solo para local
ENV PORT=8080
EXPOSE 8080

CMD ["./mesamusic-api"]
