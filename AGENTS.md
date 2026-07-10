# AGENTS.md — Git Mirror Sync

面向 Agent / 维护者的项目说明。用户文档见 `README.md`。

## 一句话

纯 Go、CI 定时运行的 Git 多平台镜像备份工具：GitHub → CNB / GitLab / Gitee / Codeberg。

## 功能与设计目标

**目标**

- GitHub 为唯一源，自动发现账号下仓库（含 private）
- 镜像同步到 CNB / GitLab / Gitee / Codeberg（分支、标签、完整历史）
- 目标仓不存在时自动创建；可见性：`private` | `public` | `follow`
- 无服务器；Token 只走环境变量 / CI Secret
- 纯 Go：禁止调用系统 `git`、shell、外部脚本

**同步内容**：Commit / Branch / Tag / History  
**不做**：Issue、PR、Wiki、Release、LFS、Web 后台、数据库、删除目标仓

**流程**

```
读配置 → 初始化 Provider → 列 GitHub 仓
  → 每目标：Exists? → 否且 create_missing → Create
  → PrepareForMirror（如 GitLab 允许 force push / 解除标签保护）
  → Mirror Clone → Force Push（仅 heads/tags，不 prune）→ 报告
```

**策略默认**：`visibility=follow`，`include_forks=true`，`include_archived=true`，`create_missing=true`，`delete_missing=false`，`max_retries=3`（瞬时网络错误）

## 项目结构

```
cmd/main.go                   CLI 入口
internal/
  config/                     TOML 加载与校验
  model/                      Repository / CreateOptions / 可见性
  provider/                   Source / Target 接口
    github/                   源
    gitlab/ gitee/ cnb/ codeberg/  目标
  mirror/                     go-git Mirror Engine
  sync/                       编排与并发
  report/                     同步报告
configs/config.example.toml   示例配置
config.toml                   运行配置（改 owner，勿写 Token）
Dockerfile                    多阶段镜像构建
.github/workflows/release.yml 手动发布二进制（linux/windows）
cicd/                         定时同步流水线模板（GitHub / GitLab / CNB）
```

**扩展新平台**：实现 `provider.Target`，在 `cmd/main.go` 注册，补 CI Secret 说明。

## API 与依赖

| 平台 | 文档 / SDK | 本仓库用法 |
|------|------------|------------|
| GitHub | [go-github](https://github.com/google/go-github) · [REST](https://docs.github.com/en/rest) | 列仓；Token 需 `repo` |
| GitLab | [client-go/v2](https://gitlab.com/gitlab-org/api/client-go) · [Projects API](https://docs.gitlab.com/api/projects/) | Exists / Create；需 `api` + `write_repository` |
| Gitee | [API v5 Swagger](https://gitee.com/api/v5/swagger) | 无稳定主流 Go SDK → 直接 HTTP |
| CNB | [OpenAPI](https://docs.cnb.cool/en/develops/openapi.html) · [go-cnb](https://cnb.cool/cnb/sdk/go-cnb) | `api.cnb.cool`，Bearer Token |
| Codeberg | [Forgejo API](https://forgejo.org/docs/latest/user/api-usage/) · [Swagger](https://codeberg.org/api/swagger) | Forgejo `/api/v1`，直接 HTTP；Token 需 `write:repository` |
| Git | [go-git](https://github.com/go-git/go-git) | `Mirror: true` clone；仅 force push `heads/*` + `tags/*` |
| 配置 | [go-toml/v2](https://github.com/pelletier/go-toml) | `config.toml` |

**环境变量**：`GITHUB_TOKEN` · `GITLAB_TOKEN` · `GITEE_TOKEN` · `CNB_TOKEN` · `CODEBERG_TOKEN`  
（Actions 用 Secret `GH_PAT` 注入为 `GITHUB_TOKEN`，勿用默认 Actions token）

**配置要点**：每目标必填 `owner`（用户/组织/群组）；可选 `base_url` / `api_url`。Codeberg 改 URL 可对接自建 Forgejo。

## 维护规范

1. **依赖**：只用主流稳定库的最新稳定版；`go get` 后 `go mod tidy`；禁止引入冷门/未维护包。
2. **纯 Go**：Git 操作只走 `internal/mirror`（go-git）；禁止 `os/exec` 调 git。
3. **密钥**：Token 禁止写入配置或提交仓库；只读 `token_env` 对应环境变量。
4. **分层**：平台差异只在 `internal/provider/*`；同步逻辑不感知具体平台。
5. **改动后**：`go test ./...` 与 `go build -o git-mirror-sync ./cmd` 必须通过。
6. **风格**：小改动、少注释；不顺手重构无关文件；用户文档改 `README.md`，本文件只保留 Agent 要点。
7. **精简实现**：用最少代码实现最佳方案；流程清晰、逻辑闭环。
8. **拒绝冗余**：避免重复兜底与不符合真实场景的防御性代码。
9. **修 Bug**：先定位准确根因再给最佳方案；禁止无用补丁；新增实现时清理旧的无用代码。
10. **以代码为准**：分析对照最新代码状态；禁止幻想猜测与过时旧文档。

## 迭代规范

| 优先级 | 方向 | 说明 |
|--------|------|------|
| P0 | 稳定性 | 大仓/空仓/鉴权失败、单仓失败不拖垮整次、报告清晰 |
| P1 | 可用性 | 仓库过滤、命名映射、超时重试、并发与磁盘策略 |
| P2 | 平台 | 自建 GitLab、更多目标 Provider |
| 不做（除非明确要求） | 元数据同步、LFS、删除目标、Web/DB |

**合入标准**：行为与本文「功能与设计目标」一致；配置向后兼容或文档同步更新；有测试或可复现验证步骤。

**版本依赖以 `go.mod` 为准**；升级大版本时先确认 API 破坏性变更再改 Provider。
