#!/bin/sh
# Compatibility entry point; common actions live in Makefile.
exec make -C "$(dirname "$0")/.." check
