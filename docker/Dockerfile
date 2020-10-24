FROM golang:1.14-alpine AS builder
RUN apk update && apk add --no-cache git make gcc libc-dev
WORKDIR /build
COPY . .
RUN make build_static

FROM scratch
COPY --from=builder /build/pod-meta-exporter /bin/pod-meta-exporter
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/pod-meta-exporter"]
