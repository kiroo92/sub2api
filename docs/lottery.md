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

抽奖使用独立 Cloudflare Turnstile，与系统登录、注册 CAPTCHA 完全分开。在 Cloudflare Turnstile 创建专用于抽奖的 Managed widget，添加本站域名，然后在「抽奖活动管理」填写 Site Key 和 Secret Key。无需开启系统验证码。

Secret Key 不返回到任何接口响应；编辑时留空保留已有密钥。验证配置立即生效，无需等待新一期。升级自动执行 `239_lottery_turnstile.sql`。配置完成前暂停参与入口。

用户在参与按钮旁完成自动或点击验证，之后提交 `{ "round_id": 1, "turnstile_token": "…" }`。后端只读取抽奖配置中的密钥，通过 Cloudflare Siteverify 校验；缺失、过期、重复使用的票据及服务异常均阻止参与。每次提交后重置验证码。Cloudflare 域名限制需要在对应 widget 设置中配置。
