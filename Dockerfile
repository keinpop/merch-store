# build stage
FROM golang:1.23.1-alpine as builder
WORKDIR /build
COPY ./go.mod .
RUN go mod download
COPY ./ .
RUN go build -o /main cmd/main.go

# run stage
FROM alpine
COPY --from=builder main /bin/main
COPY ./config/config.yaml /app/config/config.yaml
WORKDIR /app
ENTRYPOINT [ "/bin/main" ]