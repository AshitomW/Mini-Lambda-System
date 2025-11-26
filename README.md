# Mini Lambda System

It’s the kind of minimalist “serverless-but-not-really” toy that spins up containers on demand, pretends to orchestrate things, and confidently behaves like it belongs in a cloud brochure. It fetches images, fires them up, returns results, and then casually wanders off like it just performed a miracle. No promises, no guarantees, just vibes, enthusiasm, and the sheer audacity to call itself an execution platform.
Think of it as Lambda's chaotic younger sibling who runs on coffee, pulls random images on demand, skips production readiness checks, and proudly says, "Scaling? Never heard of her."

## Features

- Function registration and management
- Docker-based function execution
- Synchronous function invocation via RESTful API
- Async invocation infrastructure
- Prometheus metrics integration with invocation counters and duration histograms
- Timeout handling with configurable timeouts
- JSON-based function persistence
- Docker image upload support
- Concurrent function execution support

## Prerequisites

- Go 1.24.5 or higher
- Docker Engine running

## Installation

1. Clone the repository:

```bash
git clone <your-repo-url>
cd Mini-Lambda-System
```

2. Install dependencies:

```bash
go mod tidy
```

3. Build and run the server:

```bash
go run .
```

The server will start on port 8300.

## Quick Start Example

### 1. Build the Sample Function

First, let's build the included Python function:

```bash
# Navigate to the function directory
cd function

# Build the Docker image
docker build -f hello-image.dockerfile -t hello-python .

# Go back to root directory
cd ..
```

### 2. Register the Function

```bash
curl -X POST http://localhost:8300/functions \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hello-python",
    "image": "hello-python"
  }'
```

Response:

```json
{
  "id": "5ac94553-cd05-4cc0-b657-acae0c6559e1",
  "name": "hello-python",
  "image": "hello-python",
  "created_at": "2025-01-16T16:24:48.719407+05:45"
}
```

### 3. List All Functions

```bash
curl http://localhost:8300/functions
```

### 4. Invoke the Function

```bash
curl -X POST http://localhost:8300/invoke/5ac94553-cd05-4cc0-b657-acae0c6559e1 \
  -H "Content-Type: application/json" \
  -d '{
    "event": {
      "message": "Hello from Mini Lambda!"
    },
    "timeout": 30
  }'
```

Response:

```json
{
  "result": "{\"message\": \"Processed: Hello from Mini Lambda!\", \"input_received\": {\"message\": \"Hello from Mini Lambda!\"}}\n",
  "duration": 1250,
  "timestamp": "2025-01-16T16:30:15.123Z"
}
```

### 5. Upload Docker Images

You can upload Docker images directly to the system:

```bash
# Save your Docker image to a tar file
docker save my-function:latest > my-function.tar

# Upload the image
curl -X POST http://localhost:8300/images \
  -F "image=@my-function.tar"
```

### 6. List Available Docker Images

```bash
curl http://localhost:8300/images
```

## API Documentation

### Register Function

- **POST** `/functions`
- **Body**:
  ```json
  {
    "name": "function-name",
    "image": "docker-image-name"
  }
  ```

### List Functions

- **GET** `/functions`
- Returns array of all registered functions

### Invoke Function (Synchronous)

- **POST** `/invoke/{function-id}`
- **Body**:
  ```json
  {
    "event": {}, // Any JSON payload
    "timeout": 120 // Timeout in seconds (optional, default: 120)
  }
  ```
- Returns immediate results with output, duration, and timestamp

### Upload Docker Image

- **POST** `/images`
- **Form Data**:
  - `image`: Docker image tar file
- Uploads and loads a Docker image into the local Docker registry

### List Available Docker Images

- **GET** `/images`
- Returns array of all available Docker images in the local registry

### Metrics

- **GET** `/metrics`
- Returns Prometheus metrics including:
  - `Total_Invocations`: Counter of function invocations by function name
  - `Invocation Duration ms`: Histogram of invocation durations in milliseconds

## Architecture Overview

### Function Execution Flow

