# Git 常用指令

## 规范

每次commit -m 的内容规范

| 前缀 | 全称 | 含义 | 触发版本变化 |
|---|---|---|---|
| `feat` | feature | 新增功能 | ✅ MINOR |
| `fix` | fix | 修复 bug | ✅ PATCH |
| `perf` | performance | 性能优化 | ✅ MINOR（或 PATCH） |
| `BREAKING CHANGE` | — | 破坏性/不兼容变更 | ✅ **MAJOR** |
| `security` | security | 安全修复 | ✅ PATCH |
| `docs` | documentation | 文档变更 | ❌ 单独不发版（API 文档变更可升 MINOR） |
| `style` | style | 格式/空格/缩进（不影响逻辑） | ❌ |
| `refactor` | refactor | 重构 | ❌ |
| `test` | test | 测试相关 | ❌ |
| `build` | build | 构建系统/依赖管理变更 | ❌（breaking 则 MAJOR） |
| `ci` | continuous integration | CI 配置 | ❌ |
| `chore` | chore | 杂项 | ❌ |
| `revert` | revert | 回滚 | 取决于被回滚内容的类型 |
| `deps` | dependencies | 依赖更新 | 取决于更新性质 |
| `i18n` | internationalization | 国际化 | ✅ MINOR |
| `a11y` | accessibility | 无障碍 | ✅ MINOR / PATCH |


## 远程关联流程

查看本地公钥
cat ~/.ssh/id_rsa.pub

设置并查看远程关联
git remote set-url origin git@github.com-hi-done:hi-done/clog.git && git remote -v

打标签并推送
git tag v1.0.1
git push --follow-tags





