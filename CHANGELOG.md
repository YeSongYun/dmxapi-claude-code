# 更新日志

所有版本的更新内容记录。













## 1.8.8 (2026-07-08)

### 🐛 问题修复

- 修复清除配置功能的10个缺陷,含菜单未清VSCode的用户反馈根因

### 📖 文档

- 版本号同步清单新增 README.md
- Update CHANGELOG.md for v1.8.7
- 更新 README 至 v1.8.7 实际功能与用法
- Update CHANGELOG.md for v1.8.6

### 📝 其他更新

- Merge branch 'main' of https://cnb.cool/dmxapi/dmxapi_claude_code
- Merge branch 'main' of https://cnb.cool/dmxapi/dmxapi_claude_code

### 🔨 杂项

- 发布 v1.8.8

## 1.8.7 (2026-07-02)

### 🐛 问题修复

- 修复 L2 模型菜单超出终端高度导致的顶部框线残留

### 🔨 杂项

- Bump version 1.8.6 → 1.8.7

### 🚀 新功能

- 新增 Fable 模型配置位并引入 Claude 5 系列模型

## 1.8.6 (2026-06-17)

### 🐛 问题修复

- 清除当前配置后直接退出而非返回选择配置页

### 📖 文档

- Update CHANGELOG.md for v1.8.5

### 🔨 杂项

- Bump version 1.8.5 → 1.8.6

## 1.8.5 (2026-06-17)

### 📖 文档

- Update CHANGELOG.md for v1.8.4
- Update CHANGELOG.md for v1.8.3

### 📝 其他更新

- Merge remote-tracking branch 'origin/main'
- Merge remote-tracking branch 'origin/main'

### 🔨 杂项

- Bump version 1.8.4 → 1.8.5

## 1.8.4 (2026-06-16)

### 🔨 杂项

- Bump version 1.8.3 → 1.8.4

### 🚀 新功能

- 新增"清除当前配置"，保留已保存配置

## 1.8.3 (2026-06-08)

### 📖 文档

- Update CHANGELOG.md for v1.8.2

### 🔨 杂项

- Bump version 1.8.2 → 1.8.3

### 🚀 新功能

- 每次配置时清除历史错误写入的 _NAME 环境变量键

## 1.8.2 (2026-06-05)

### 📖 文档

- Update CHANGELOG.md for v1.8.1
- Update CHANGELOG.md for v1.8.0

### 🔨 杂项

- Bump version 1.8.1 → 1.8.2
- Bump version 1.8.0 → 1.8.1

### 🚀 新功能

- 思考等级启用值改为 max 并三处同步写入

## 1.8.1 (2026-06-04)

### 🔨 杂项

- Bump version 1.8.0 → 1.8.1

### 🚀 新功能

- 思考等级启用值改为 max 并三处同步写入

## 1.8.0 (2026-06-02)

### 🐛 问题修复

- 顶层 effortLevel 与 env 同步写入并归一历史 ultracode

### 📖 文档

- Update CHANGELOG.md for v1.7.9

### 🔨 杂项

- 发布 v1.8.0

## 1.7.9 (2026-06-02)

### 🐛 问题修复

- 补全 HOME 隔离的 Windows 分支并修正 manageNamedConfig 过时注释
- 删除命名配置后返回主菜单列表，而非直接退出程序
- CHANGELOG 改用「上次已写入版本..本次 tag」区间生成，避免漏版本

### 📖 文档

- 补全 CHANGELOG 缺失版本 v1.5.2~v1.7.6
- Update CHANGELOG.md for v1.7.8

### 🔨 杂项

- 发布 v1.7.9

### 🚀 新功能

- 新增配置流程自动启用思考等级 ultracode

### 🧪 测试

- 隔离 TestBuildVSCodeEnvVars 的 HOME，避免读到开发机 settings.json

## 1.7.8 (2026-06-01)

### 📖 文档

- Update CHANGELOG.md for v1.7.7

### 🔨 杂项

- 升级版本号至 1.7.8

### 🚀 新功能

- 配置摘要新增「Git 署名」状态行

## 1.7.7 (2026-06-01)

### 🐛 问题修复

- 署名配置菜单新增「完成配置」出口项
- CHANGELOG 回推改用 CNB 内置凭据并加 GPG 签名

