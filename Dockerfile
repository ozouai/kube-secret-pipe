FROM golang:1.17.1-alpine as builder
WORKDIR /tmp/build
COPY . . 
RUN go build -o /server .
FROM alpine
COPY --from=builder /server /server
CMD ["/server"]