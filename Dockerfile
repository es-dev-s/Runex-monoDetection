FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go test -count=1 ./...
RUN CGO_ENABLED=0 go build -o /out/detection-engine ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache git ca-certificates && adduser -D -H detector
COPY --from=build /out/detection-engine /usr/local/bin/detection-engine
USER detector
EXPOSE 8090
ENV DETECTOR_ADDR=0.0.0.0:8090
ENTRYPOINT ["detection-engine"]