### 🔨 杂项

- 升级版本号至 1.7.7

## 1.7.6 (2026-06-01)

### 🚀 新功能

- 新手引导流程默认开启 dmxapi 的 Git 提交署名
- 新增 Claude Code 的 Git 署名配置（提交与 PR 可分别设置）

### 🐛 问题修复

- 修复编辑配置时「配置 Git 署名」后无法保存的问题

### 🔨 杂项

- 升级版本号至 1.7.6

## 1.7.5 (2026-06-01)

### 🎨 代码风格

- 菜单内容与右边框之间增加 2 列间距，显示更清爽

## 1.7.4 (2026-06-01)

### 🐛 问题修复

- 选中与装饰图标改用 ASCII 字符，彻底修复选中行右边框错位

## 1.7.3 (2026-06-01)

### 🐛 问题修复

- 菜单分隔符由「·」改为「|」，修复对齐问题

## 1.7.2 (2026-06-01)

### 🐛 问题修复

- 修复含分隔符的菜单行右边框错位问题

## 1.7.1 (2026-06-01)

### 🐛 问题修复

- 修复超长标题导致菜单崩溃的问题
- 菜单边框按最长一行动态加宽，修复长名称时右边框对不齐

### 🔨 杂项

- 发布 v1.7.1

## 1.7.0 (2026-05-30)

### 🐛 问题修复

- 测试连接时自动去除模型名末尾的 [1m] 标记
- 模型配置页新增「完成模型配置」出口，修复向导卡在无限循环
- 全面修复无法返回上一页的问题，统一用 ESC 键返回

### 🧪 测试

- 隔离环境变量，修复本机测试失败

### 🔨 杂项

- 发布 v1.7.0

## 1.6.9 (2026-05-29)

### 🐛 问题修复

- 编辑命名配置时，在 URL 第一步按 ESC 返回不再误保存

### 🔨 杂项

- 发布 v1.6.9

## 1.6.8 (2026-05-29)

### 🚀 新功能

- 补齐配置名称与推荐配置流程的返回功能

### 🐛 问题修复

- 修正中文输入退格时的删除残影问题

### 🔨 杂项

- 发布 v1.6.8

## 1.6.7 (2026-05-29)

### 🚀 新功能

- 新增配置时将「填写配置名称」步骤提前到模式选择之后

### ♻️ 代码重构

- 新增配置直接走完整流程，命名配置管理新增「编辑」功能

### 🐛 问题修复

- 输入结束（EOF）时优雅退出，避免界面无限刷屏

### 🔨 杂项

- 发布 v1.6.7

## 1.6.6 (2026-05-29)

### 🚀 新功能

- 新增命名配置永久保存功能
- 推荐配置默认内置模型升级为 claude-opus-4-8-cc[1m]

### 🐛 问题修复

- 取消「清除所有配置」时不再误删命名配置
- 推荐配置文案中 Claude Opus 4.7 同步更新为 4.8
- 修复清除 effort 设置时累积多余换行的问题
- effort 默认值改为 ultracode，并修复禁用时清除不彻底

### 🎨 代码风格

- 格式化修正测试文件的对齐

### 🔨 杂项

- 发布 v1.6.6

## 1.6.5 (2026-04-29)

### 🔨 杂项

- 升级版本号至 1.6.5

## 1.6.4 (2026-04-29)

### 🐛 问题修复

- 修复更新日志推送时动态注入 CNB 凭据的问题

### 🔨 杂项

- 升级版本号至 1.6.4

## 1.6.3 (2026-04-24)

### 🐛 问题修复

- 测试连接时不再附加模型名后缀，升级版本号至 1.6.3

## 1.6.2 (2026-04-24)

### 🐛 问题修复

- 修复更新日志推送时 CNB 仓库地址多余的 .git 后缀

### 🔨 杂项

- 升级版本号至 1.6.2

## 1.6.1 (2026-04-24)

### 🔨 杂项

- 升级版本号至 1.6.1

## 1.6.0 (2026-04-17)

### 🚀 新功能

- 加固跨 Shell、编辑器、WSL 与 Windows 的「清除所有配置」功能
- 增强对旧版 cmd/PowerShell 的 Windows 兼容性
- 刷新预设模型列表并显示折扣提示
- 顶部菜单重构为三条一键直达路径

