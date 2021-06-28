ARG PSCALE_VERSION=v0.51.0

FROM golang:1.16 as build
WORKDIR /app
COPY . .

ARG VERSION
ARG COMMIT
ARG DATE

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X main.commit=$COMMIT -X main.version=$VERSION -X main.date=$DATE" github.com/jharlap/good-day-app

FROM planetscale/pscale:${PSCALE_VERSION}
EXPOSE 80

WORKDIR /app
COPY --from=build /app/good-day-app /usr/bin
ENV DATABASE_DNS=root@127.0.0.1:3306/good-day
ENTRYPOINT ["/usr/bin/pscale", "connect", "good-day", "main", "--execute", "/usr/bin/good-day-app"]
