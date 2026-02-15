#!/bin/sh

# source: https://github.com/ollama/ollama/issues/1890
ollama list | tail -n +2 | awk '{print $1}' | while read -r model; do
  echo $model
  ollama pull $model
done