### 🔨 杂项

- 升级版本号至 1.6.0

## 1.5.8 (2026-04-01)

### 🚀 新功能

- 新增强制配置选项，并修复 Windows CMD 安装命令

## 1.5.7 (2026-04-01)

### 🐛 问题修复

- 增强 Windows 环境变量清理的可靠性

### 🔨 杂项

- 升级版本号至 1.5.7

## 1.5.6 (2026-04-01)

### 🐛 问题修复

- 增强 Windows Shell 兼容性

### 🔨 杂项

- 升级版本号至 1.5.6

## 1.5.5 (2026-04-01)

### 🐛 问题修复

- 修复环境变量清理兼容性，升级版本号至 1.5.5

## 1.5.4 (2026-03-31)

### 🚀 新功能

- 同步 Claude 设置与托管环境配置
- 按模型前缀路由 CNB AI 请求

## 1.5.3 (2026-03-22)

### 📖 文档

- 新增 Windows 环境变量写入可靠性的方案与设计文档

### 🔨 杂项

- 升级版本号至 1.5.3

## 1.5.2 (2026-03-16)

### 🚀 新功能

- 新增「清除所有配置」菜单项（选项 6）及相关清除功能

### 🐛 问题修复

- 增强 AI 更新日志流程的健壮性

### 📖 文档

- 新增清除所有配置功能的设计与实现文档，补全完整版本历史

### 🔧 CI/CD

- 在 tag 推送流程中加入 AI 增强的更新日志生成

### 🧪 测试

- 新增清除配置相关测试

### 🔨 杂项

- 升级版本号至 1.5.2

## 1.5.1 (2026-03-16)

### 🐛 问题修复

- Install.cmd 修复 LF 换行导致 CMD 静默失败，新增 ARM64 支持
- Install.ps1 将 Write-Host 中文替换为英文并在结束时关闭终端

### 🔨 杂项

- Bump version to 1.5.1
## 1.5.0 (2026-03-16)

### 🐛 问题修复

- Windows 统一显示 ASCII art Logo 和版本号
- Install.ps1 UTF-8 编码 + L1 模型菜单 Esc 后清除残留

### 🔨 杂项

- Bump version to 1.5.0

### 🚀 新功能

- 新增 stripJSONC，修复 VSCode JSONC 格式 settings.json 解析失败
## 1.4.9 (2026-03-16)

### 🐛 问题修复

- 修复 PowerShell iwr|iex 下 TUI 无法正常显示的问题

### 🔨 杂项

- Bump version to 1.4.9
## 1.4.8 (2026-03-15)

### 🐛 问题修复

- 用 FIONREAD ioctl 替换 poll 超时，修复 curl|bash 下 ESC 失效问题

### 🔨 杂项

- Bump version to 1.4.8
## 1.4.7 (2026-03-15)

### ♻️ 代码重构

- ConfigureAgentTeams 新增 exitOnDone 参数

### 🐛 问题修复

- 修正 VSCode 配置键名为 claudeCode.environmentVariables，兼容旧键检测
- Curl | bash 模式下交互菜单 stdin 重定向到终端
- 修正 raw 文件 URL 为 CNB 正确格式 /-/git/raw/
- 模式5复用外层 cfg，移除冗余 loadExistingConfig 调用
- GetWindowsHomeFromWSL cmd.exe 参数修正，环境变量正确展开
- VscodeSettingsPathFor Windows 分支改用 strings.Join

### 📖 文档

- README 新增快速安装章节
- 新增 CLAUDE.md，记录发版时需同步修改的版本号清单

### 📝 其他更新

- 合并来自 feat/release-clean-keep-recent-3 的合并请求 #2

### 🔧 CI/CD

- GitHub Release 说明新增快速安装章节（镜像同步）
- CNB Release 说明新增快速安装章节
- 添加 release-clean 步骤，保留最近 3 个版本

### 🔨 杂项

- Bump version to 1.4.7
- 将 .worktrees/ 加入 .gitignore
- 版本号升级 v1.4.5 → v1.4.6

### 🚀 新功能

