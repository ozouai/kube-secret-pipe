FROM golang:1.17.1-alpine as builder
WORKDIR /tmp/build
COPY . . 
WORKDIR /tmp/build/cmd
RUN go build -o /kube-secret-pipe .
FROM alpine
COPY --from=builder /kube-secret-pipe /kube-secret-pipe
ENTRYPOINT ["/kube-secret-pipe"]