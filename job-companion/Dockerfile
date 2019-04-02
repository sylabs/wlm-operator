FROM golang:alpine as builder
RUN apk update
RUN apk add git
RUN apk add ca-certificates
RUN apk add make
RUN adduser -D -g '' appuser
COPY . /usr/local/go/src/github.com/sylabs/slurm-operator/job-companion
WORKDIR /usr/local/go/src/github.com/sylabs/slurm-operator/job-companion
RUN make

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/go/src/github.com/sylabs/slurm-operator/job-companion/bin/* /app/
USER appuser
WORKDIR /app
ENTRYPOINT ["./job-companion"]
