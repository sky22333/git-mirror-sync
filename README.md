## Git Mirror Sync

纯 Go 的 Git 多平台镜像同步工具：以 GitHub 为源，自动备份到 CNB / GitLab / Gitee / Codeberg。CI 定时运行，无需自建服务器。

## 特性

- 自动扫描 GitHub 账号下仓库（含 private / fork / 已归档）
- 镜像同步分支、标签与完整历史（不含 PR 引用）
- 目标仓不存在时自动创建
- GitLab 推送前自动允许 force push，并解除标签保护
- 可见性默认跟随源仓（`follow`；也可 `private` / `public`）
- 网络等瞬时失败自动重试（默认最多 3 次）
- 纯 Go（go-git），不调用系统 git
- Token 仅通过环境变量 / CI Secret 注入

## 使用

### 1. 配置

```bash
cp configs/config.example.toml config.toml
```

编辑 `config.toml`，填写各目标的 `owner`（用户名 / 组织 / 群组）。按需保留或删除 `[[targets]]`。

### 2. Token

| 环境变量 | 说明 |
|---------|------|
| `GITHUB_TOKEN` | GitHub PAT（`repo`）；Actions 用 Secret `GH_PAT` 注入 |
| `GITLAB_TOKEN` | GitLab PAT（需 `api`，含读写仓库与管理保护分支） |
| `GITEE_TOKEN` | Gitee 私人令牌 |
| `CNB_TOKEN` | CNB Access Token |
| `CODEBERG_TOKEN` | Codeberg / Forgejo Access Token（需 `write:repository`；组织仓还需组织写权限） |

### 3. 运行

```
export GITHUB_TOKEN="ghp_xxxx"
export GITLAB_TOKEN="glpat-xxxx"
export GITEE_TOKEN="xxxx"
export CNB_TOKEN="xxxx"
export CODEBERG_TOKEN="xxxx"
```

```bash
./git-mirror-sync -config config.toml
```

Docker：

```bash
docker run \
  --rm \
  -v "$PWD/config.toml:/config/config.toml:ro" \
  -e GITHUB_TOKEN \
  -e GITLAB_TOKEN \
  -e GITEE_TOKEN \
  -e CNB_TOKEN \
  -e CODEBERG_TOKEN \
  ghcr.io/wananle88/git-mirror-sync
```

### 4. CI 定时任务示例

将 `cicd/` 下对应模板拷到目标平台启用，并配置上述 Secret：

- GitHub：`cicd/github-ci.yml`
- GitLab：`cicd/.gitlab-ci.yml`
- CNB：`cicd/.cnb.yml`
