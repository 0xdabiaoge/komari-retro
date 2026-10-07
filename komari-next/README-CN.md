# Komari Retro

Komari Retro 是 Komari 的第二个内置前台主题，基于 Komari-Next `1.4.19` 源码标签（`914761b838d4b0f5c03a040debc0b79535be0669`）改造，并保留上游 MIT 许可证，见 `LICENSE`。

主题保留节点仪表盘、详情图表、搜索、多语言、外观控制和管理员托管设置。世界地图组件、地图数据、地图专用设置与依赖已移除；页眉、页脚、主题元数据、图标和应用清单使用 Komari Retro 品牌。

## 构建

```sh
npm ci
npm run build
```

静态产物输出到 `dist/`。发布工作流会将其复制到 `web/public/retroTheme/`，与默认主题一起嵌入 Go 服务端。主题 ID 为 `retro`。

发布构建通过 `NEXT_PUBLIC_THEME_VERSION` 同步页脚和内置主题元数据中的版本号。

## 上游署名

本衍生主题包含 [Komari-Next](https://github.com/tonyliuzj/komari-next) 源码，版权归 Tony Liu 所有，采用 MIT 许可证。完整声明见 `LICENSE`。
