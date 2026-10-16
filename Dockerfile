FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /tron ./cmd/tron

FROM alpine:3.23
RUN adduser -D -u 10001 tron
USER tron
COPY --from=build /tron /usr/local/bin/tron
EXPOSE 8080
ENTRYPOINT ["tron"]
CMD ["serve"]