- Mode 1 先配置模型再验证 API 连接
- 配置摘要新增 Agent Teams 和 VSCode Plugin 状态行
- 将 Agent Teams/VSCode 问询提前到 API 验证后、模型配置前
- 新增 install.cmd Windows CMD 一键安装脚本
- 新增 install.ps1 Windows PowerShell 一键安装脚本
- 新增 install.sh Linux/macOS 一键安装脚本
- 主菜单新增模式5，模式1追加 Agent Teams 和 VSCode 可选配置
- 新增 configureVSCode 交互函数
- 新增 winPathToWSL、getWindowsHomeFromWSL、getVSCodeSettingsPath、saveVSCodeConfig I/O 函数
- 新增 mergeVSCodeSettings 纯函数及测试
- 新增 buildVSCodeEnvVars 纯函数及测试
- 新增 vscodeSettingsPathFor 纯函数及测试
## 1.4.5 (2026-03-13)

### ♻️ 代码重构

- 提取 wslContentMatches 辅助函数，解耦 isWSL() 的 I/O 与逻辑层
- 提取 renderConfirmMenuCore，支持自定义 labels/descs

### 🐛 问题修复

- 将版本检查读取限制从 64KB 提升至 256KB
- 补充 wslContentMatches 文档注释，统一 bash/zsh 路径末尾换行处理
- Fish shell 值加引号，修正 fishMarker 注释，fish universal 变量无需 source
- 规范 REG ADD 错误消息格式，去除嵌入换行，统一前缀
- 规范 setEnvVarsWindows REG ADD 错误消息格式
- Windows setEnvVarsWindows 超长值改用 REG ADD，绕过 setx 1024字节限制
- 修复 visibleLength ANSI 解析，支持所有 CSI 终止字节（0x40-0x7E）
- 首次配置时为 cfg.Model 补填默认值，修正 SplitN 参数和注释
- 修复 fetchLatestVersion 响应截断问题，补充 compareVersions 语义注释
- 修复 removeEnvVar 文件权限保留和错误传播问题

### 📖 文档

- 将 macOS xattr 安全解除步骤纳入标准安装流程

### 🔨 杂项

- 版本号调整 v1.5.0 → v1.4.5
- 版本号升级 v1.4.4 → v1.5.0

### 🚀 新功能

- 新增 detectShellProfile()，根据 \$SHELL 自动检测 shell 并写入对应配置文件
- 新增 isWSL() 检测，WSL 环境下显示额外提示信息
- Agent Teams 页面新增功能介绍、状态改为红色未开启、菜单改为启用/禁用
- 新增 runEnableDisableMenu，用于启用/禁用语义菜单
- 主菜单和修复菜单改为键盘上下箭头选择
- 新增版本更新检查，修正 appVersion 为 1.4.4，新增 compareVersions 单元测试
- 新增主菜单选项4，支持启用/禁用 Agent Teams 实验性功能
- 启动时检测 Claude Code 是否已安装
- API 验证使用用户配置的默认模型，增加修改模型名选项
## 1.4.4 (2026-03-06)

### 🐛 问题修复

- 跨平台兼容性优化（修复 Linux ARM64 编译 + CJK 显示 + 终端安全）

### 🚀 新功能

- 支持 ESC 键退出模型配置菜单
- 新增 qwen3.5-flash-cc 至预设模型列表，修复 runL2Menu 硬编码
## 1.4.3 (2026-03-06)

### 🐛 问题修复

- RenderL2Menu 行数和自定义索引改为动态计算
## 1.4.2 (2026-03-06)

### 🐛 问题修复

- Claude-opus-4-6 和 claude-sonnet-4-6 不带 -cc 后缀

### 🚀 新功能

- 新增 claude-haiku-4-5-20251001（无 -cc）至预设列表末尾
- 将 claude-opus/sonnet-4-6-cc 移至预设列表最上方
- 同时保留 -cc 和无后缀的 claude-opus/sonnet 模型
- 更新预设模型列表，新增 mimo-v2-flash-cc 并调整顺序
## 1.4.1 (2026-03-04)

### 🐛 问题修复

- 使用 ReadConsoleInputW 彻底修复 Windows 方向键无响应
- 修复 Windows 控制台乱码和方向键失效问题

### 📖 文档

- 更新环境变量说明
## 1.4.0 (2026-03-03)

### 🐛 问题修复

