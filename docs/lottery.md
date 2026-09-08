# 抽奖活动

用户入口：`/lottery`；管理员入口：`/admin/lottery`。侧边栏已增加对应菜单，简易模式隐藏并阻止访问。

首次升级后活动默认关闭。管理员保存开启配置后生成第一期。初始规则为单份奖金 $5、6 个中奖名额、60 人开奖、累计充值门槛 $50；金额沿用平台余额单位。

每位用户每期免费参与一次。以系统已有 `users.total_recharged` 判断参与门槛，抽奖奖金只增加余额，不增加累计充值。中奖资格以参与时的状态和本期门槛为准。当前期的奖金、名额、人数及门槛固定；修改规则从下一期开始使用。关闭开关暂停新参与，保留当前期和全部参与记录，重新开启后继续当前期。

最后一个名额参与成功时，同一事务内完成参与记录、密码学随机抽取、余额入账、中奖记录和新一期创建。使用配置行锁串行化多实例操作，并以 `(round_id, user_id)` 唯一约束防重复参与。客户端必须提交显示的 `round_id`，旧期重试不会自动加入新一期。发奖失败回滚整笔事务，原参与者可重试。账户余额使用 PostgreSQL NUMERIC 运算，中奖记录作为发奖凭证。

用户页展示最近 20 个中奖记录、自己的最近 20 次中奖及最近 20 期。中奖邮箱在服务端脱敏，响应不暴露其他用户 ID 或完整邮箱。页面可手动刷新，并每 15 秒刷新可见页面。

接口：

- `GET /api/v1/lottery`：当前用户的活动概况及记录，需要 JWT。
- `POST /api/v1/lottery/join`：`{"round_id": 1}`，需要 JWT，身份从会话获取。
- `GET /api/v1/admin/lottery`：活动配置和当前期概况，需要管理员身份。
- `PUT /api/v1/admin/lottery/config`：完整配置，包含 `enabled`、`prize_amount`、`winner_count`、`participant_target`、`min_recharge`，需要管理员身份。

部署新版后启动流程自动执行 `238_lottery.sql`，创建活动配置、期数、参与及中奖记录表。该迁移可重复执行。

验证：

```powershell
# 指向专用临时 PostgreSQL。测试为每次运行创建独立 schema，结束后删除。
$env:LOTTERY_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55438/postgres?sslmode=disable'
# 在 backend 目录
 go test ./internal/repository ./internal/service ./internal/handler -run TestLottery -count=1
# 在 frontend 目录
 pnpm exec vitest run src/views/user/__tests__/LotteryView.spec.ts
 pnpm build
```

数据库测试覆盖并发满员开奖、重复请求、旧期请求、暂停与恢复、次期规则、充值门槛、精确奖金入账、充值累计值不变、发奖故障的完整回滚和迁移重放。余额缓存发奖后失效，API Key 鉴权缓存另有事务内持久化失效通知。

参与抽奖强制执行腾讯天御或阿里云动作式 CAPTCHA。先在系统设置启用并完整配置其中一个服务商，并在服务商控制台选择滑动验证场景。点击参与后弹出验证码，取消则不提交，提交后重置一次性票据。后端在执行参与事务之前验证票据，缺失、失效、配置缺失或服务异常均阻止参与。仅开启 Turnstile 不满足抽奖滑动验证配置。

参与请求除 `round_id` 外，腾讯使用 `tencent_captcha_ticket`、`tencent_captcha_randstr`；阿里云沿用已有请求协议，将 `captchaVerifyParam` 放入 `turnstile_token`。验证码配置从公开配置接口读取，密钥始终保留在服务端。
