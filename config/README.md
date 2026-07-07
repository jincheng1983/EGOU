# EGOU 配置目录（预留）

本目录存放 IDE 配置文件，未来用于外置化 AI 配置、快捷键、主题等。

## 规划文件

- `ai_agents.json` — AI 智能体定义（替代硬编码在 aiAgents.js）
- `ai_models.json` — AI 模型预设列表（替代 localStorage）
- `ide.config.json` — IDE 全局配置（替代 localStorage）

当前这些配置仍在前端 localStorage 中，后续阶段逐步外置化。
