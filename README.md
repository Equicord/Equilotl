# Equilotl

The Equicord Installer allows you to install [Equicord, the cutest Discord Desktop client mod](https://github.com/Equicord/Equicord)

![image](https://i.imgur.com/oHN41ss.png)

## Usage

Windows

- [GUI](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl.exe)
- [CLI](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli.exe)

MacOS

- [Universal GUI](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl.dmg)
- [X64 GUI](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-x64.dmg)
- [ARM64 GUI](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-arm64.dmg)
- [Universal CLI](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-universal)
- [X64 CLI](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-x64)
- [ARM64 CLI](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-arm64)

The CLI builds are plain binaries, so run `chmod +x <file>` after downloading.

Linux

- [GUI](https://github.com/Equicord/Equilotl/releases/latest/download/Equilotl-x11)
- [CLI](https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli-Linux)

## Building from source

### Prerequisites

You need to install the [Go programming language](https://go.dev/doc/install) and GCC, the GNU Compiler Collection (MinGW on Windows)

<details>
<summary>Additionally, if you're using Linux, you have to install some additional dependencies:</summary>

#### Base dependencies

```sh
apt install -y pkg-config libsdl2-dev libglx-dev libgl1-mesa-dev
dnf install pkg-config libGL-devel libXxf86vm-devel
```

#### X11 dependencies

```sh
apt install -y xorg-dev
dnf install libXcursor-devel libXi-devel libXinerama-devel libXrandr-devel
```

#### Wayland dependencies

```sh
apt install -y libwayland-dev libxkbcommon-dev wayland-protocols extra-cmake-modules
dnf install wayland-devel libxkbcommon-devel wayland-protocols-devel extra-cmake-modules
```

</details>

### Building

#### Install dependencies

```sh
go mod tidy
```

#### Build the GUI

##### Windows / Mac / Linux X11

```sh
make GUI=1
```

##### Linux Wayland

```sh
make GUI=1 WAYLAND=1
```

#### Build the CLI

```
go build --tags cli
```

You might want to pass some flags to this command to get a better build.
See [the GitHub workflow](https://github.com/Equicord/Equilotl/blob/main/.github/workflows/release.yml) for what flags I pass or if you want more precise instructions
