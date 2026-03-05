# =============================================================================
# Astral Backend - AWS Lambda Container Image
# Multi-stage build for CGO-enabled Go binary with Swiss Ephemeris
# =============================================================================

# -----------------------------------------------------------------------------
# STAGE 1: Builder
# Using Debian-based image with full build toolchain for CGO compilation
# -----------------------------------------------------------------------------
FROM golang:1.24-bookworm AS builder

# Install build dependencies
# - gcc: Required for CGO compilation
# - libc6-dev: C standard library headers
# - make: For building Swiss Ephemeris C library
RUN apt-get update && apt-get install -y \
    gcc \
    libc6-dev \
    make \
    git \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy dependency files first (for layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build Swiss Ephemeris C library
# The .se1 files are small (476KB) - bake them directly into the image
WORKDIR /build/eph/sweph/src
RUN make clean && make libswe.a

# Create directory for ephemeris data
RUN mkdir -p /var/task/ephemeris/

# Copy ephemeris data files to the Lambda working directory
RUN cp ephe/*.se1 /var/task/ephemeris/
RUN cp sefstars.txt /var/task/

# Return to build root
WORKDIR /build

# Build the Lambda binary with CGO enabled
# Critical flags:
# - CGO_ENABLED=1: Required for C library bindings
# - GOOS=linux: Target Linux (Lambda execution environment)
# - GOARCH=amd64: Standard Lambda architecture
# - ldflags '-w -s': Strip debug info for smaller binary
# - extldflags '-static': Static linking where possible
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags '-w -s -linkmode external -extldflags "-static"' \
    -tags lambda.norpc \
    -o /var/task/bootstrap \
    ./cmd/lambda/main.go

# Verify the binary was created
RUN ls -la /var/task/

# -----------------------------------------------------------------------------
# STAGE 2: Runtime
# Using Amazon Linux 2023 - the official Lambda base image
# -----------------------------------------------------------------------------
FROM public.ecr.aws/lambda/provided:al2023

# Lambda working directory is /var/task/ by default
# The 'provided' runtime looks for a file named 'bootstrap' to execute

# Copy the compiled binary from builder - MUST be at /var/task/bootstrap
COPY --from=builder /var/task/bootstrap /var/task/bootstrap

# Copy ephemeris data files
# Swiss Ephemeris expects .se1 files in /var/task/ephemeris/
COPY --from=builder /var/task/ephemeris/ /var/task/ephemeris/

# Copy fixed stars data (optional, for star calculations)
COPY --from=builder /var/task/sefstars.txt /var/task/

# Ensure the bootstrap binary is executable
RUN chmod +x /var/task/bootstrap

# Verify files are present
RUN ls -la /var/task/ && \
    ls -la /var/task/ephemeris/ && \
    echo "Bootstrap and ephemeris files loaded successfully"

# Set the entrypoint to the bootstrap
ENTRYPOINT ["/var/task/bootstrap"]
