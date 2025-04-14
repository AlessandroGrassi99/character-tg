# --- Builder Stage ---
ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-alpine AS builder

# Install git (needed for go mod download potentially) and build-base (needed if CGO_ENABLED=1)
# Using --no-cache keeps the layer smaller
RUN apk add --no-cache git build-base

WORKDIR /usr/src/app

# Copy go module files first to leverage Docker cache
COPY go.mod go.sum ./
# Download dependencies - This will now use Go 1.24
RUN go mod download && go mod verify

# Copy the rest of the source code
COPY . .

# Build the Go application. Go 1.24 is available.
# Decide if you need CGO enabled or disabled based on your project's needs.
# Option 1: CGO Enabled (Requires build-base installed above)
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -v -o /app/run-app .
# Option 2: CGO Disabled (Simpler if your project doesn't need C libs)
# RUN CGO_ENABLED=0 go build -ldflags="-w -s" -v -o /app/run-app .


# --- Final Stage ---
FROM alpine:3.21 AS final

# If CGO_ENABLED=1 was used above, you *might* need runtime libs for linked C code.
# The binary is built against musl libc (present in alpine base).
# libc6-compat might be needed if any glibc specifics were indirectly linked (less common with go builds on alpine)
# RUN apk add --no-cache libc6-compat
# Add any other specific C libraries your application dynamically links against:
# RUN apk add --no-cache libpq sqlite-libs # Example
RUN apk add --no-cache ca-certificates

# Create a non-root user and group for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/run-app /app/run-app

# Ensure the binary is executable by the user
RUN chown appuser:appgroup /app/run-app

# Switch to the non-root user
USER appuser

# Define the command to run the application
CMD ["/app/run-app"]