- 在 raw 模式下将 renderL1Menu/renderL2Menu 的 \n 改为 \r\n
- 修复 renderL1Menu/renderL2Menu 菜单边框错位问题
- 优化选项 3 的文案，改为面向用户痛点的描述

### 📝 其他更新

- Merge branch 'main' of https://cnb.cool/dmxapi/dmxapi_claude_code
- 编辑文件 .cnb.yml

### 🚀 新功能

- 在配置界面显示遮盖格式的当前 Token
- 将 styledConfirm 的 y/N 输入改为箭头键导航菜单
- 实现交互式上下键导航模型配置菜单
- 优化菜单文案，选项 1 改为「从头配置」，选项 3 改为「解决 400 报错」
- 新增「仅禁用 Betas」配置模式（选项 3）
- 美化终端 UI，实现现代 CLI 风格界面
## 1.3.4 (2026-02-25)

### 📖 文档

- 同步更新 release.yml 和 .cnb.yml 中的安装说明
- 丰富安装说明，覆盖所有平台架构
- 更新发布工作流中的环境变量和文档链接
- 更新使用文档链接
## 1.3.3 (2026-02-25)

### 🐛 问题修复

- 修正 .cnb.yml 中的环境变量说明表格
## 1.3.2 (2026-02-25)

### 📝 其他更新

- 编辑文件 .cnb.yml
## 1.3.1 (2026-02-25)

### ♻️ 代码重构

- 将编译产物文件名中的 darwin 改为 macos

### 🐛 问题修复

- 修复 macOS zsh 配置兼容性问题

### 📖 文档

- 更新使用文档链接

### 📝 其他更新

- 编辑文件 .cnb.yml
- 编辑文件 .cnb.yml
- 编辑文件 .cnb.yml
- 删除文件 a.txt.txt
- 合并来自 main 的合并请求 #1

### 🔨 杂项

- 忽略通用二进制文件
- 无代码变更

### 🚀 新功能

- 添加 CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1 固定配置
- 更新默认模型常量为 Claude 4.6 版本
## 1.2.4 (2026-01-15)

### 🔧 CI/CD

- 更新 git-cliff 镜像为默认构建环境
## 1.2.3 (2026-01-15)

### 🔧 CI/CD

- 在生成更新日志步骤中安装 git
## 1.2.2 (2026-01-15)

### 🔨 杂项

- 在生成更新日志前获取所有标签
## 1.2.1 (2026-01-15)

### 🔧 CI/CD

- 优化生成更新日志的流程
## 1.2.0 (2026-01-15)

### 🔧 CI/CD

- 更新 git-cliff 镜像地址
## 1.1.9 (2026-01-15)

### 🔨 杂项

- 使用 git-cliff 替换 changelog 生成工具
## 1.1.8 (2026-01-15)

### 🚀 新功能

- 增强发布流程并添加自动生成变更日志功能
## 1.1.7 (2026-01-14)

### 🐛 问题修复

- 使用 git 命令直接推送标签到 GitHub
## 1.1.6 (2026-01-14)

### 🚀 新功能

- 更新CNB配置并添加版本清理功能
## 1.1.5 (2026-01-14)

### 🚀 新功能

- 添加同步标签到GitHub的步骤
## 1.1.4 (2026-01-14)

### 📖 文档

- 更新CNB配置文件的标题描述

### 🔧 CI/CD

- 添加 GitHub Actions 发布工作流
## 1.1.3 (2026-01-14)

### 🔧 CI/CD

- 调整 Release 创建和附件上传顺序
## 1.1.2 (2026-01-14)

### ⚡ 性能优化

- 优化 Windows 环境变量设置的并行处理

### 🚀 新功能

- 添加CNB云原生构建配置文件
- 为保存配置添加旋转动画和用户确认
- 优化环境变量设置逻辑，支持批量操作
- 添加配置模式选择功能
## refs/tags/1.0.3 (2026-01-09)

### 🐛 问题修复

- 移除API密钥的掩码显示

### 🚀 新功能

- 添加API连接验证失败时的交互式修复选项
## refs/tags/1.0.1 (2026-01-08)

### 📦 构建

- 添加跨平台编译脚本
## refs/tags/1.0.0 (2026-01-08)

### 📖 文档

- 添加项目 README 文档

### 🚀 新功能

- 添加DMXAPI Claude代码配置工具
- 添加Claude CLI交互式配置工具

