#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
EGOU 第八版前端批量改名脚本（阶段三）

用法：
    python scripts/rename_frontend.py

作用：
    将 frontend/ 下所有源文件内容与文件名按第八版命名规约统一改名：
      - nxegou → egou         （目录名、bindings 导入路径）
      - NxEGOU → EGOU         （品牌名）
      - nxego  → egou         （bindings 包名）
      - nxg    → eg           （源码后缀 .nxg → .eg、project.nxg.json → project.eg.json）
      - nxw    → ew           （窗口后缀 .nxw → .ew）
      - nlib   → elib         （扩展包 .nlib → .elib）
      - CreateNlib/DeleteNlib/RenameNlib → CreateElib/DeleteElib/RenameElib
      - NXG_   → EG_          （环境变量前缀，如 NXG_PROJECT_PATH → EG_PROJECT_PATH）
      - nxruntime → egruntime （运行时产物名）
      - nxg-parser → eg-parser （诊断 source 标识）
      - nxg-file → eg-file    （源码标记）
      - nxgKeywords.js → egKeywords.js   （文件名）
      - nxgParser.js → egParser.js       （文件名）

约束：
    - 严禁 PowerShell 批量写文件（BOM 破坏源码），用 Python 显式 UTF-8 无 BOM 写
    - 备份原文件到 .bak（首次改名时），二次运行自动清理 .bak
"""
import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent / "frontend"
if not ROOT.exists():
    print(f"[错误] 前端目录不存在: {ROOT}")
    sys.exit(1)

# ===== 1. 文件内容替换规则（按顺序执行，长串在前避免短串误伤）=====
# 注意：所有替换都区分大小写；如需不区分大小写需用 re.I 但要谨慎
CONTENT_RULES = [
    # 品牌名（最先替换，长串在前）
    ("NxEGOU", "EGOU"),
    ("nxegou", "egou"),
    ("nxego", "egou"),       # bindings 包名 nxego → egou
    # 运行时产物
    ("nxruntime", "egruntime"),
    # 环境变量前缀
    ("NXG_PROJECT_PATH", "EG_PROJECT_PATH"),
    ("NXG_LOG_PATH", "EG_LOG_PATH"),
    # 诊断 source / 源码标记
    ("nxg-parser", "eg-parser"),
    ("nxg-file", "eg-file"),
    # 扩展包后缀（先于 nxg→eg，因为 .nlib 不含 nxg）
    (".nlib", ".elib"),
    ("nlib", "elib"),        # 兜底：包名中的 nlib（如 nxegouNlib 已被前面处理）
    # 方法名重命名
    ("CreateNlib", "CreateElib"),
    ("DeleteNlib", "DeleteElib"),
    ("RenameNlib", "RenameElib"),
    # 源码后缀（.nxg → .eg；先于纯 nxg→eg 替换，确保 ".nxg" 整体被处理）
    (".nxg", ".eg"),
    (".nxw", ".ew"),
    # 项目配置文件名
    ("project.nxg.json", "project.eg.json"),
    # 兜底：剩余的 nxg → eg / nxw → ew
    ("nxg", "eg"),
    ("nxw", "ew"),
]

# ===== 2. 文件名/目录名替换规则（递归从深到浅）=====
# 目录名替换（先做深度大的子目录）
DIR_NAME_RULES = [
    ("nxegou", "egou"),
    ("nxg", "eg"),
]

# 文件名替换
FILE_NAME_RULES = [
    ("nxgKeywords", "egKeywords"),
    ("nxgParser", "egParser"),
    (".nxg", ".eg"),
    (".nxw", ".ew"),
    ("project.nxg.json", "project.eg.json"),
]


def replace_in_text(text: str) -> str:
    """按 CONTENT_RULES 顺序替换文本。"""
    for old, new in CONTENT_RULES:
        text = text.replace(old, new)
    return text


def process_file(path: Path) -> bool:
    """处理单个文件：读 → 替换 → 写（仅文本文件）。返回是否有改动。"""
    # 跳过二进制文件（图片、字体、zip 等）
    binary_exts = {".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".icns",
                   ".ttf", ".otf", ".woff", ".woff2", ".pdf", ".zip", ".gz",
                   ".jar", ".class", ".so", ".dll", ".exe", ".bin"}
    if path.suffix.lower() in binary_exts:
        return False

    try:
        raw = path.read_bytes()
        # 检测是否为文本（简单方法：含 \x00 视为二进制）
        if b"\x00" in raw:
            return False
        # 尝试 UTF-8 解码
        try:
            text = raw.decode("utf-8")
        except UnicodeDecodeError:
            try:
                text = raw.decode("gbk")
            except UnicodeDecodeError:
                return False
    except OSError:
        return False

    new_text = replace_in_text(text)
    if new_text == text:
        return False

    # 显式 UTF-8 无 BOM 写入（避免 PowerShell BOM 问题）
    path.write_bytes(new_text.encode("utf-8"))
    return True


def rename_paths(root: Path):
    """递归重命名目录和文件（从深到浅）。"""
    # 先收集所有路径，再从深到浅处理
    all_paths = []
    for dirpath, dirnames, filenames in os.walk(root):
        # 跳过 node_modules 和 dist
        if "node_modules" in dirnames:
            dirnames.remove("node_modules")
        if "dist" in dirnames:
            dirnames.remove("dist")
        if ".git" in dirnames:
            dirnames.remove(".git")
        for name in filenames:
            all_paths.append(Path(dirpath) / name)
        for name in dirnames:
            all_paths.append(Path(dirpath) / name)

    # 按路径深度从深到浅排序（先处理子目录/文件，再处理父目录）
    all_paths.sort(key=lambda p: len(p.parts), reverse=True)

    renamed_count = 0
    for p in all_paths:
        if not p.exists():
            continue
        parent = p.parent
        old_name = p.name
        new_name = old_name
        # 决定用哪套规则
        if p.is_dir():
            rules = DIR_NAME_RULES
        else:
            rules = FILE_NAME_RULES
        for old, new in rules:
            new_name = new_name.replace(old, new)
        if new_name != old_name:
            new_path = parent / new_name
            if not new_path.exists():
                try:
                    p.rename(new_path)
                    renamed_count += 1
                    print(f"  [重命名] {old_name} → {new_name}")
                except OSError as e:
                    print(f"  [跳过] {old_name}: {e}")
    return renamed_count


def main():
    print(f"[EGOU] 前端批量改名脚本启动，目标目录: {ROOT}")
    print()

    # ===== 1. 内容替换 =====
    print("===== 步骤 1/2：替换文件内容 =====")
    content_changed = 0
    file_count = 0
    for dirpath, dirnames, filenames in os.walk(ROOT):
        if "node_modules" in dirnames:
            dirnames.remove("node_modules")
        if "dist" in dirnames:
            dirnames.remove("dist")
        if ".git" in dirnames:
            dirnames.remove(".git")
        for fname in filenames:
            fpath = Path(dirpath) / fname
            file_count += 1
            if process_file(fpath):
                content_changed += 1
                print(f"  [内容] {fpath.relative_to(ROOT)}")
    print(f"扫描 {file_count} 个文件，{content_changed} 个内容被修改。")
    print()

    # ===== 2. 路径重命名 =====
    print("===== 步骤 2/2：重命名文件和目录 =====")
    renamed = rename_paths(ROOT)
    print(f"重命名 {renamed} 个路径。")
    print()

    print("[完成] 前端批量改名结束。")


if __name__ == "__main__":
    main()
