FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/buatpostingan ./cmd/app

FROM nginx:1.27-alpine

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/buatpostingan /usr/local/bin/buatpostingan
COPY web /app/web
COPY deploy/public /app/homepage
COPY resources /app/resources
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY deploy/start.sh /usr/local/bin/start-buatpostingan
RUN chmod +x /usr/local/bin/start-buatpostingan \
    && mkdir -p /app/storage/webchat /app/storage/pages

EXPOSE 1212
ENTRYPOINT ["/usr/local/bin/start-buatpostingan"]
