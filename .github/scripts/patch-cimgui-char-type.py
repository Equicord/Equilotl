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

source = Path("vendor/github.com/AllenDang/cimgui-go/imgui/funcs.go")
content = source.read_bytes()
old = b"(*int8)(result.Data)"
new = b"internal.ReinterpretCast[*int8](result.Data)"
if old in content:
    content = content.replace(old, new)
elif new not in content:
    raise SystemExit("cimgui-go C.char vector casts not found")
source.write_bytes(content)
