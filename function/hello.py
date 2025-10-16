
import json
import sys

# Read from stdin
input_data = sys.stdin.read()
try:
    event = json.loads(input_data) if input_data.strip() else {}
except:
    event = {}

# Process and output
result = {
    "message": f"Processed: {event.get('message', 'No message')}",
    "input_received": event
}

print(json.dumps(result))
