// Package egou 是 EGOU IDE 的根包。
//
// 第八版采用完整外置化架构：所有资源（前端 dist、字体、示例、wails-template）
// 都以磁盘文件形式存放在 exe 同级目录，IDE 启动时按需加载。
// 这样 exe 体积最小化，且用户可直接替换字体/模板/示例等资源进行个性化。
//
// 资源目录结构（与 exe 同级）：
//
//	bin/
//	  ├─ EGOU.exe              IDE 主程序（纯逻辑，无嵌入资源）
//	  ├─ frontend/dist/        前端构建产物（WebView 加载）
//	  ├─ fonts/                字体文件（egou.ttf 等）
//	  ├─ examples/             示例 .elib 扩展包源码
//	  ├─ wails-template/       用户程序编译模板
//	  ├─ libs/                 支持库（启动时释放示例到 libs/examples/）
//	  └─ config/               IDE 配置（预留）
//
// 本包不再持有任何 go:embed 变量，保留仅为兼容历史 import（如需）。
package egou
