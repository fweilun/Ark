# README: Build and run the Ark API server

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ark-api ./cmd/ark-api

FROM alpine:3.19
WORKDIR /app
COPY --from=build /out/ark-api /app/ark-api
EXPOSE 8080
ENV ARK_HTTP_ADDR=:8080
CMD ["/app/ark-api"]
