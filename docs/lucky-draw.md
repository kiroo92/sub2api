# 幸运抽奖

用户入口 `/lucky-draw`，管理员入口 `/admin/lucky-draw`，侧边栏提供入口，简易模式禁用。独立于原有满员开奖活动。

首次升级后默认关闭。管理员可配置活动开关、每次所需累计充值额、连续抽奖保底次数和 2–12 项奖品。默认每累计充值 $50 获得一次机会，最多连续 10 次获得余额奖品。默认奖池为 $0.50（40%）、$1（20%）、$5（5%）、$20（1%）和未中奖（34%）。管理员启用前应确认这些示例配置符合实际活动规则。

金额单位沿用账户余额。次数按本站 `users.total_recharged` 计算，包含历史充值和现有规则计入该字段的其他加款，不等同于纯支付净充值。可用次数 = max(0, floor(累计充值 / 门槛) - 已抽次数)，零头保留；退款等导致累计下降时不返还已用次数。首次产生抽奖记录后充值门槛锁定，防止重新折算已用次数。中奖奖金只增加余额，不增加充值累计。

翻牌子、转盘、滚轮共用同一服务端概率和结果。普通抽奖按整数权重随机，总权重必须为 10000。奖品金额 0 代表未中奖，至少一个正金额奖品。保底次数为 N 时，累计连续 N−1 次未中奖后，下一次只在正金额奖品中按相对权重选择，任何中奖都重置计数。N=1 表示每次必中，N=0 关闭保底。转盘扇区大小用于展示，不表示中奖概率。

活动开关、奖池和保底更新对新的抽奖立即生效，已经发放的奖品保留原值。已完成请求的重试总是返回原记录，即使活动已暂停或规则已更改。暂停不删除次数和历史记录。保底未中奖计数在暂停和规则更新后保留。

后端事务锁定配置及用户余额，检查次数、密码学随机抽奖、保存奖品和规则快照、扣次数、记录保底并发放余额。任一失败整体回滚。同一用户和请求 UUID 唯一；客户端提交当前配置版本，旧版本的新请求拒绝且不消耗次数。客户端会按用户保存待确认请求，超时或刷新后用原 UUID 确认结果。发奖事务同时写入鉴权缓存失效 outbox，提交后失效余额和鉴权缓存。

迁移 `248_lucky_draw.sql` 创建独立配置、用户进度和抽奖记录表，启动时自动执行。使用明细清理不影响活动数据。

接口均需要认证，管理接口还需要管理员身份：

- `GET /api/v1/lucky-draw`：当前用户次数、保底进度、奖池及最近 20 次记录。
- `POST /api/v1/lucky-draw/draw`：`{"request_id":"UUID", "config_version":2}`，身份取自会话；受现有面板限流控制。
- `GET /api/v1/admin/lucky-draw`：配置。
- `PUT /api/v1/admin/lucky-draw/config`：完整配置及当前 `version`，成功后版本递增；并发编辑旧版本拒绝，刷新后重新保存。

验证须串行执行，并通过容器或 systemd cgroup 限制 CPU 为 0.5 核、内存为 2GB，禁用 swap。Go 使用单进程编译，Vitest 使用单 worker。后端数据库地址必须指向专用测试库，测试创建并清理独立 schema。以下命令在已施加上述限制的环境中执行：

```sh
cd backend
export GOMAXPROCS=1 GOMEMLIMIT=1200MiB GOGC=75
export LOTTERY_TEST_DATABASE_URL='postgres://postgres:password@127.0.0.1:55449/postgres?sslmode=disable'
go test -p 1 -gcflags='github.com/Wei-Shaw/sub2api/...=-N -l' internal/service/lucky_draw_service_test.go
go test -p 1 -gcflags='github.com/Wei-Shaw/sub2api/...=-N -l' ./internal/repository ./internal/handler ./internal/server/routes ./cmd/server -run 'Test(LuckyDraw|Lottery)' -count=1
cd ../frontend
./node_modules/.bin/vitest run src/views/user/__tests__/LuckyDrawView.spec.ts src/views/user/__tests__/LotteryView.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts --maxWorkers=1 --minWorkers=1
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build --config vite.config.ts
```

上述 Go 验证命令关闭测试构建的优化和内联以降低编译内存峰值。前端 `build` 脚本先运行类型检查，Vite 插件只在开发时检查类型，避免在打包时重复启动检查进程。脚本显式使用 `vite.config.ts`，避免历史生成的 `vite.config.js` 优先被读取。本机最后一次全量类型复查触及 Node 堆内存上限，未完成；应在验证报告中单独记录，不能把单独打包成功视为全量类型检查成功。

本次验证结果：新增服务规则测试、PostgreSQL 并发／保底／回滚测试、接口身份校验测试通过；原有满员抽奖的仓储和接口回归测试通过；路由和服务入口编译检查通过。前端幸运抽奖 9 项、原抽奖 11 项和语言键检查 3 项均通过。最终前端生产打包在 0.5 核、2GB 内存、禁用 swap 的限制下通过，并检查产物包含最终修改；全量类型复查受内存限制未完成。测试数据库在验证后删除，线上未启用活动。
