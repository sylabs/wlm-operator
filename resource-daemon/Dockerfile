FROM golang:alpine as builder
RUN apk update
RUN apk add git && apk add ca-certificates
RUN adduser -D -g '' appuser
COPY . /usr/local/go/src/github.com/sylabs/slurm-operator/resource-daemon
WORKDIR /usr/local/go/src/github.com/sylabs/slurm-operator/resource-daemon
RUN CGO_ENABLED=0 go build -o main cmd/main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/go/src/github.com/sylabs/slurm-operator/resource-daemon/main /app/
USER appuser
WORKDIR /app
