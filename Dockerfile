FROM golang:1.18 AS build
WORKDIR /go/src/app
COPY *.go go.mod go.sum ./
RUN go build -o app

FROM gcr.io/distroless/base-debian10 AS run
WORKDIR /
COPY --from=build /go/src/app/app /app
COPY config/ /config
COPY html /html
ENTRYPOINT ["/app"]

