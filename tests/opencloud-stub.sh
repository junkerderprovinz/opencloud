#!/bin/sh
# Stands in for the server binary: init finds a config that already exists, and
# server prints the environment the entrypoint hands over.
case "${1:-}" in
    init) exit 1 ;;
    server) env | sed 's/^/ENV /' ;;
esac
