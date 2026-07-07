#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""EGOU 第八阶段二：批量改名脚本。

将第七版 NxEGOU 的命名改为第八版 EGOU 的命名。
改名规则（按优先级从长到短，避免部分匹配）：
  1. nxruntime → egruntime  (运行时产物名)
  2. NxEGOU    → EGOU       (项目名，大写)
  3. nxegou    → egou       (语言 ID/模块名)
  4. nxego     → egou       (module 路径，注意：必须晚于 nxegou)
  5. nlib      → elib       (扩展包后缀，仅 .nlib / "nlib" / nlib 上下文)
  6. .nxg      → .eg        (源码文件后缀)
  7. .nxw      → .ew        (窗口设计文件后缀)
  8. "nxg"     → "eg"       (字符串中的语言 ID)
  9. "nxw"     → "ew"       (字符串中的窗口后缀)

注意：不替换 nxg/nxw 作为变量名前缀（如 nxgParser），因为这些会在后续手动处理。
本脚本只处理安全的、明确的替换。
"""

import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# 需要改名的文件扩展名
EXTS = {'.go', '.json', '.js', '.vue', '.ts', '.md', '.yaml', '.yml', '.tmpl', '.mjs'}

# 排除的目录
EXCLUDE_DIRS = {
    'node_modules', 'dist', 'bin', '.git', '.trae',
    'frontend/node_modules', 'frontend/dist', 'frontend/bindings',
}

# 排除的文件（自身）
EXCLUDE_FILES = {'rename.py', 'build.py', 'cleanup.py', 'fix_lib_json.py'}


def should_exclude(path: Path) -> bool:
    """判断路径是否应排除。"""
    parts = path.parts
    for exc in EXCLUDE_DIRS:
        if exc.replace('/', os.sep) in str(path):
            return True
    if path.name in EXCLUDE_FILES:
        return True
    return False


def replace_content(content: str) -> tuple[str, int]:
    """对文件内容执行安全替换，返回 (新内容, 替换次数)。"""
    count = 0
    new_content = content

    # 1. nxruntime → egruntime（运行时产物名）
    new_content, n = re.subn(r'nxruntime', 'egruntime', new_content)
    count += n

    # 2. NxEGOU → EGOU（项目名，大写）
    new_content, n = re.subn(r'NxEGOU', 'EGOU', new_content)
    count += n

    # 3. nxegou → egou（语言 ID/模块名，必须在 nxego 之前）
    new_content, n = re.subn(r'nxegou', 'egou', new_content)
    count += n

    # 4. nxego → egou（module 路径，必须在 nxegou 之后）
    new_content, n = re.subn(r'nxego\b', 'egou', new_content)
    count += n

    # 5. .nlib → .elib（扩展包文件后缀）
    new_content, n = re.subn(r'\.nlib', '.elib', new_content)
    count += n

    # 6. "nlib" → "elib"（字符串中的扩展包名）
    new_content, n = re.subn(r'"nlib"', '"elib"', new_content)
    count += n

    # 7. .nxg → .eg（源码文件后缀）
    new_content, n = re.subn(r'\.nxg', '.eg', new_content)
    count += n

    # 8. .nxw → .ew（窗口设计文件后缀）
    new_content, n = re.subn(r'\.nxw', '.ew', new_content)
    count += n

    # 9. "nxg" → "eg"（字符串中的语言 ID，如 Monaco language id）
    new_content, n = re.subn(r'"nxg"', '"eg"', new_content)
    count += n

    # 10. "nxw" → "ew"（字符串中的窗口后缀）
    new_content, n = re.subn(r'"nxw"', '"ew"', new_content)
    count += n

    # 11. project.nxg.json → project.eg.json（项目配置文件名）
    #     注意：上面 .nxg → .eg 已经处理了 project.nxg.json → project.eg.json
    #     但为了确保，显式处理一次
    new_content, n = re.subn(r'project\.nxg\.json', 'project.eg.json', new_content)
    count += n

    return new_content, count


def rename_file(path: Path) -> bool:
    """重命名单个文件（如果文件名包含需要改名的部分）。"""
    name = path.name
    new_name = name
    new_name = new_name.replace('nxruntime', 'egruntime')
    new_name = new_name.replace('NxEGOU', 'EGOU')
    new_name = new_name.replace('nxegou', 'egou')
    new_name = new_name.replace('nxego', 'egou')
    new_name = new_name.replace('.nlib', '.elib')
    new_name = new_name.replace('.nxg', '.eg')
    new_name = new_name.replace('.nxw', '.ew')
    if new_name != name:
        new_path = path.parent / new_name
        path.rename(new_path)
        print(f"  重命名文件: {name} → {new_name}")
        return True
    return False


def main():
    total_files = 0
    total_replacements = 0
    renamed_files = 0

    print("=" * 60)
    print("EGOU 第八版阶段二：批量改名")
    print("=" * 60)

    # 遍历所有文件
    for root, dirs, files in os.walk(ROOT):
        root_path = Path(root)

        # 排除目录
        rel = str(root_path.relative_to(ROOT))
        if any(exc.replace('/', os.sep) in rel for exc in EXCLUDE_DIRS):
            continue

        for fname in files:
            if fname in EXCLUDE_FILES:
                continue

            fpath = root_path / fname
            ext = fpath.suffix.lower()

            # 只处理文本文件
            if ext not in EXTS and fname not in ('Taskfile', 'Taskfile.yml'):
                continue

            # 读取文件内容
            try:
                with open(fpath, 'r', encoding='utf-8') as f:
                    content = f.read()
            except (UnicodeDecodeError, PermissionError):
                continue

            # 执行替换
            new_content, count = replace_content(content)

            # 写回文件（如果有替换）
            if count > 0:
                with open(fpath, 'w', encoding='utf-8', newline='') as f:
                    f.write(new_content)
                total_files += 1
                total_replacements += count
                print(f"  替换 {count:3d} 处: {fpath.relative_to(ROOT)}")

            # 重命名文件（如果文件名需要改）
            if rename_file(fpath):
                renamed_files += 1

    # 重命名目录（nxegou → egou）
    print("\n重命名目录...")
    for root, dirs, _ in os.walk(ROOT, topdown=False):
        root_path = Path(root)
        for dname in dirs:
            new_dname = dname
            new_dname = new_dname.replace('nxegou', 'egou')
            new_dname = new_dname.replace('nxego', 'egou')
            if new_dname != dname:
                old_path = root_path / dname
                new_path = root_path / new_dname
                if old_path.exists() and not new_path.exists():
                    old_path.rename(new_path)
                    print(f"  重命名目录: {dname} → {new_dname}")
                    renamed_files += 1

    print("\n" + "=" * 60)
    print(f"完成: 替换 {total_files} 个文件, {total_replacements} 处, 重命名 {renamed_files} 个文件/目录")
    print("=" * 60)


if __name__ == "__main__":
    main()