1. **Registration**: Functions are registered with a name and Docker image
2. **Invocation**: When invoked, the system:
   - Creates a new Docker container from the specified image
   - Passes the JSON payload via stdin
   - Captures stdout/stderr as output and logs
   - Measures execution duration
   - Records metrics
   - Cleans up the container

### Async Infrastructure

The system includes backend support for asynchronous invocations with:

- **Invocation Status Tracking**: PENDING → RUNNING → COMPLETED/FAILED
- **Result Storage**: Output, logs, duration, and error information
- **Concurrent Execution**: Multiple functions can run simultaneously
- **Thread-Safe Operations**: Mutex-protected invocation management

_Note: Async API endpoints are not yet implemented but the infrastructure is ready._

## Creating Custom Functions

### Python Example

1. Create your function file (`my-function.py`):

```python
import json
import sys
from datetime import datetime

# Read input from stdin
input_data = sys.stdin.read()
event = json.loads(input_data) if input_data.strip() else {}

# Your function logic here
result = {
    "message": f"Hello {event.get('name', 'World')}!",
    "timestamp": str(datetime.now())
}

# Output result as JSON
print(json.dumps(result))
```

2. Create Dockerfile (`my-function.dockerfile`):

```dockerfile
FROM python:3.9-alpine
COPY my-function.py /app/function.py
WORKDIR /app
CMD ["python3", "function.py"]
```

3. Build and register:

```bash
docker build -f my-function.dockerfile -t my-function .

curl -X POST http://localhost:8300/functions \
  -H "Content-Type: application/json" \
  -d '{"name": "my-function", "image": "my-function"}'
```

### Node.js Example

1. Create `package.json`:

```json
{
  "name": "node-function",
  "version": "1.0.0",
  "main": "index.js"
}
```

2. Create `index.js`:

```javascript
let input = "";
process.stdin.on("data", (chunk) => (input += chunk));
process.stdin.on("end", () => {
  const event = input ? JSON.parse(input) : {};

  const result = {
    message: `Processed: ${event.message || "No message"}`,
    nodeVersion: process.version,
  };

  console.log(JSON.stringify(result));
});
```

3. Create Dockerfile:

```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package.json index.js ./
CMD ["node", "index.js"]
```

## Function Requirements

Your Docker function must:

1. Read JSON input from stdin
2. Output JSON result to stdout
3. Exit with code 0 on success
4. Handle empty/invalid input gracefully

## Monitoring & Metrics

Access Prometheus metrics at `http://localhost:8300/metrics` to monitor:

- **Total Invocations**: `Total_Invocations` counter by function name
- **Invocation Duration**: `Invocation Duration ms` histogram with linear buckets (10ms-1000ms)
- **Function Performance**: Per-function execution statistics

Example metrics output:

```
# HELP Total_Invocations Number of function invocations
# TYPE Total_Invocations counter
Total_Invocations{function="hello-python"} 5

# HELP Invocation Duration ms Invocation latency in ms
# TYPE Invocation Duration ms histogram
Invocation Duration ms_bucket{function="hello-python",le="10"} 0
Invocation Duration ms_bucket{function="hello-python",le="110"} 2
```

## Troubleshooting

### Function Not Found

- Ensure the function ID exists by calling `/functions`
- Check that the function was properly registered

### Container Execution Issues

- Verify Docker is running: `docker ps`
- Check that the Docker image exists: `docker images`
- Ensure the Docker image is executable and has proper CMD/ENTRYPOINT

### Timeout Issues

- Increase the timeout parameter in your invoke request
- Check function logs for performance bottlenecks
- Monitor execution time via `/metrics`

### Image Upload Issues

- Ensure the uploaded file is a valid Docker tar export
- Check file size limits and available disk space
- Verify the tar file was created with `docker save`

## Future Enhancements

- REST API endpoints for async invocation management
- Function versioning support
- Resource limits and quotas
- Log aggregation and search
- Function scaling and load balancing
- Authentication and authorization
- Function marketplace/registry
