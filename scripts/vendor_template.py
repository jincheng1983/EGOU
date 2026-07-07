"""为 wails-template 生成 vendor 目录，让用户程序编译时无需联网下载依赖。

工作流程：
1. 临时把 go.mod.tmpl 复制为 go.mod
2. 运行 `go mod download` 把依赖拉到本机 GOPATH/pkg/mod 缓存
3. 运行 `go mod vendor` 在 wails-template/vendor/ 下生成完整依赖源码
4. 删除临时 go.mod（保留 go.mod.tmpl 作为模板源文件）
5. 验证 vendor/modules.txt 存在并输出 vendor 目录大小

使用方法：
    python scripts/vendor_template.py

前置条件：
    - 本机已安装 Go SDK（IDE 开发者环境）
    - 能访问 GOPROXY（首次拉依赖时；后续 vendor 已生成则无需联网）

生成后：
    - runtime/wails-template/vendor/ 包含所有依赖源码
    - extractTemplate 复制 wails-template 到用户临时构建目录时，vendor/ 一并被复制
    - buildRuntime 检测到 vendor/ 存在时自动添加 -mod=vendor 标志，完全离线编译

已知问题：
    - Wails v3 alpha2.110 的 webviewloader 包含 //go:embed arm64/WebView2Loader.dll 指令，
      但该 DLL 不在源码包中（仅 `native_webview2loader` 构建标签下生效）。
      `go mod vendor` 会在最后校验 embed 模式时报错退出（exit code 1），
      但 vendor/ 目录和 modules.txt 实际已完整生成。
      本脚本检测到此错误时打印警告但不视为失败（构建时不会触发该 embed）。
"""

import os
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TEMPLATE_DIR = ROOT / "runtime" / "wails-template"
GO_MOD_TMPL = TEMPLATE_DIR / "go.mod.tmpl"
GO_MOD = TEMPLATE_DIR / "go.mod"


def run(cmd, cwd=None, allow_fail=False):
    """运行命令，实时打印输出。allow_fail=True 时返回非零退出码不退出。"""
    print(f">>> {cmd}  (cwd={cwd or ROOT})")
    result = subprocess.run(cmd, cwd=cwd, shell=True, capture_output=True, text=True)
    if result.stdout:
        print(result.stdout)
    if result.stderr:
        print(result.stderr, file=sys.stderr)
    if result.returncode != 0 and not allow_fail:
        print(f"!!! 命令失败，退出码 {result.returncode}", file=sys.stderr)
        sys.exit(result.returncode)
    return result.returncode, result.stdout, result.stderr


def dir_size(path):
    """计算目录大小（MB）。"""
    total = 0
    for p in Path(path).rglob("*"):
        if p.is_file():
            total += p.stat().st_size
    return total / 1024 / 1024


def main():
    if not GO_MOD_TMPL.exists():
        print(f"!!! 找不到 {GO_MOD_TMPL}", file=sys.stderr)
        sys.exit(1)

    print("=== EGOU wails-template 依赖 vendor 工具 ===\n")

    # 1. 复制 go.mod.tmpl → go.mod（临时）
    print("[1/4] 临时复制 go.mod.tmpl → go.mod")
    shutil.copy2(GO_MOD_TMPL, GO_MOD)
    print(f"    已创建: {GO_MOD}")

    try:
        # 2. go mod download（确保依赖在本机缓存）
        print("\n[2/4] go mod download（拉取依赖到 GOPATH/pkg/mod 缓存）")
        run("go mod download", cwd=str(TEMPLATE_DIR))

        # 3. go mod vendor（生成 vendor/ 目录）
        # 允许失败：Wails v3 alpha2.110 的 webviewloader embed 校验报错，
        # 但 vendor/ 目录已完整生成
        print("\n[3/4] go mod vendor（生成 vendor/ 目录）")
        # 如果 vendor/ 已存在，先删除避免残留
        vendor_dir = TEMPLATE_DIR / "vendor"
        if vendor_dir.exists():
            shutil.rmtree(vendor_dir)
            print(f"    已清理旧 vendor/: {vendor_dir}")
        rc, _, stderr = run("go mod vendor", cwd=str(TEMPLATE_DIR), allow_fail=True)
        if rc != 0:
            # 检查是否是已知的 embed 校验错误
            if "WebView2Loader.dll" in stderr or "matching files found" in stderr:
                print("    [警告] Wails v3 webviewloader embed 校验失败（已知问题）")
                print("    [警告] vendor/ 目录已生成，构建时不会触发该 embed，可安全忽略")
            else:
                print(f"!!! go mod vendor 失败（退出码 {rc}），未知错误", file=sys.stderr)
                sys.exit(rc)

    finally:
        # 4. 删除临时 go.mod（保留 go.mod.tmpl）
        print("\n[4/4] 清理临时 go.mod")
        if GO_MOD.exists():
            GO_MOD.unlink()
            print(f"    已删除: {GO_MOD}")

    # 验证 vendor/modules.txt 存在
    modules_txt = TEMPLATE_DIR / "vendor" / "modules.txt"
    if not modules_txt.exists():
        print(f"!!! vendor/modules.txt 未生成，vendoring 失败", file=sys.stderr)
        sys.exit(1)

    size_mb = dir_size(TEMPLATE_DIR / "vendor")
    print(f"\n=== vendor 完成 ===")
    print(f"vendor 目录: {TEMPLATE_DIR / 'vendor'}")
    print(f"vendor 大小: {size_mb:.2f} MB")
    print(f"modules.txt: {modules_txt}")
    print(f"\n下一步：build.py 复制 wails-template/ 到 bin/ 时会自动包含 vendor/")
    print(f"      buildRuntime 检测到 vendor/ 存在时会自动启用 -mod=vendor 离线编译")


if __name__ == "__main__":
    main()

