## Git Mirror Sync

纯 Go 的 Git 多平台镜像同步工具：以 GitHub 为源，自动备份到 CNB / GitLab / Gitee / Codeberg。CI 定时运行，无需自托管。

## 特性

- 自动扫描 GitHub 账号下仓库（含 private / fork / 已归档）
- 镜像同步分支、标签与完整历史（不含 PR 引用）
- 目标仓不存在时自动创建
- GitLab 推送前自动允许 force push，并解除标签保护
- 可见性默认跟随源仓（`follow`；也可 `private` / `public`）
- 网络等瞬时失败自动重试（默认最多 3 次）
- 纯 Go（go-git），不调用系统 git
- Token 仅通过环境变量 / CI Secret 注入

## 使用教程

### 1. 配置文件

```bash
cp configs/config.example.toml config.toml
```

编辑 `config.toml`，具体说明见[示例配置](configs/config.example.toml)中的注释。

### 2. Token变量说明

| 环境变量 | 说明 |
|---------|------|
| `GITHUB_TOKEN` | GitHub 访问令牌 |
| `GITLAB_TOKEN` | GitLab 访问令牌 |
| `GITEE_TOKEN` | Gitee 访问令牌 |
| `CNB_TOKEN` | CNB 访问令牌 |
| `CODEBERG_TOKEN` | Codeberg / Forgejo 访问令牌 |

需要仓库的完整读写权限，组织仓库也需要组织读写权限。

令牌必须以`环境变量`或者ci的`密钥变量`方式配置，不要明文写入配置文件。

### 3. 运行示例

1：配置Token
```
export GITHUB_TOKEN="ghp_xxxx"
export GITLAB_TOKEN="glpat-xxxx"
export GITEE_TOKEN="xxxx"
export CNB_TOKEN="xxxx"
export CODEBERG_TOKEN="xxxx"
```

2：二进制文件运行
```bash
./git-mirror-sync -config config.toml
```

3：Docker运行
```bash
docker run \
  --rm \
  -v "$PWD/config.toml:/config/config.toml:ro" \
  -e GITHUB_TOKEN \
  -e GITLAB_TOKEN \
  -e GITEE_TOKEN \
  -e CNB_TOKEN \
  -e CODEBERG_TOKEN \
  ghcr.io/sky22333/git-mirror-sync
```

### 正式环境CI定时任务

正式环境建议使用ci流水线定时运行，将 [`example`](example) 下对应ci模板示例拷到目标平台启用，并配置上述环境变量密钥
