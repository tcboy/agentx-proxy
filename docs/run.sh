#!/bin/bash

while true; do
    claude --dangerously-skip-permissions \
           -p "$(cat PROMPT.md)"
done