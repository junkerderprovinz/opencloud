#!/bin/sh
# Stands in for the server binary: init finds a config that already exists, and
# server prints the environment the entrypoint hands over. version reports
# STUB_VERSION, and init --diff writes the patch a test put into diff.patch.
case "${1:-}" in
    version)
        if [ -n "${STUB_VERSION:-}" ]; then echo "Version: ${STUB_VERSION}"; fi
        ;;
    init)
        case " $* " in
            *" --diff "*)
                echo "diff admin-password=${IDM_ADMIN_PASSWORD:-unset}" >>/etc/opencloud/stub-calls
                if [ -f /etc/opencloud/diff.patch ]; then
                    cp /etc/opencloud/diff.patch /etc/opencloud/opencloud.config.patch
                fi
                exit 0
                ;;
        esac
        exit 1
        ;;
    server) env | sed 's/^/ENV /' ;;
esac
