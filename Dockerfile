
# build stage
FROM golang:alpine AS builder
ENV GO111MODULE=on
RUN apk add --no-cache git
WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-extldflags "-static"' -o migration -v

# final stage
FROM scratch
COPY --from=builder /go/src/app/migration /migration
LABEL Name=migration Version=0.0.1
ENTRYPOINT ["/migration"]
