# EGOU 插件目录

本目录存放 EGOU IDE 插件，每个子目录是一个插件包。

## 插件结构

```
plugins/
└─ my-plugin/
   ├─ package.json     插件元数据（name/version/description/author）
   ├─ main.js          插件入口（调用 activate(api) 注册功能）
   └─ ...              其他资源文件
```

## package.json 示例

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "我的插件",
  "author": "作者名"
}
```

## main.js 示例

```javascript
export function activate(api) {
  // 注册命令
  api.registerCommand('my-command', () => {
    api.output('执行了我的命令')
  })
  // 注册菜单项
  api.registerMenuItem('工具/我的命令', 'my-command')
}
```

IDE 启动时自动扫描本目录，加载每个含 package.json 的子目录。
