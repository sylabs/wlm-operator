FROM golang:alpine as builder
RUN apk update
RUN apk add git && apk add ca-certificates
RUN adduser -D -g '' appuser
RUN mkdir $GOPATH/src/job-companion
COPY job-companion/ $GOPATH/src/job-companion
COPY vendor $GOPATH/src/job-companion/vendor
WORKDIR $GOPATH/src/job-companion
RUN ls
RUN CGO_ENABLED=0 go build -o main cmd/main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder $GOPATH/src/job-companion/main /app/
USER appuser
WORKDIR /app
