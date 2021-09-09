# Lightweight Alpine-based
FROM golang:1.15.5-alpine3.12

# Build ARGS
ARG VERSION="latest-alpine-3.12"

RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go build -v -ldflags "-s -w -X main.programVersion=${VERSION}"

# Multi-stage build: only copy build result and resources
FROM alpine:3.12

LABEL original_developer="ICPAC" \
    contributor="Erick Otenyo<otenyo.erick@gmail.com>" \
    vendor="ICPAC" \
	url="https://icpac.net" \
	os.version="3.12"

RUN apk --no-cache add ca-certificates && mkdir /app
WORKDIR /app/
COPY --from=0 /app/timeseries-mbgl-maps /app/
COPY --from=0 /app/fonts /app/fonts

VOLUME ["/config"]

USER 1001
EXPOSE 7800

ENTRYPOINT ["/app/timeseries-mbgl-maps"]
CMD []