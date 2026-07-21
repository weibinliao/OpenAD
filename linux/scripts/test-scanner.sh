#!/bin/bash
echo "=== Linux Folder Scanner ==="

if ! command -v node &> /dev/null; then
    echo "ERROR: Node.js required"
    exit 1
fi

echo "Testing scanner..."
node scanner.js "/tmp" 1