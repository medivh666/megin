# `/api` Article 开发规范与最佳实践

本文以 [`internal/api/article.go`](../internal/api/article.go)、[`internal/router/api.go`](../internal/router/api.go) 和 [`internal/module/article/options.go`](../internal/module/article/options.go) 为例，说明前台业务接口（`/api/...`）的推荐写法。

当前推荐方式是：在 router 注册接口时声明缓存、限速和分布式锁策略，由泛型路由包装器统一执行。Handler 不再手写这些通用控制逻辑。

缓存默认不启用。只有需求明确要求 Redis 缓存或本地缓存时才接入；限速和锁也应根据实际并发场景配置，不能机械地套给每个接口。

## 1. 推荐调用链

```text
Gin 请求
  -> router 绑定并校验请求 DTO
  -> 创建 api.Context
  -> 根据路由上的 RequestOption 生成本次请求策略
  -> 执行限速、缓存、锁等通用策略
  -> Handler
  -> Service
  -> Repository
  -> 统一响应
```

分层职责如下：

- `router`：区分鉴权分组、注册路由、挂载请求策略。
- `internal/api`：接收已校验参数、调用 Service、返回统一结果。
- `service`：处理业务规则和数据转换。
- `repository`：负责数据库访问。
- `internal/module/<module>/options.go`：集中声明模块的路由请求策略，避免 router 文件过长。

不要把缓存、限速、锁放进 Service 或 Repository，也不要为了配置策略修改 Handler 方法签名。

## 2. Handler 保持纯净

Article Handler 的推荐形式如下：

```go
func (this *Article) Detail(ctx *api.Context, req *base.BaseId) (*api.Result[dto.Article], error) {
	data, err := service.NewArticle(ctx).GetById(req.ID)
	if err != nil {
		return nil, err
	}
	return api.ResultData(data)
}
```

Handler 只做三件事：

1. 使用已绑定、已校验的请求参数。
2. 调用 Service。
3. 返回统一结果。

一般不应在 Handler 中出现下面这些样板代码：

```go
ctx.Limiter().AllowRequestOnce(...)
ctx.Locker().AcquireRequestLock(...)
api.RememberRedisStruct(...)
ctx.Redis().Delete(...)
```

当通用 `RequestOption` 无法表达特殊业务语义时，才在 Handler 或更合适的业务层显式使用底层能力，并说明原因。

## 3. 从 router 配置请求策略

`router.GET` 和 `router.POST` 可以在 Handler 后继续接收 `api.RequestOption`：

```go
router.GET(group, path, handler, options...)
router.POST(group, path, handler, options...)
```

Article 当前把策略集中定义在 `internal/module/article/options.go`，再在 router 中挂载：

```go
article := &api.Article{}
router.GET(noAuthGroup, "/article/detail", article.Detail, articleModule.DetailOptions...)
router.POST(noAuthGroup, "/article/create", article.Create, articleModule.CreateOptions...)
router.POST(noAuthGroup, "/article/update", article.Update, articleModule.UpdateOptions...)
router.POST(noAuthGroup, "/article/delete", article.Delete, articleModule.DeleteOptions...)
router.GET(noAuthGroup, "/article/pageList", article.PageList)
```

没有传 `RequestOption` 时，路由包装器直接执行 Handler，行为与普通接口一致。

简单策略也可以直接写在 router 中：

```go
router.GET(
	noAuthGroup,
	"/article/detail",
	article.Detail,
	requestapi.WithCache("article", []string{"id"}, time.Minute),
)
```

此时需要把 `megin/pkg/context/api` 以 `requestapi` 等别名导入，避免与 `megin/internal/api` 的 Handler 包重名。同一模块存在多个策略时，推荐像 Article 一样放进模块的 `options.go`，让路由文件保持简洁。

## 4. Article 的标准配置

```go
package article

import (
	"megin/pkg/context/api"
	"time"
)

var DetailOptions = []api.RequestOption{
	api.WithCache("article", []string{"id"}, time.Minute),
}

var CreateOptions = []api.RequestOption{
	api.WithRateLimit("article:create", []string{"title"}, true, 5*time.Second),
}

var UpdateOptions = []api.RequestOption{
	api.WithLock("article", []string{"id"}, 5*time.Second),
	api.WithDeleteCache("article", []string{"id"}),
}

var DeleteOptions = []api.RequestOption{
	api.WithLock("article", []string{"id"}, 5*time.Second),
	api.WithDeleteCache("article", []string{"id"}),
}
```

这四组配置分别表达：

- `Detail`：按文章 ID 读取 Redis 缓存；未命中时执行 Handler 并回填，TTL 为 1 分钟。
- `Create`：按用户 ID 和标题限速，5 秒内相同 key 只通过一次；无法取得登录用户时，用户 ID 为 `0`。
- `Update`：按文章 ID 加锁；成功后删除对应详情缓存。
- `Delete`：按文章 ID 加锁；成功后删除对应详情缓存。

`PageList` 当前没有策略，因此直接注册 Handler。普通查询接口不因为是 GET 就默认加缓存。

## 5. 四种 RequestOption

### 5.1 `WithCache`

```go
api.WithCache(prefix, parts, ttl)
```

声明 Redis 读穿透缓存：

1. 执行 Handler 前读取缓存。
2. 命中时直接返回，不执行 Handler 和 Service。
3. 未命中时执行 Handler。
4. Handler 成功后缓存完整响应。

Article 详情配置生成的 key 为：

```text
cache:article:{id}
```

要求：

- 只用于适合缓存的查询接口。
- TTL 必须明确。
- 一个接口只配置一个读缓存策略。
- 写接口影响缓存数据时，必须同时设计失效策略。

