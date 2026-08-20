# ---- builder ----
FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS builder
WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn \
    GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/reins ./.

# ---- runtime ----
FROM docker.m.daocloud.io/library/alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/reins /app/reins
COPY --from=builder /src/internal/webfs/web /app/web
ENV REINS_DSN=/data/reinsurance.db
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/reins"]
CMD ["--addr=:8080", "--dsn=/data/reinsurance.db"]
