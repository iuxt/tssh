# tssh 官网

使用 Hugo 渲染的中文介绍页面。自包含模板，无外部主题、字体、npm 或 Go 模块依赖。

## 本地开发

安装 Hugo（CI 固定为 0.168.0），在仓库根目录运行：

```sh
hugo server --source website
```

访问命令输出的本地地址。生产构建：

```sh
hugo --source website --gc --minify --panicOnWarning
```

输出到 `website/public/`，不提交构建产物。

## 内容与样式

- `hugo.toml`：站点地址、SEO 描述、项目链接。
- `data/features.yaml`：功能介绍。
- `layouts/home.html`：首页结构与其他中文文案。
- `assets/css/main.css`：响应式样式，支持减少动态效果的系统设置。
- `assets/js/main.js`：示意主机搜索、键盘选择、示例切换、命令复制。

终端中的地址采用文档示例地址，页面不会进行 SSH 连接。文件传输为流程示意，下载入口指向本仓库 Releases。

## GitHub Pages

工作流：`.github/workflows/pages.yml`。在仓库 **Settings → Pages → Build and deployment → Source** 选择 **GitHub Actions**。

推送 `website/**` 或该工作流到 `main` 后，自动构建并部署；也可在 Actions 中手动运行。PR 只验证构建，不部署。部署步骤使用 Pages 返回的 `base_url`，兼容项目子路径与自定义域名。

默认地址：https://iuxt.github.io/tssh/

部署只需要 GitHub 自动提供的 `GITHUB_TOKEN`，无需额外密钥。修改域名后，需同步 `hugo.toml` 中的 `baseURL` 以保持本地生产构建地址一致。
