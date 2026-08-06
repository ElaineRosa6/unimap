#!/bin/sh
set -eu

config_path="${UNIMAP_CONFIG_PATH:-/app/configs/config.yaml}"
config_template="${UNIMAP_CONFIG_TEMPLATE:-/app/configs/config.docker.yaml}"

if [ ! -f "$config_path" ]; then
    if [ ! -f "$config_template" ]; then
        echo "configuration template not found: $config_template" >&2
        exit 1
    fi
    mkdir -p "$(dirname "$config_path")"
    cp "$config_template" "$config_path"
    chmod 600 "$config_path"
fi

exec "$@"