当前 router 的 `RequestOption` 读穿透能力使用 Redis。需要本地缓存时，应按 [`docs/cache-topic.md`](cache-topic.md) 中的现有方式显式实现，不能把 `WithCache` 当成本地缓存。

### 5.2 `WithDeleteCache`

```go
api.WithDeleteCache(prefix, parts)
```

声明 Handler 成功后删除 Redis 缓存。Handler 返回错误时不会删除。

Article 更新和删除配置生成的 key 与详情缓存一致：

```text
cache:article:{id}
```

缓存前缀和参与 key 的字段必须与查询接口完全一致，否则写操作不会删除正确的缓存。

### 5.3 `WithLock`

```go
api.WithLock(prefix, parts, ttl)
```

声明基于 Redis 的分布式锁。锁覆盖完整 Handler 调用，Handler 成功或失败后都会释放；锁被占用时返回业务错误 `429`，不会执行 Handler。

Article 更新和删除配置生成的 key 为：

```text
lock:article:{id}
```

适用场景：

- 同一资源的并发更新。
- 并发删除。
- 状态流转、确认、结算等临界操作。

锁 TTL 应覆盖正常业务执行时间，但不能无边界放大。只有需要保护同一资源并发一致性的写接口才配置锁。

### 5.4 `WithRateLimit`

```go
api.WithRateLimit(prefix, parts, withTokenUID, ttl)
```

声明固定时间窗内同一 key 只允许通过一次。首次通过后不会主动释放限速标识，TTL 内的后续请求返回业务错误 `429`，不会执行 Handler。

当 `withTokenUID=true` 时，key 会加入当前登录用户 ID。Article 创建配置生成的 key 为：

```text
limit:article:create:{userID}:{title}
```

适用场景：

- 创建、提交类接口防连续点击。
- 短时间重复请求拦截。
- 简单的匿名接口防刷。

它不是精确 QPS、复杂配额或长周期计费限额方案。

`withTokenUID=true` 应用于能取得登录用户的鉴权路由。未登录请求的用户 ID 为 `0`；匿名接口应使用 `false`，并通过 `parts` 选择足以区分请求的字段，或在特殊场景中显式设计 IP 等维度。

当前 Article 创建接口注册在 `noAuthGroup`，因此它无法取得登录用户时，实际按 `0 + title` 限速。如果业务目标是“按登录用户限速”，应把路由放入 `apiGroup`；如果必须匿名访问，则应重新设计限速维度。

## 6. key 生成规则

通用格式为：

```text
{kind}:{prefix}:{tokenUID（可选）}:{parts...}
```

其中：

- `kind` 由策略自动确定：`cache`、`lock` 或 `limit`。
- `prefix` 是业务前缀，例如 `article`、`article:create`。
- `tokenUID` 只在限速策略启用 `withTokenUID` 时加入。
- `parts` 按声明顺序从已绑定的请求 DTO 中取值，并进行 URL Path 转义。

字段查找支持请求字段的 `json`、`form`、`uri` tag，也支持不区分大小写的 Go 字段名。例如 `[]string{"id"}` 可以读取带有 `json:"id"` 或 `form:"id"` tag 的 `ID` 字段。

配置 key 时必须遵守：

- 使用请求 DTO 中真实存在的直接字段。
- 字段名优先写 tag 中对外暴露的名称，例如 `id`、`title`。
- 不要使用会变化但不影响资源唯一性的字段。
- 不要把敏感信息放入 key。
- 不要把所有用户或资源压到同一个固定 key。
- 查询缓存与写后删缓存必须使用相同的 `prefix` 和 `parts`。

字段不存在或无法读取时会产生空 key 片段，可能导致不同请求共用缓存、锁或限速标识，因此配置后必须核对 DTO。

## 7. 执行顺序和错误语义

当前统一执行顺序为：

```text
参数绑定与校验
  -> 限速检查
  -> 获取锁
  -> 读取缓存（如配置）
  -> Handler
  -> 回填缓存（如配置）
  -> 释放锁
  -> 成功后删除缓存（如配置）
  -> 返回响应
```

需要注意：

- 任一步返回错误，后续业务不再执行。
- 缓存命中时不会执行 Handler 和 Service。
- 限速命中或锁被占用时不会执行 Handler。
- 删除缓存只在前面的执行整体成功后发生。
- 缓存、锁、限速依赖 Redis；Redis 错误会直接返回，不会自动静默降级。

不要在 router 已配置策略后，又在 Handler 中重复加同类缓存、锁或限速，否则会产生重复 key、嵌套锁或不一致的失效行为。

## 8. 开发检查清单

新增或修改 `/api` 接口时，按下面顺序检查：

1. 路由是否按实际业务放入鉴权或免鉴权分组。
2. Handler 是否只保留参数、Service 调用和统一响应。
3. 是否确实需要缓存、限速或锁；缓存默认不启用。
4. 能否通过 router 的 `RequestOption` 表达通用策略。
5. `parts` 是否能从请求 DTO 的直接字段正确取值。
6. 写接口是否需要删除相关查询缓存。
7. `withTokenUID=true` 的路由是否能取得登录用户。
8. 锁和限速的 TTL 是否符合业务执行时间与防重窗口。
9. 是否复用了模块 `options.go` 的现有组织方式。
10. 改动 `.go` 文件后执行 `go build ./...`；纯文档改动不强制编译。

## 9. 核心原则

- 通用请求控制优先在 router 声明，不在 Handler 中堆样板代码。
- Handler、Service、Repository 保持各自分层边界。
- 缓存按需启用，并同时考虑写后失效。
- 限速解决时间窗内重复请求，锁解决执行期间的并发互斥，两者不能混用。
- key 必须由稳定、可区分业务对象的请求字段构成。
