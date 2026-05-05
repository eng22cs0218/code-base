#!/bin/bash

CONTEXT=$1
OUTPUT_PATH="${2:-$HOME/config.yaml}"

if [ -z "$CONTEXT" ]; then
  echo "❌ Usage: <script> <context> [output_path]"
  exit 1
fi

echo "🔍 Validating context: $CONTEXT"

kubectl config get-contexts "$CONTEXT" >/dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "❌ Context '$CONTEXT' not found"
  exit 1
fi

echo "📦 Exporting kubeconfig..."

kubectl config view --minify --context="$CONTEXT" --raw > "$OUTPUT_PATH"

if [ $? -eq 0 ]; then
  echo "✅ Saved to $OUTPUT_PATH"
else
  echo "❌ Failed to export kubeconfig"
  exit 1
fi