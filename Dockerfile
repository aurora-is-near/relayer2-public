FROM golang:alpine as build
ARG VERSION

RUN apk add --no-cache git build-base

WORKDIR /app

# Generate build.info file
RUN echo "version: ${VERSION}" > build.info && \
    echo "build_time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")" >> build.info

# Copy go mod and sum files
COPY go.mod go.sum ./

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:3.17

WORKDIR /app

RUN apk add --no-cache ca-certificates curl

COPY --from=build /app/build.info /version
COPY --from=build /app/app /app/app

