# Package Testing

This directory contains Docker-based tests to verify ocuroot package installations across different architectures.

## Test Structure

```
test/
├── Dockerfile.amd64     # AMD64 package verification
├── Dockerfile.arm64     # ARM64 package verification
├── test-packages.sh     # Automated test runner
└── README.md           # This file
```

## Running Tests

### Automated Testing
Run all package tests:
```bash
./distribution/test/test-packages.sh [version]
```

This will:
1. Verify packages exist in `.build/packages/`
2. Build and test AMD64 package installation
3. Build and test ARM64 package installation
4. Report results

### Manual Testing
Test individual architectures:
```bash
# Test AMD64 package
cd distribution/test
docker build -f Dockerfile.amd64 -t ocuroot-test-amd64 .

# Test ARM64 package  
docker build -f Dockerfile.arm64 -t ocuroot-test-arm64 .

# Run interactive tests
docker run -it --rm ocuroot-test-amd64
docker run -it --rm ocuroot-test-arm64
```

## Test Coverage

Each test verifies:
- ✅ Package installation via `dpkg`
- ✅ Binary exists at `/usr/bin/ocuroot`
- ✅ Binary has correct architecture
- ✅ Binary is executable
- ✅ `ocuroot --help` works
- ✅ `ocuroot version` works

## Prerequisites

1. Build packages first:
   ```bash
   ./distribution/build-packages.sh
   ```

2. Ensure Docker supports multi-platform builds:
   ```bash
   docker buildx ls
   ```
