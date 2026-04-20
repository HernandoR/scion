# Scion 中文用户教程（仅使用）

> 本文只讲“如何使用”，不讲部署与安装。

## 1. 先建立心智模型：核心概念与交互关系

- **Grove**：项目空间（通常对应项目里的 `.scion`），是 Agent 的边界。
- **Agent**：执行任务的容器化工作单元。
- **Hub**：团队协作控制平面（可选）。
- **Runtime Broker**：真正运行 Agent 的执行节点（本机或远端）。

它们的交互关系可以理解为：

1. 你在某个 **Grove** 中启动 Agent。  
2. Agent 在 **Broker** 上运行，执行代码任务。  
3. 如果启用 **Hub**，Hub 负责调度、鉴权和状态汇总。  
4. 你通过 CLI/Web 与 Agent 交互，最终把代码变更并入仓库。

## 2. 建议的使用顺序（从易到难）

### 2.1 在项目内启动与管理 Agent

```bash
# 启动并让 Agent 执行任务
scion start fix-login "修复登录流程中的 500 错误"

# 查看状态
scion list

# 查看日志
scion logs fix-login

# 进入交互会话
scion attach fix-login

# 结束后清理
scion delete fix-login
```

### 2.2 Hub 场景下启用 Broker（给团队提供算力）

```bash
scion broker start
scion broker register
scion broker provide
scion broker status
```

## 3. Agent 如何拿到 Git 代码、用什么凭据、如何保存变更

### 3.1 代码来源（两种模式）

#### 本地模式 (worktree)

- Agent 使用独立 git worktree 工作，典型路径是：  
  `../.scion_worktrees/<grove>/<agent>`
- 每个 Agent 在独立分支上改代码，避免相互踩踏。

#### Hub 模式 (git-clone)

- `sciontool init` 会在容器内检测 `SCION_GIT_CLONE_URL`，将仓库初始化到 `/workspace`。  
- 实现不是直接 `git clone`，而是 `git init + git fetch + checkout`，以兼容挂载目录不为空的情况。
- 分支选择顺序：
  - 先尝试 `SCION_AGENT_BRANCH`。
  - 失败后回退 `SCION_GIT_BRANCH`（默认 `main`）。
  - 必要时再探测远端默认分支。
  - 最后切换/创建 agent 分支（默认 `scion/<agentName>`）。

### 3.2 Git 凭据

- 默认使用 `GITHUB_TOKEN`。  
- 若启用 GitHub App，使用 `sciontool credential-helper` 按需刷新 token。  
- 凭据 helper 写入 **Agent HOME 的 `.gitconfig`**，避免污染共享工作区。  
- clone 后会把远端 URL 中的 token 清理掉，避免凭据落盘在 `git remote -v`。

### 3.3 变更如何保留到仓库

- Agent 在自己的分支上修改并提交。  
- 你审查 diff 与日志后，按团队 Git 流程合并到主分支。  
- 本地 worktree 和 Hub clone 模式都遵循“分支开发 → 人审/CI → 合并”的闭环。

## 4. Agent 如何与人/其他 Agent 交互

### 人与 Agent

- `scion attach <agent>`：进入会话实时协作。  
- `scion message <agent> "..."`：追加指令。  
- `scion messages`：查看消息收件箱（尤其适合长任务）。

### Agent 与 Agent

- Agent 可在权限允许下创建/管理子 Agent（scope 例如 `grove:agent:create`、`grove:agent:lifecycle`）。  
- 系统在 token 与权限模型中记录 ancestry（祖先链），用于跨子 Agent 的传递式访问控制。

## 5. 项目隔离与认证系统（用户视角）

### 5.1 项目与运行隔离

- Grove 是项目边界，Agent 默认在隔离容器里运行。  
- 当完整仓库根目录挂载进容器时，系统会用 tmpfs 覆盖容器内的仓库根目录 `.scion`（挂载点路径为 `/repo-root/.scion`），减少跨 Agent 读取敏感目录的风险。  
- 在 shared-workspace 场景下，每个 Agent 的状态文件会放在共享工作区之外，降低同级 Agent 互读风险。

### 5.2 认证与鉴权（Hub）

- 统一认证中间件按顺序处理：Agent Token、Broker HMAC、开发 token、用户 PAT、用户 JWT 等。  
- Broker 到 Hub 调用会对请求规范串做 HMAC 签名。  
- timestamp、nonce 等参与签名；Hub 侧会校验签名结果，并检查时钟偏差和 nonce 重放。  
- Agent 访问 Hub 使用带 scope 的 JWT（含 `grove_id`、`scopes`、`ancestry`）。

## 6. 实现依据（代码即事实）

以下是本文对应的关键实现位置（便于核对）：

- Git clone/workspace 初始化：`cmd/sciontool/commands/init.go`
- GitHub 凭据 helper：`cmd/sciontool/commands/credential_helper.go`
- Broker 下发 clone 环境变量：`pkg/runtimebroker/start_context.go`
- Agent 侧共享工作区 git 凭据配置：`pkg/agent/provision.go`
- 隔离（tmpfs 覆盖 `.scion`）：`pkg/runtime/common.go`
- Agent 路径与 shared-workspace 外置状态：`pkg/config/grove_marker.go`
- 统一认证顺序：`pkg/hub/auth.go`
- Agent Token（scope/ancestry）：`pkg/hub/agenttoken.go`
- Broker HMAC 签名校验：`pkg/hub/brokerauth.go`
