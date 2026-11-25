# Equilotl

The Equicord Installer allows you to install [Equicord, the cutest Discord Desktop client mod](https://github.com/Equicord/Equicord)

![image](https://i.imgur.com/oHN41ss.png)

## Usage

Windows

| | X64 | ARM64 |
| --- | --- | --- |
| GUI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl.exe) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-arm64.exe) |
| CLI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli.exe) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-arm64.exe) |

MacOS

| | Universal | X64 | ARM64 |
| --- | --- | --- | --- |
| GUI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl.dmg) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-x64.dmg) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-arm64.dmg) |
| CLI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-universal) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-x64) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-arm64) |

The CLI builds are plain binaries, so run `chmod +x <file>` after downloading.

Linux

| | X64 | ARM64 |
| --- | --- | --- |
| Combined GUI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-arm64) |
| X11 GUI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-x11) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-x11-arm64) |
| Wayland GUI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-wayland) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-wayland-arm64) |
| CLI | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-linux) | [Download](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-linux-arm64) |


## Building from source

### Prerequisites

You need to install the [Go programming language](https://go.dev/doc/install) and GCC, the GNU Compiler Collection (MinGW on Windows)

<details>
<summary>Additionally, if you're using Linux, you have to install some additional dependencies:</summary>

The Linux GUI uses the x86-64 GLFW and ImGui libraries bundled with cimgui-go.

```sh
apt install -y libgl-dev libx11-dev
dnf install libglvnd-devel libX11-devel
```

</details>

### Building

#### Install dependencies

```sh
go mod tidy
```

#### Build the GUI

##### Windows

Install [MSYS2](https://www.msys2.org), open the **MSYS2 MINGW64** shell and run:

```sh
pacman -S --needed git make mingw-w64-x86_64-gcc mingw-w64-x86_64-go
go install github.com/tc-hib/go-winres@v0.3.3
export PATH="$PATH:$(go env GOPATH)/bin"
make GUI=1
make GUI=0
```

This writes `build/Equilotl.exe` and `build/EquilotlCli.exe`. For ARM64, install `mingw-w64-clang-aarch64-clang` and run `make GUI=1 ARCH=arm64 CC=aarch64-w64-mingw32-clang CXX=aarch64-w64-mingw32-clang++`.

If you'd rather not use MSYS2, any MinGW toolchain works by pointing cgo at it, for example LLVM's clang with a MinGW sysroot:

```sh
CGO_LDFLAGS=-lwinpthread CC="clang --target=x86_64-w64-windows-gnu" CXX="clang++ --target=x86_64-w64-windows-gnu" \
  CGO_ENABLED=1 go build -tags "static gui" -ldflags "-s -w -H=windowsgui -extldflags=-static" -o Equilotl.exe
```

##### Mac / Linux

On Linux the same binary selects X11 or Wayland at startup.

```sh
make GUI=1
```

#### Build the CLI

```
go build --tags cli
```

You might want to pass some flags to this command to get a better build.
See [the GitHub workflow](https://github.com/Equicord/Equilotl/blob/main/.github/workflows/release.yml) for what flags I pass or if you want more precise instructions
