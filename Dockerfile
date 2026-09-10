# Compilación en una imagen con toolchain, ejecución en una imagen mínima.
# El binario resultante es estático: la imagen final no necesita libc ni shell.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /api
# Ejecuta como usuario sin privilegios (uid 65532), nunca como root.
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/api"]
