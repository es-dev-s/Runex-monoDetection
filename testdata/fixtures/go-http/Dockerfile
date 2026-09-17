FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wfm-api ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/wfm-api /wfm-api
EXPOSE 18780
ENV ADDR=:18780
ENTRYPOINT ["/wfm-api"]
