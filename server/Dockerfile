FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -ldflags "-s -w" -o /bin/cloudguard ./cmd/cloudguard

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/cloudguard /usr/local/bin/cloudguard
COPY configs/cloudguard-docker.yaml /etc/cloudguard/cloudguard.yaml

VOLUME ["/data"]

EXPOSE 8081

ENTRYPOINT ["cloudguard"]
CMD ["--config", "/etc/cloudguard/cloudguard.yaml"]
