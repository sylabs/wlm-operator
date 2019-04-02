FROM golang:alpine as builder
RUN apk update
RUN apk add git && apk add ca-certificates
RUN adduser -D -g '' appuser
COPY . $GOPATH/src/github.com/sylabs/slurm-operator/job-companion
WORKDIR $GOPATH/src/github.com/sylabs/slurm-operator/job-companion
RUN CGO_ENABLED=0 go build -o main cmd/main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder $GOPATH/src/github.com/sylabs/slurm-operator/job-companion/main /app/
USER appuser
WORKDIR /app
