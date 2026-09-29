#!/usr/bin/env bash

set -euo pipefail

# Use the selected toolchain's default SDK rather than xcrun's newest SDK.
# A newer SDK left by another installation may be unreadable by this linker.
# Preserve explicit SDKROOT overrides and leave other platforms unchanged.
if [[ "$(uname -s)" == "Darwin" && -z "${SDKROOT:-}" ]]; then
  developer_dir="$(xcode-select -p)"
  for sdk in \
    "$developer_dir/SDKs/MacOSX.sdk" \
    "$developer_dir/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk"; do
    if [[ -d "$sdk" ]]; then
      export SDKROOT="$sdk"
      break
    fi
  done
fi

exec go "$@"
