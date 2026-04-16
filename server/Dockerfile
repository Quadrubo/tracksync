# Build
FROM golang:1.26-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /server .

# Main
FROM alpine:3.23

COPY --from=build /server /usr/local/bin/server

WORKDIR /app

EXPOSE 8080
ENTRYPOINT ["server"]
