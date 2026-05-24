# syntax=docker/dockerfile:1.7

# --- Build stage --------------------------------------------------------
# Compiles a fully static binary so the runtime image needs no glibc.
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache module downloads independently from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary; -ldflags strips debug info.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/server \
    ./cmd/server

# --- Runtime stage ------------------------------------------------------
# Distroless gives us a minimal, non-root, CA-trusted base.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/server /app/server

ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
