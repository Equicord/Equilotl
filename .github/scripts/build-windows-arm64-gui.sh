#!/usr/bin/env bash
set -euo pipefail

module_dir="$(go mod download -json github.com/AllenDang/cimgui-go | python -c 'import json,sys; print(json.load(sys.stdin)["Dir"])')"
if [[ -z "$module_dir" ]]; then
  echo "cimgui-go module directory not found" >&2
  exit 1
fi
cimgui="$(cygpath -u "$module_dir")"
if [[ ! -d "$cimgui/thirdparty/glfw" ]]; then
  echo "cimgui-go source directory not found: $cimgui" >&2
  exit 1
fi
temp_dir="$(cygpath -u "$RUNNER_TEMP")"
workspace="$(cygpath -u "$GITHUB_WORKSPACE")"
glfw_build="$temp_dir/glfw-arm64"
cimgui_build="$temp_dir/cimgui-arm64"
arm64_lib="$workspace/vendor/github.com/AllenDang/cimgui-go/lib/windows/arm64"

cmake -S "$cimgui/thirdparty/glfw" -B "$glfw_build" -G Ninja \
  -DCMAKE_SYSTEM_PROCESSOR=aarch64 \
  -DCMAKE_C_COMPILER=aarch64-w64-mingw32-clang \
  -DCMAKE_CXX_COMPILER=aarch64-w64-mingw32-clang++ \
  -DCMAKE_RC_COMPILER=aarch64-w64-mingw32-windres \
  -DGLFW_BUILD_EXAMPLES=OFF \
  -DGLFW_BUILD_TESTS=OFF \
  -DGLFW_BUILD_DOCS=OFF
cmake --build "$glfw_build" --parallel 4

cmake -S "$cimgui/lib" -B "$cimgui_build" -G Ninja \
  -DCMAKE_SYSTEM_PROCESSOR=aarch64 \
  -DCMAKE_C_COMPILER=aarch64-w64-mingw32-clang \
  -DCMAKE_CXX_COMPILER=aarch64-w64-mingw32-clang++ \
  -DCMAKE_RC_COMPILER=aarch64-w64-mingw32-windres
cmake --build "$cimgui_build" --parallel 4

mkdir -p "$arm64_lib"
cp "$glfw_build/src/libglfw3.a" "$arm64_lib/"
cp "$cimgui_build/cimgui.a" "$arm64_lib/"
