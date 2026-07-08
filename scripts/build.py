#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""EGOU IDE 静态编译脚本（Wails v3 + Vue）。

强制使用此脚本编译 EGOU IDE，禁止用 PowerShell 批量写文件（BOM 会破坏源码）。
基于第七版 NxEGOU scripts/build.py 改名（NxEGOU → EGOU）。
"""

import argparse
import os
import shutil
import subprocess
import sys
import threading
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
FRONTEND = ROOT / "frontend"
BIN_DIR = ROOT / "bin"


def resolve_wails3():
    """解析 wails3 CLI 路径：优先 WAILS3 环境变量，其次 PATH 查找，最后 GOPATH/bin 回退。"""
    env = os.environ.get("WAILS3")
    if env:
        p = Path(env)
        if p.exists():
            return p
    found = shutil.which("wails3")
    if found:
        return Path(found)
    gopath = os.environ.get("GOPATH", "")
    if gopath:
        candidate = Path(gopath) / "bin" / ("wails3.exe" if os.name == "nt" else "wails3")
        if candidate.exists():
            return candidate
    return None


def run(cmd, cwd=None, check=True, env=None):
    print(f"$ {' '.join(str(c) for c in cmd)}")
    result = subprocess.run(cmd, cwd=cwd or ROOT, text=True, env=env)
    if check and result.returncode != 0:
        sys.exit(result.returncode)
    return result


def ensure_garble():
    """确保 garble.exe 可用。返回路径字符串或 None。

    查找顺序：
      1. tools/garble.exe（已预编译，离线可用）
      2. PATH 中的 garble（用户自行 go install mvdan.cc/garble@latest）
      3. 用 `go install mvdan.cc/garble@latest` 在线安装到 GOPATH/bin，再复制到 tools/

    v0.8.0 起 UPX 完全移除，Garble 成为 IDE 本体和用户产物的唯一防逆向手段。
    v0.8.0 修订：IDE 本体固定用 garble -tiny（仅变量名/函数名混淆），
    去掉 -literals（字符串字面量运行时解密会触发杀软误报 TrojanSpy/Stealer.uj）。
    首次构建需联网（go install），后续从 tools/garble.exe 离线复用。
    """
    # 1. tools/garble.exe 已存在
    local_garble = ROOT / "tools" / "garble.exe"
    if local_garble.exists():
        return str(local_garble)

    # 2. PATH 查找
    found = shutil.which("garble") or shutil.which("garble.exe")
    if found:
        return found

    # 3. 在线 go install（首次构建）
    print("未找到 garble，尝试 go install mvdan.cc/garble@latest（首次需联网）...")
    tools_dir = ROOT / "tools"
    tools_dir.mkdir(parents=True, exist_ok=True)
    result = subprocess.run(
        ["go", "install", "mvdan.cc/garble@latest"],
        cwd=ROOT, text=True, capture_output=True,
    )
    if result.returncode != 0:
        print(f"go install garble 失败: {result.stderr}")
        print("  提示: 检查网络连接，或手动执行 `go install mvdan.cc/garble@latest`")
        return None

    # 从 GOPATH/bin 复制到 tools/garble.exe
    gopath = os.environ.get("GOPATH", "")
    if not gopath:
        # 默认 GOPATH 为 ~/go
        gopath = str(Path.home() / "go")
    gopath_garble = Path(gopath) / "bin" / "garble.exe"
    if gopath_garble.exists():
        shutil.copy2(gopath_garble, local_garble)
        print(f"已预编译 garble 到: {local_garble}")
        return str(local_garble)

    print(f"go install 成功但未找到 garble.exe（GOPATH/bin 路径异常）: {gopath_garble}")
    return None


def main():
    parser = argparse.ArgumentParser(description="EGOU IDE 构建脚本")
    parser.add_argument("--skip-test", action="store_true", help="跳过 go test（开发迭代加速）")
    parser.add_argument("--skip-frontend", action="store_true", help="跳过前端构建（前端无变更时加速）")
    args = parser.parse_args()

    BIN_DIR.mkdir(exist_ok=True)

    # 0. 清理前端缓存，确保前端代码全部重新构建（避免旧缓存导致修改不生效）
    for cache_dir in ["frontend/dist", "frontend/bindings", "frontend/node_modules/.cache"]:
        p = FRONTEND / cache_dir.replace("frontend/", "")
        if p.exists():
            print(f"清理缓存: {p}")
            shutil.rmtree(p, ignore_errors=True)

    # 1. 依赖整理
    run(["go", "mod", "tidy"])

    # 2. 生成 Wails 前端绑定（扫描 cmd/egou 主包，输出到 frontend/bindings）
    wails3 = resolve_wails3()
    if not wails3:
        print("未找到 Wails3 CLI（请设置 WAILS3 环境变量或加入 PATH）", file=sys.stderr)
        sys.exit(1)
    run([str(wails3), "generate", "bindings", "-d", "frontend/bindings", "-ts", "./cmd/egou/..."])

    # 3. 前端构建（可与 go test 并行）
    npm = shutil.which("npm")
    if not npm:
        print("未找到 npm", file=sys.stderr)
        sys.exit(1)
    if not args.skip_frontend:
        test_thread = None
        if not args.skip_test:
            test_thread = threading.Thread(target=lambda: run(["go", "test", "./..."]))
            test_thread.start()
        run([npm, "install"], cwd=FRONTEND)
        run([npm, "run", "build"], cwd=FRONTEND)
        if test_thread:
            test_thread.join()
    elif not args.skip_test:
        run(["go", "test", "./..."])

    # 3.5 预生成 wails-template 的 vendor 目录（离线编译依赖）
    #      必须在步骤4（runtime-frontend 缓存）之前运行，确保预构建缓存时也能用 vendor 模式
    #      解决中国网络环境 proxy.golang.org 不可达导致的 "dial tcp ... timeout" 错误
    template_src = ROOT / "runtime" / "wails-template"
    vendor_dir = template_src / "vendor"
    if not vendor_dir.exists():
        print("\n[3.5] 生成 wails-template vendor 目录（离线编译依赖）...")
        vendor_script = ROOT / "scripts" / "vendor_template.py"
        if vendor_script.exists():
            r = subprocess.run([sys.executable, str(vendor_script)], cwd=str(ROOT), text=True)
            if r.returncode != 0:
                print(f"vendor 生成失败（退出码 {r.returncode}），运行时前端缓存将需要联网构建", file=sys.stderr)
        else:
            print(f"未找到 {vendor_script}，跳过 vendor 生成")
    else:
        print(f"\n[3.5] vendor 目录已存在，跳过生成: {vendor_dir}")

    # 4. 预构建运行时前端缓存，缩短 IDE 首次运行/构建用户程序的耗时
    #    同时预打包到 bin/runtime-frontend/（随 exe 分发，无 Node.js 环境也可用）
    bin_cache = BIN_DIR / "runtime-frontend"
    if bin_cache.exists():
        shutil.rmtree(bin_cache)
    prepare_script = ROOT / "scripts" / "prepare_runtime_cache.go"
    if prepare_script.exists():
        run(["go", "run", "scripts/prepare_runtime_cache.go"])

    # 5. 静态编译桌面应用
    #    -tags production,netgo,osusergo: 纯静态链接
    #    -trimpath: 去除构建机器路径
    #    -buildvcs=false: 去除 VCS 元数据
    #    -ldflags -w -s: 去除调试符号和 DWARF 信息
    #    -X main.Version: 注入版本号
    #    生成 .syso 图标资源，让 exe 带图标
    icon_ico = ROOT / "build" / "windows" / "icon.ico"
    # 先确保 icon.ico 是多尺寸（16/32/48/64/128/256），避免任务栏因缺尺寸回退到默认图标
    appicon_png = ROOT / "build" / "appicon.png"
    if not appicon_png.exists():
        # 回退到根目录 appicon.png
        root_appicon = ROOT / "appicon.png"
        if root_appicon.exists():
            appicon_png = root_appicon
    if appicon_png.exists() and icon_ico.parent.exists():
        try:
            from PIL import Image
            img = Image.open(appicon_png).convert("RGBA")
            sizes = [(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
            img.save(str(icon_ico), format="ICO", sizes=sizes)
            print(f"已生成多尺寸 icon.ico: {icon_ico.name} (源: {appicon_png.name} {img.size})")
        except ImportError:
            print("未安装 Pillow，跳过多尺寸 ICO 生成（pip install pillow 可修复）")
        except Exception as e:
            print(f"生成多尺寸 ICO 失败: {e}，使用现有 icon.ico")
    # 确保根目录 icon.ico 也存在（go build 链接 syso 用）
    root_icon = ROOT / "icon.ico"
    if root_icon.exists() and icon_ico.exists() and not icon_ico.exists():
        shutil.copy(str(root_icon), str(icon_ico))
    if icon_ico.exists():
        # syso 文件必须放在 main 包目录（cmd/egou/），Go 编译时才会自动链接到 exe
        syso_file = ROOT / "cmd" / "egou" / "windows_amd64.syso"
        manifest_path = ROOT / "build" / "windows" / "wails.exe.manifest"
        info_path = ROOT / "build" / "windows" / "info.json"
        # 方案1：优先用 wails3 generate syso（同时嵌入图标 + manifest + 版本信息）
        if wails3 and manifest_path.exists() and info_path.exists():
            cmd = [str(wails3), "generate", "syso",
                   "-arch", "amd64",
                   "-icon", str(icon_ico),
                   "-manifest", str(manifest_path),
                   "-info", str(info_path),
                   "-out", str(syso_file)]
            result = subprocess.run(cmd, capture_output=True, text=True)
            if result.returncode == 0:
                print(f"已生成图标资源（wails3：图标+清单+版本信息）: {syso_file.name}")
            else:
                print(f"wails3 生成失败: {result.stderr}，尝试 rsrc 回退")
                wails3 = None
        # 方案2：rsrc 回退（仅图标，rsrc 嵌入 manifest 会导致 IDE 无法启动）
        if not wails3:
            gopath = os.environ.get("GOPATH", "")
            rsrc = None
            if gopath:
                candidate = Path(gopath) / "bin" / ("rsrc.exe" if os.name == "nt" else "rsrc")
                if candidate.exists():
                    rsrc = str(candidate)
            if not rsrc:
                rsrc = shutil.which("rsrc")
            if rsrc:
                cmd = [rsrc, "-ico", str(icon_ico), "-arch", "amd64", "-o", str(syso_file)]
                result = subprocess.run(cmd, capture_output=True, text=True)
                if result.returncode == 0:
                    print(f"已生成图标资源（rsrc：仅图标）: {syso_file.name}")
                else:
                    print(f"rsrc 生成失败: {result.stderr}，尝试 windres 回退")
                    rsrc = None
            # 方案3：windres 最终回退
            if not rsrc:
                rc_file = ROOT / "icon.rc"
                with open(rc_file, "w", encoding="ascii") as f:
                    f.write('1 ICON "build/windows/icon.ico"\n')
                try:
                    windres = shutil.which("windres")
                    if windres:
                        subprocess.run([windres, "-O", "coff", "-J", "rc", "-i", str(rc_file), "-o", str(syso_file)],
                                       check=True, capture_output=True)
                        print(f"已生成图标资源（windres）: {syso_file.name}")
                    else:
                        print("未找到 wails3、rsrc 和 windres，跳过图标资源生成（exe 将无图标）")
                finally:
                    if rc_file.exists():
                        rc_file.unlink()
    version = "0.0.1"
    exe_path = BIN_DIR / "EGOU.exe"
    # v0.8.0 修订2：IDE 本体改回普通 go build（不用 Garble）
    # 原因：Garble 会混淆未导出的包名（如 internal/app 的 "app"），导致 Wails v3 运行时
    #   通过反射计算的方法 ID 与 wails3 generate bindings（未混淆）生成的方法 ID 不一致，
    #   前端调用时报 "unknown bound method id" 错误。
    # IDE 本体防逆向方案：-trimpath（去路径）+ -s -w（去符号表/调试信息），不依赖 garble。
    # 用户产物仍支持 garble 混淆（用户产物不使用 Wails binding，不受此限制）。
    print("IDE 本体用普通 go build（Garble 与 Wails binding 反射不兼容）...")
    run([
        "go", "build",
        "-tags", "production,netgo,osusergo",
        "-trimpath",
        "-buildvcs=false",
        "-p", "4",
        "-ldflags", f"-H windowsgui -w -s -X main.Version={version}",
        "-o", str(exe_path),
        "./cmd/egou",
    ])

    # 编译完成后清理 .syso 文件（避免污染源码目录）
    for syso in (ROOT / "cmd" / "egou").glob("windows_*.syso"):
        syso.unlink()
    for syso in ROOT.glob("windows_*.syso"):  # 兼容旧位置清理
        syso.unlink()

    # 6. v0.8.0 起 UPX 完全移除（杀软误杀严重）
    #    IDE 本体和用户产物都不再使用 UPX，改用 Garble 源码混淆（步骤 5 已对 IDE 本体启用）
    #    用户产物的 Garble 混淆由 runner.go buildRuntime 控制（前端"编译选项"开关同步）

    # 7. 完整外置化：把所有资源复制到 bin/，exe 同级目录就是完整发布包。
    #    第八版采用外置化架构：exe 只做 IDE 逻辑，资源（前端/字体/示例/模板/upx）全外置。
    #    用户安装只需 IDE 文件夹 + Go 环境，无需其他依赖。

    # 7.1 前端构建产物（WebView 资源）
    dist_src = FRONTEND / "dist"
    dist_dst = BIN_DIR / "frontend" / "dist"
    if dist_src.exists():
        if dist_dst.exists():
            shutil.rmtree(dist_dst)
        shutil.copytree(dist_src, dist_dst)
        print(f"已复制前端构建产物: frontend/dist/")

    # 7.2 字体目录
    # 字体已通过 Vite 构建进 frontend/dist/fonts/egou.ttf，不再单独复制到 bin/fonts/。
    # 用户如需替换字体，修改 frontend/public/fonts/egou.ttf 后重新 python scripts/build.py 即可。

    # 7.3 全局支持库（.elib 扩展包，所有项目共享）
    #     参考 NxEGO2 设计：libs/ 直接放 .elib，不再用 examples 释放机制
    libs_src = ROOT / "libs"
    libs_dst = BIN_DIR / "libs"
    if libs_src.exists():
        if libs_dst.exists():
            shutil.rmtree(libs_dst)
        shutil.copytree(libs_src, libs_dst)
        print(f"已复制全局支持库: libs/")
    else:
        # 确保 libs/ 目录存在（后端 ScanGlobalLibs 依赖）
        libs_dst.mkdir(parents=True, exist_ok=True)
        print(f"已创建空 libs/ 目录（无全局支持库）")

    # 7.3.1 项目模板（新建项目对话框的模板源）
    templates_src = ROOT / "templates"
    templates_dst = BIN_DIR / "templates"
    if templates_src.exists():
        if templates_dst.exists():
            shutil.rmtree(templates_dst)
        shutil.copytree(templates_src, templates_dst)
        print(f"已复制项目模板: templates/")
    else:
        templates_dst.mkdir(parents=True, exist_ok=True)
        print(f"已创建空 templates/ 目录（无项目模板）")

    # 7.3.2 插件目录（外置插件生态，每个子目录是一个插件包）
    plugins_src = ROOT / "plugins"
    plugins_dst = BIN_DIR / "plugins"
    if plugins_src.exists():
        if plugins_dst.exists():
            shutil.rmtree(plugins_dst)
        shutil.copytree(plugins_src, plugins_dst)
        print(f"已复制插件目录: plugins/")
    else:
        plugins_dst.mkdir(parents=True, exist_ok=True)
        print(f"已创建空 plugins/ 目录（无插件）")

    # 7.3.3 配置目录（预留，未来放 ai_agents.json 等外置配置）
    config_src = ROOT / "config"
    config_dst = BIN_DIR / "config"
    if config_src.exists():
        if config_dst.exists():
            shutil.rmtree(config_dst)
        shutil.copytree(config_src, config_dst)
        print(f"已复制配置目录: config/")

    # 7.3.4 组件库目录（预留，窗口设计器的外置组件生态）
    components_src = ROOT / "components"
    components_dst = BIN_DIR / "components"
    if components_src.exists():
        if components_dst.exists():
            shutil.rmtree(components_dst)
        shutil.copytree(components_src, components_dst)
        print(f"已复制组件库目录: components/")

    # 7.4 wails-template 用户程序编译模板（vendor 已在步骤3.5生成）
    template_src = ROOT / "runtime" / "wails-template"
    template_dst = BIN_DIR / "wails-template"

    if template_src.exists():
        if template_dst.exists():
            shutil.rmtree(template_dst)
        shutil.copytree(template_src, template_dst)
        # 把 go.mod.tmpl 重命名为 go.mod（外置化不需要 .tmpl 后缀）
        mod_tmpl = template_dst / "go.mod.tmpl"
        mod_real = template_dst / "go.mod"
        if mod_tmpl.exists():
            shutil.copy2(mod_tmpl, mod_real)
            mod_tmpl.unlink()
        # 移除跨平台无用目录（android/darwin/ios/linux/docker），减小发布包体积
        for platform_dir in ["build/android", "build/darwin", "build/ios", "build/linux", "build/docker"]:
            p = template_dst / platform_dir
            if p.exists():
                shutil.rmtree(p)
        # 验证 vendor 目录已复制
        vendor_dst = template_dst / "vendor"
        if vendor_dst.exists():
            print(f"已复制用户程序模板（含 vendor 离线依赖）: wails-template/")
        else:
            print(f"警告: vendor 目录未复制，用户程序编译需要联网", file=sys.stderr)

    # 7.5 Garble 工具（随 IDE 打包，用户程序编译时直接用，无需用户另装）
    #     v0.8.0 起 UPX 完全移除，Garble 成为唯一防逆向手段
    #     v0.8.0 修订2：IDE 本体不用 garble（与 Wails binding 反射不兼容），
    #       但 garble.exe 仍随包分发，供用户产物编译使用
    garble_exe = ensure_garble()
    garble_dst_dir = BIN_DIR / "tools"
    garble_dst_dir.mkdir(parents=True, exist_ok=True)
    if garble_exe:
        garble_dst = garble_dst_dir / "garble.exe"
        # garble_exe 可能是 tools/garble.exe（已预编译）或 PATH 中的路径
        if Path(garble_exe).resolve() != garble_dst.resolve():
            shutil.copy2(garble_exe, garble_dst)
        print(f"已复制 Garble 工具: tools/garble.exe")
    else:
        print("未找到 garble，bin/tools/ 不包含 garble（用户产物将无法启用混淆）")

    # 7.5b Wails3 CLI（随 IDE 打包，用户程序前端资源构建/缓存失效时使用）
    #      无 wails3 时 runtime-frontend 缓存失效将无法重新构建前端
    if wails3:
        wails3_dst = garble_dst_dir / "wails3.exe"
        if Path(wails3).resolve() != wails3_dst.resolve():
            shutil.copy2(wails3, wails3_dst)
        wails3_size = wails3_dst.stat().st_size / (1024 * 1024)
        print(f"已复制 Wails3 CLI: tools/wails3.exe（{wails3_size:.1f} MB）")
    else:
        print("警告: wails3 未找到，用户程序前端缓存失效时将无法重新构建", file=sys.stderr)

    # 7.6 内置 Go SDK（精简版，用户无需自装 Go 环境）
    #     从系统 GOROOT 复制到 bin/go/，去掉 doc/misc/test/api 减小体积
    #     保留 bin + lib + pkg + src + go.env + VERSION（编译必需）
    go_sdk_dst = BIN_DIR / "go"
    go_root = os.environ.get("GOROOT", "")
    if not go_root:
        # 从 go env GOROOT 获取
        try:
            result = subprocess.run(["go", "env", "GOROOT"], capture_output=True, text=True)
            go_root = result.stdout.strip()
        except Exception:
            go_root = ""
    if go_root and Path(go_root).exists():
        # 精简：跳过 doc/misc/test/api 目录
        skip_dirs = {"doc", "misc", "test", "api"}
        if go_sdk_dst.exists():
            shutil.rmtree(go_sdk_dst)
        go_sdk_dst.mkdir(parents=True)
        total_files = 0
        for item in Path(go_root).iterdir():
            if item.name in skip_dirs:
                continue
            dst = go_sdk_dst / item.name
            if item.is_dir():
                shutil.copytree(item, dst, symlinks=True)
            else:
                shutil.copy2(item, dst)
            total_files += 1
        # 验证 go.exe 存在
        go_exe_dst = go_sdk_dst / "bin" / "go.exe"
        if not go_exe_dst.exists():
            print(f"错误：Go SDK 复制失败，{go_exe_dst} 不存在", file=sys.stderr)
        else:
            # 清理 go.env：转换 CRLF→LF，移除会干扰运行时环境的硬编码配置行
            # 问题：系统 go.env 使用 CRLF，Go 解析时 \r 会成为值的一部分导致
            #       "invalid GOTOOLCHAIN \"auto\r\"" 错误；且 GOROOT/GOPROXY/GOSUMDB
            #       等硬编码路径会覆盖运行时 buildGoEnv() 设置的隔离环境。
            go_env_dst = go_sdk_dst / "go.env"
            if go_env_dst.exists():
                try:
                    raw = go_env_dst.read_bytes()
                    text = raw.decode("utf-8", errors="replace")
                    text = text.replace("\r\n", "\n").replace("\r", "\n")
                    lines = text.split("\n")
                    cleaned = []
                    strip_prefixes = ("GOROOT=", "GOTOOLCHAIN=", "GOPROXY=", "GOSUMDB=",
                                      "GONOSUMCHECK=", "GONOSUMDB=", "GOFLAGS=", "GOWORK=", "GOBIN=")
                    for line in lines:
                        stripped = line.strip()
                        if stripped.startswith(strip_prefixes):
                            continue
                        cleaned.append(line)
                    text = "\n".join(cleaned)
                    if not text.endswith("\n"):
                        text += "\n"
                    go_env_dst.write_bytes(text.encode("utf-8"))
                except Exception:
                    pass
        # 统计体积
        sdk_size = sum(f.stat().st_size for f in go_sdk_dst.rglob("*") if f.is_file()) / (1024 * 1024)
        print(f"已复制精简 Go SDK: go/（{sdk_size:.0f} MB，跳过 doc/misc/test/api）")

        # 7.6b 自动打包 dlv（Delve 调试器）到 bin/go/bin/dlv.exe
        #     查找顺序：PATH → GOPATH/bin（go install 默认位置）
        dlv_src = shutil.which("dlv") or shutil.which("dlv.exe")
        if not dlv_src:
            gopath = os.environ.get("GOPATH", "")
            if not gopath:
                gopath = str(Path.home() / "go")
            for candidate in [Path(gopath) / "bin" / "dlv.exe", Path(gopath) / "bin" / "dlv"]:
                if candidate.exists():
                    dlv_src = str(candidate)
                    break
        if dlv_src and Path(dlv_src).exists():
            dlv_dst = go_sdk_dst / "bin" / ("dlv.exe" if dlv_src.endswith(".exe") else "dlv")
            shutil.copy2(dlv_src, dlv_dst)
            dlv_size = dlv_dst.stat().st_size / (1024 * 1024)
            print(f"已打包内置 Delve 调试器: go/bin/{dlv_dst.name}（{dlv_size:.1f} MB）")
        else:
            print("提示：未找到 dlv.exe（执行 `go install github.com/go-delve/delve/cmd/dlv@latest` 安装），"
                  "发布包将不含内置调试器，用户需自行安装 dlv 到 PATH")
    else:
        print("警告：未能获取 GOROOT，Go SDK 未复制！用户程序编译将需要系统安装 Go。", file=sys.stderr)

    # 7.6 复制 README.md（如果存在）
    readme_src = ROOT / "README.md"
    readme_dst = BIN_DIR / "README.md"
    if readme_src.exists():
        shutil.copy2(readme_src, readme_dst)

    print(f"\n构建完成: {exe_path}")
    print(f"发布目录结构（完整外置化，exe 无嵌入资源）:")
    for item in sorted(BIN_DIR.iterdir(), key=lambda x: (not x.is_dir(), x.name.lower())):
        if item.is_dir():
            print(f"  📁 {item.name}/")
        else:
            size = item.stat().st_size
            print(f"  📄 {item.name} ({size / 1024 / 1024:.2f} MB)" if size > 1024 * 1024 else f"  📄 {item.name} ({size // 1024} KB)")


if __name__ == "__main__":
    main()
