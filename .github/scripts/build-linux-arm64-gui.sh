#!/usr/bin/env bash
set -euo pipefail

cimgui="$(go mod download -json github.com/AllenDang/cimgui-go | python -c 'import json,sys; print(json.load(sys.stdin)["Dir"])')"
if [[ ! -d "$cimgui/thirdparty/glfw" ]]; then
  echo "cimgui-go source directory not found: $cimgui" >&2
  exit 1
fi
glfw_build="$RUNNER_TEMP/glfw-arm64"
cimgui_build="$RUNNER_TEMP/cimgui-arm64"
arm64_lib="$GITHUB_WORKSPACE/vendor/github.com/AllenDang/cimgui-go/lib/linux/arm64"

cmake -S "$cimgui/thirdparty/glfw" -B "$glfw_build" \
  -DGLFW_BUILD_EXAMPLES=OFF \
  -DGLFW_BUILD_TESTS=OFF \
  -DGLFW_BUILD_DOCS=OFF \
  -DGLFW_BUILD_X11=ON \
  -DGLFW_BUILD_WAYLAND=ON
cmake --build "$glfw_build" --parallel 4

cmake -S "$cimgui/lib" -B "$cimgui_build"
cmake --build "$cimgui_build" --parallel 4

mkdir -p "$arm64_lib"
cp "$glfw_build/src/libglfw3.a" "$arm64_lib/"
cp "$cimgui_build/cimgui.a" "$arm64_lib/"
