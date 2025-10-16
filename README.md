# Mini Lambda System

A lightweight serverless function execution system built with Go that runs functions in Docker containers. Similar to AWS Lambda but simplified for local development and testing.

## Features

- Function registration and management
- Docker-based function execution
- RESTful API for function invocation
- Prometheus metrics integration
- Timeout handling
- JSON-based function persistence

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

### Invoke Function

- **POST** `/invoke/{function-id}`
- **Body**:
  ```json
  {
    "event": {}, // Any JSON payload
    "timeout": 120 // Timeout in seconds (optional, default: 120)
  }
  ```

### Metrics

- **GET** `/metrics`
- Returns Prometheus metrics

## Creating Custom Functions

### Python Example

1. Create your function file (`my-function.py`):

```python
import json
import sys

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

## Monitoring

Access Prometheus metrics at `http://localhost:8300/metrics` to monitor:

- Total function invocations
- Invocation duration
- Function-specific metrics

## Configuration

- **Port**: 8300 (hardcoded in main.go)
- **Default Timeout**: 120 seconds
- **Data Directory**: `./function/` (for persistence)
- **Max Container Runtime**: Based on timeout parameter

## Troubleshooting

### Function Not Found

- Ensure the function ID exists by calling `/functions`
- Check that the function was properly registered

### Container Execution Issues

- Verify Docker is running
- Check that the Docker image exists: `docker images`
- Ensure the Docker image is executable

### Timeout Issues

- Increase the timeout parameter in your invoke request
- Check function logs for performance bottlenecks

## Development

To modify the system:

1. Edit the Go source files
2. Restart the server: `go run .`
3. Test with the provided examples
