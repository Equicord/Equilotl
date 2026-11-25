from pathlib import Path

source = Path("vendor/github.com/AllenDang/cimgui-go/internal/type_wrapper.go")
content = source.read_bytes()

for old, new in (
    (b"func WrapString[RET ~int8](", b"func WrapString[RET ~int8 | ~uint8]("),
    (b"func WrapStringList[RET ~int8](", b"func WrapStringList[RET ~int8 | ~uint8]("),
):
    if old in content:
        content = content.replace(old, new)
    elif new not in content:
        raise SystemExit(f"cimgui-go string helper signature not found: {old.decode()}")

source.write_bytes(content)
