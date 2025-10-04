# Multi-stage build untuk Go application yang ringan
FROM golang:1.24.1-alpine AS builder

# Install dependencies yang diperlukan untuk build
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build aplikasi dengan optimasi
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o main ./main.go

# Final stage - menggunakan distroless image yang sangat ringan
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/main /main

# Expose port
EXPOSE 5000

# Run application
ENTRYPOINT ["/main"]
