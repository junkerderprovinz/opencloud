#!/bin/sh
# -----------------------------------------------------------------------------
# print-banner.sh <container-name> <subtitle>
# Shared Junker-der-Provinz init-log banner (POSIX sh - the OpenCloud base image
# is Alpine and has no bash). The ASCII art in /usr/local/share/banner.txt is
# identical across all container images; the name + subtitle are passed at
# runtime so the shared art stays generic.
# -----------------------------------------------------------------------------
CONTAINER="${1:-Container}"
SUBTITLE="${2:-}"
BANNER_FILE="/usr/local/share/banner.txt"

echo ""

if [ -f "${BANNER_FILE}" ]; then
    cat "${BANNER_FILE}"
    # The shared banner file has no trailing newline; add blank lines so the
    # banner gets breathing room before the title block.
    echo ""
    echo ""
else
    echo ""
    echo "  Junker der Provinz"
    echo ""
fi

# Clean title block: name + subtitle only, no rules (house look).
printf '  %s\n' "${CONTAINER}"
if [ -n "${SUBTITLE}" ]; then
    printf '  %s\n' "${SUBTITLE}"
fi
echo ""
