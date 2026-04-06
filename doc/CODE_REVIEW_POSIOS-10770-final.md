# Code Review — POSIOS-10770 Final Report

**Date:** 2026-04-06  
**Branch:** `POSIOS-10770` vs `main`  
**Repo:** `elastic-search-service` (98 files changed, 5411 insertions, 4522 deletions)  
**Reviewers:** v1 (line-by-line) + v2 (skill.md checklist) + v3 (silent-failure-hunter + type-design-analyzer + feature-dev:code-reviewer)  
**Kết luận: BLOCKED — Cần fix trước khi merge.**

---

## So sánh v1 / v2 / v3

| | v1 | v2 | v3 | **Final** |
|-|----|----|----|-------|
| 🔴 MUST FIX | 14 | 7 | 13 | **29** |
| 🟡 SHOULD FIX | 10 | 7 | 6 | **20** |
| 🔵 SUGGEST | 6 | 4 | 5 | **12** |
| **Tổng** | **30** | **18** | **24** | **61** |

**Issues chỉ có trong v2 (missed by v1):**

| # | File | Vấn đề | Severity |
|---|------|---------|----------|
| [3b] | `product/service_test.go` | Production URL + password hardcoded | 🔴 NEW |
| [NEW-A] | `config/elastic_search.go` | `NewESConnection` per-request — HTTP pool leak | 🔴 NEW |
| [NEW-B] | `services/product/service.go:227` | `Bulk` error swallowed — returns "success" khi fail | 🔴 NEW |
| [NEW-C] | `services/job/service.go:154` | `err == redis.Nil` → must use `errors.Is` | 🟡 NEW |
| [NEW-D] | `worker/sync.go:92,233` | SyncProduct + SyncInventory xóa cùng 1 Redis key | 🟡 NEW |
| [NEW-E] | `worker/sync.go:410-413` | `fetchInventoryInNetSuite` return `nil` khi location empty | 🟡 NEW |
| [NEW-F] | `services/job/service.go:43,145,230` | Value receiver `(s Service)` không nhất quán | 🟡 NEW |

**Issues chỉ có trong v3 (missed by v1+v2):**

| # | File | Vấn đề | Severity |
|---|------|---------|----------|
| [v3-1] | `cmd/grpc.go:167` | `panic` trong goroutine + không gọi `GracefulStop` | 🔴 NEW |
| [v3-2] | `cmd/grpc.go:115-127` | `mysqlReadDsn`/`mysqlWriteDsn`/`kafkaBrokers` flags không register → gRPC không start | 🔴 NEW |
| [v3-3] | `cmd/cron.go:78,137` | Unbuffered signal channel + `cronJob.Stop()` không gọi | 🔴 NEW |
| [v3-4] | `migration/migration.go:8` | Không có version tracking — mọi migration re-run mỗi lần | 🔴 NEW |
| [v3-5] | `worker/sync.go:162` | `retries` chỉ reset cho Shopify — non-Shopify exhausts cross-page | 🔴 NEW |
| [v3-6] | `processor.go:120` | Điều kiện inverted logs EVERY inventory item | 🔴 NEW |
| [v3-7] | `product/service.go:47` | `Get()` trả `empty+nil` khi ES fail — false success | 🔴 NEW |
| [v3-8] | `product/service.go:153` | `Delete` loop: partial failures silently succeed | 🔴 NEW |
| [v3-9] | `product/service.go:175` | gRPC Dial failure trong SalesChannel mapping trả false success | 🔴 NEW |
| [v3-10] | `processor.go:78,161` | Redis checkpoint errors discarded → silent full re-sync | 🔴 NEW |
| [v3-11] | `processor.go:57` | `Fetch()` không có `ctx` parameter — uninterruptible | 🔴 NEW |
| [v3-12] | `cmd/rest.go:78` | `ListenAndServe` error via `fmt.Println` → zombie process | 🔴 NEW |
| [v3-13] | `cmd/rest.go:107,122` | `io.ReadAll` errors discarded → request body corruption | 🔴 NEW |
| [v3-14] | `inventory/builder.go:29` | `buildListInventoryItemFilter` mutates caller's request | 🟡 NEW |
| [v3-15] | `cmd/grpc.go:135` | `Hornet` flag registered nhưng không đọc vào `platform` struct | 🟡 NEW |
| [v3-16] | `model/store.go:1` | 200+ production store IDs hardcoded trong source | 🟡 NEW |
| [v3-17] | `repositories/respositories.go:12` | `UpdateStatus` nhận `string` thay vì `model.JobStatus` | 🟡 NEW |
| [v3-18] | `dto/sync_job.go:4` | `Status string`, `Page/Limit int32` allow negative | 🟡 NEW |
| [v3-19] | `product/service.go:172` | Two `grpc.Dial + defer conn.Close()` same scope — fragile | 🟡 NEW |
| [v3-20] | `services/job/service.go:31` | `NewService` không có nil guards | 🔵 NEW |
| [v3-21] | `model/sync_job.go:55` | `Type:""` possible tại construction, không validate | 🔵 NEW |
| [v3-22] | `model/sync_job.go:98` | Redis snapshot emit cả `metadata` và `metadata_json` | 🔵 NEW |
| [v3-23] | `config/redis.go` | Không có constructor, fields public | 🔵 NEW |
| [v3-24] | `model/sync_job.go:43` | `CanceledAt` field dead — `CancelJob` chưa implement | 🔵 NEW |

---

## 🔴 MUST FIX (16 issues)

### [1] `CancelJob` gọi `panic` — crash toàn bộ gRPC server
**File:** `internal/services/job/service.go:230-233`

```go
// HIỆN TẠI — bất kỳ client nào gọi CancelJob = server sập
func (s Service) CancelJob(ctx context.Context, request *jobPb.GetJobRequest) (*jobPb.Job, error) {
    //TODO implement me
    panic("implement me")
}
```

`CancelJob` đã được register vào gRPC server tại `cmd/grpc.go:165`. Một request duy nhất sẽ crash toàn bộ process.

**Fix:**
```go
func (s *Service) CancelJob(ctx context.Context, request *jobPb.GetJobRequest) (*jobPb.Job, error) {
    return nil, status.Error(codes.Unimplemented, "CancelJob not yet implemented")
}
```

---

### [2] REST server không gọi `srv.Shutdown` — requests bị drop khi stop
**File:** `cmd/rest.go:86-90`

```go
// HIỆN TẠI — nhận signal xong là return ngay, không drain
<-c
ctx, cancel := context.WithCancel(ctx)  // ctx bị shadow vô nghĩa
defer cancel()
fmt.Println("Server stopped")
// srv.Shutdown() KHÔNG BAO GIỜ được gọi
```

In-flight HTTP requests bị kill ngay khi nhận SIGTERM.

**Fix:**
```go
<-c
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
    fmt.Println("shutdown error:", err)
}
```

---

### [3] Hardcoded production credentials trong test files — security violation
**File:** `internal/services/inventory/service_test.go:14-18`  
**File:** `internal/services/product/service_test.go:14-19` ← **NEW (v2 thêm)**

```go
// inventory/service_test.go
service := NewService(&model.ESConfig{           // model.ESConfig không tồn tại → compile error
    Password:  "da9df3f&4a36fesgb3",             // hardcoded credential
})

// product/service_test.go
service := NewService(&model.ESConfig{           // model.ESConfig không tồn tại → compile error
    Addresses: []string{"https://acd.posify.io"}, // production URL hardcoded
    Password:  "82ARtK-Dq1EUbT1XiOtW",           // production password hardcoded
}, nil)
```

Password production đã commit vào git history — tồn tại mãi mãi kể cả sau khi xóa file. Cả 2 file đều không compile (sai package `model.ESConfig` thay vì `config.ESConfig`).

**Fix:**
1. Xóa cả 2 file khỏi branch (và rotate credentials ngay lập tức)
2. Nếu cần giữ, dùng `//go:build integration` và đọc từ env var:

```go
//go:build integration

func TestGetList(t *testing.T) {
    addr := os.Getenv("ES_ADDRESS")
    pass := os.Getenv("ES_PASSWORD")
    if addr == "" || pass == "" {
        t.Skip("integration test: ES_ADDRESS or ES_PASSWORD not set")
    }
    service := NewService(&config.ESConfig{  // config.ESConfig, không phải model.ESConfig
        Addresses: []string{addr},
        Password:  pass,
    })
}
```

---

### [4] gRPC-gateway registration chạy async — requests 404 khi server mới start
**File:** `cmd/rest.go:55-68`

```go
// HIỆN TẠI — gwmux chưa được wired khi srv.ListenAndServe() nhận requests
go func() {
    err := jobPb.RegisterJobServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts)
    if nil != err {
        os.Exit(1)  // os.Exit từ goroutine — không thể recover
    }
}()
mux.Handle("/", gwmux)  // gwmux chưa sẵn sàng
```

**Fix:** Register synchronous trước khi serve:
```go
if err := jobPb.RegisterJobServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
    return fmt.Errorf("failed to register gateway: %w", err)
}
mux.Handle("/", gwmux)
```

---

### [5] `cmd/migrate.go` thiếu `init()` — command không được đăng ký
**File:** `cmd/migrate.go`

```go
// HIỆN TẠI — migrateCmd được khai báo nhưng KHÔNG có init()
var migrateCmd = &cobra.Command{Use: "migrate", ...}
// KHÔNG CÓ: func init() { rootCmd.AddCommand(migrateCmd) }
```

`./bin/es-service migrate` báo "unknown command". DB schema không bao giờ được tạo, dẫn đến [6] và [7] không thể hoạt động.

**Fix:** Thêm vào cuối file:
```go
func init() {
    rootCmd.AddCommand(migrateCmd)
}
```

---

### [6] `Total` hardcoded 1000, progress tracking bị comment out hoàn toàn
**Files:** `internal/services/job/service.go:57` + `internal/worker/sync.go:99-103, 193-197`

```go
// service.go:57 — hardcoded, không bao giờ được update
Total: 1000,

// sync.go:99-103 — comment out hoàn toàn
//var (
//    keyJobTotal   = fmt.Sprintf(constant.KeyStoreSyncTotal, storeId, jobId)
//    keyJobSuccess = fmt.Sprintf(constant.KeyStoreSyncSuccess, storeId, jobId)
//    keyJobFailed  = fmt.Sprintf(constant.KeyStoreSyncFailed, storeId, jobId)
//)
```

Client polling `GetJob` luôn nhận `Total=1000, Succeeded=0, Failed=0`. Sync Job API hoàn toàn vô nghĩa.

---

### [7] `syncJobRepo.Create()` không bao giờ được gọi — DB persistence dead
**File:** `internal/services/job/service.go:43-142`

`Service.CreateJob` nhận `syncJobRepo` làm dependency nhưng không gọi `syncJobRepo.Create(ctx, syncJob)` ở bất kỳ đâu. Bảng `sync_jobs` luôn rỗng. Sau `ExpireTime = 8h`, job biến mất vĩnh viễn.

---

### [8] Repository methods là stubs — silently no-op
**File:** `internal/repositories/sync_job/repository.go:26-34`

```go
func (repo *SyncJobRepository) UpdateStatus(...) error { return nil }        // silent no-op
func (repo *SyncJobRepository) GetByID(...) (*model.SyncJob, error) { return nil, nil }  // always "not found"
func (repo *SyncJobRepository) Find(...) ([]*model.SyncJob, error) { return nil, nil }   // always empty
```

Caller nhận `nil, nil` nhưng nghĩ rằng operation thành công.

---

### [9] TOCTOU race — double-sync cho cùng 1 store
**File:** `internal/services/job/service.go:81-90`

```go
// HIỆN TẠI — không atomic: 2 concurrent requests đều thấy exists=0
exists, err := s.redisConfig.Writer.Exists(ctx, keyJobSyncing).Result()
if exists == 1 {
    return &jobPb.Job{}, errors.New("job syncing already exists")
}
// cả 2 tiếp tục → 2 Kafka events → double sync
pipeline.Set(ctx, keyJobSyncing, jobUUID, constant.ExpireTime)
```

**Fix:** Thay `Exists` + `Set` bằng `SetNX` (atomic):
```go
set, err := s.redisConfig.Writer.SetNX(ctx, keyJobSyncing, jobUUID, constant.ExpireTime).Result()
if err != nil { return nil, fmt.Errorf("CreateJob: check syncing: %w", err) }
if !set {
    return nil, errors.New("job syncing already exists")
}
```

---

### [10] Redis set TRƯỚC Kafka publish — lock orphan nếu Kafka fail
**File:** `internal/services/job/service.go:104-136`

```go
// Step 1: Set Redis keys — lock acquired và committed
pipeline.Set(ctx, keyJobSyncing, jobUUID, constant.ExpireTime)
_, err = pipeline.Exec(ctx)

// Step 2: Publish Kafka — có thể fail SAU KHI Redis đã set
err = s.writer.WriteMessages(ctx, kafka.Message{...})
if err != nil {
    return nil, err  // Redis keyJobSyncing vẫn tồn tại → store locked 8 giờ!
}
```

**Fix:** Publish Kafka trước, set Redis sau. Hoặc rollback Redis keys nếu Kafka fail:
```go
if err := s.writer.WriteMessages(ctx, msg); err != nil {
    // Rollback: xóa Redis key
    s.redisConfig.Writer.Del(ctx, keyJobSyncing)
    return nil, fmt.Errorf("CreateJob: publish event: %w", err)
}
```

---

### [11] `defer` xóa syncing key kể cả khi sync fail
**File:** `internal/worker/sync.go:91-98` (SyncProduct) và `sync.go:233-240` (SyncInventory)

```go
defer func() {
    isSyncingKey := fmt.Sprintf(constant.KeyStoreSyncJob, storeId)
    sw.redisConfig.Writer.Del(ctx, isSyncingKey)  // luôn xóa dù success hay failure
}()
```

Rate-limit hit sau 10 retries → sync abort → defer xóa key → Redis nói "not syncing" nhưng data trong ES không đầy đủ. Không có ghi nhận lỗi.

---

### [12] `Handler` loop spin CPU khi ctx bị cancel
**File:** `internal/worker/sync.go:59-83`

```go
for {
    msg, err := sw.reader.ReadMessage(ctx)
    if err != nil {
        logger.WithError(err).Error("failed to read message from broker")
        continue  // SIGTERM → ctx cancelled → ReadMessage err → loop vô tận, CPU burn
    }
}
```

**Fix:**
```go
if err != nil {
    if ctx.Err() != nil {
        logger.Info("context cancelled, stopping handler")
        return
    }
    logger.WithError(err).Error("failed to read message from broker")
    continue
}
```

---

### [13] `pkg/context/signal.go` chỉ catch `os.Interrupt`, thiếu `SIGTERM`
**File:** `pkg/context/signal.go:18`

```go
// HIỆN TẠI — chỉ SIGINT/Ctrl+C
signal.Notify(c, os.Interrupt)
```

Kubernetes gửi **SIGTERM** khi xóa pod. `WithInterruptSignal` không cancel context → worker không graceful shutdown → pod bị force-killed.

**Fix:**
```go
import "syscall"
signal.Notify(c, os.Interrupt, syscall.SIGTERM)
```

---

### [14] DB pool misconfigured — `MaxIdleConns > MaxOpenConns`
**File:** `internal/config/database.go:33-34`

```go
db.SetMaxIdleConns(15)  // 15 idle > 10 open → impossible state
db.SetMaxOpenConns(10)
```

`database/sql` silently caps idle xuống 10. Misleading khi debug performance.

**Fix:**
```go
db.SetMaxOpenConns(20)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(300 * time.Minute)
```

---

### [NEW-A] `NewESConnection` tạo mới ES client per-request — performance bottleneck nghiêm trọng ⭐ NEW
**File:** `internal/config/elastic_search.go:19`  
**Called from:** `internal/services/product/service.go:43,88,151`, `internal/services/inventory/service.go:24`, `internal/worker/sync.go:114,251`

```go
// Product.GetList() — tạo mới ES connection mỗi request
func (s *Service) GetList(ctx context.Context, r *ppb.ListProductRequest) ... {
    esConnection := config.NewESConnection(s.esConfig, merchantID)  // ← per request!
```

`NewESConnection` khởi tạo `elasticsearch.Config` và `connection.Connection` (bao gồm HTTP transport pool) cho mỗi gRPC request. Vi phạm nguyên tắc "không allocate trong hot path".

**Fix:** Cache `*connection.Connection` theo `merchantID` bằng `sync.Map`:
```go
type Service struct {
    esConfig  *config.ESConfig
    connCache sync.Map // key: merchantID, value: *connection.Connection
}

func (s *Service) getConn(merchantID string) *connection.Connection {
    if v, ok := s.connCache.Load(merchantID); ok {
        return v.(*connection.Connection)
    }
    conn := config.NewESConnection(s.esConfig, merchantID)
    actual, _ := s.connCache.LoadOrStore(merchantID, conn)
    return actual.(*connection.Connection)
}
```

---

### [NEW-B] `product.Service.Sync` — silently swallows Bulk error, logs "success" khi fail ⭐ NEW
**File:** `internal/services/product/service.go:227-233`

```go
err := esConnection.Product.Bulk(updateItems)
if nil != err {
    logEntry.WithError(err).Error("bulk index failed")
    // ← KHÔNG return! tiếp tục xuống dưới
}

logEntry.Info("sync product success")  // log "success" kể cả khi Bulk fail
return &ppb.SyncProductResponse{}, nil  // return nil error kể cả khi Bulk fail
```

Caller (worker) không biết data không được sync vào ES. Data loss xảy ra silently.

**Fix:**
```go
if err := esConnection.Product.Bulk(updateItems); err != nil {
    logEntry.WithError(err).Error("bulk index failed")
    return &ppb.SyncProductResponse{}, fmt.Errorf("sync product: bulk index: %w", err)
}
logEntry.Info("sync product success")
return &ppb.SyncProductResponse{}, nil
```

---

## 🟡 SHOULD FIX (14 issues)

### [15] `context.Background()` trong business logic — không propagate cancellation
**File:** `internal/worker/sync.go:145` (SyncProduct), `sync.go:290` (SyncInventory)

```go
result, err := platformService.GetList(context.Background(), filter)  // ← sai
```

Dùng `ctx` được truyền từ caller để SIGTERM có thể cancel in-flight gRPC calls.

---

### [16] Goroutine fan-out không giới hạn — no backpressure
**File:** `internal/worker/sync.go:79, 81`

```go
case string(model.JobTypeSyncProduct):
    go sw.SyncProduct(ctx, event.StoreId, event.JobId)
case string(model.JobTypeSyncInventory):
    go sw.SyncInventory(ctx, event.StoreId, event.JobId)
```

Mỗi Kafka message = 1 goroutine mới giữ 1 gRPC connection, pull hàng trăm nghìn items. Không có semaphore giới hạn concurrent goroutines.

---

### [17] `logger.Logger.WithFields(...)` drops context fields
**File:** `internal/worker/sync.go:314, 326`

```go
// HIỆN TẠI — .Logger truy cập raw *logrus.Logger, drop hết context fields
logger.Logger.WithFields(logrus.Fields{
    "filter": filter,
    "total_changed": len(result.GetItems()),
}).Info("fetch inventory data")
```

`logger` là `*logrus.Entry` với fields `store`, `processor`, `method`. `.Logger` lấy raw logger → mất hết context fields.

**Fix:** `logger.WithFields(...)` (bỏ `.Logger`)

---

### [18] "success" log sau error trong NetSuite location loop
**File:** `internal/worker/sync.go:419-424`

```go
for _, location := range listLocation.GetItems() {
    err = inventory.UpsertInventoryWorker(ctx, platformService, esConnection, filter)
    if err != nil {
        logger.WithError(err).Error("upsert inventory worker failed")
        // ← KHÔNG có continue/break
    }
    logger.Info("upsert inventory worker success")  // log "success" kể cả khi có error
}
```

**Fix:** `else { logger.Info("upsert inventory worker success") }`

---

### [19] `GetJob` — 3 Redis GET riêng lẻ thay vì MGet/pipeline
**File:** `internal/services/job/service.go:175-188`

```go
totalStr, err   := s.redisConfig.Reader.Get(ctx, keyJobTotal).Result()   // round-trip 1
successStr, err := s.redisConfig.Reader.Get(ctx, keyJobSuccess).Result() // round-trip 2
failedStr, err  := s.redisConfig.Reader.Get(ctx, keyJobFailed).Result()  // round-trip 3
```

**Fix:** Dùng `MGet`:
```go
vals, err := s.redisConfig.Reader.MGet(ctx, keyJobTotal, keyJobSuccess, keyJobFailed).Result()
```

---

### [20] `GetJob` — inconsistent error handling khi Redis key miss
**File:** `internal/services/job/service.go:152-188`

```go
// keyJobInfo miss → return empty Job, nil (OK)
if err == redis.Nil { return &jobPb.Job{}, nil }

// counter miss → return nil, err (500!)
totalStr, err := s.redisConfig.Reader.Get(ctx, keyJobTotal).Result()
if err != nil { return nil, err }  // redis.Nil → 500 error
```

Counter keys có thể expire trước info key. **Fix:** Treat `redis.Nil` là 0 cho counter keys.

---

### [21] `internal/broker/kafka.go` — `consumerAdmin` không check error → panic
**File:** `internal/broker/kafka.go:60-61`

```go
consumerAdmin, _ := kafka.NewAdminClientFromConsumer(c)  // error ignored
_, err = consumerAdmin.CreateTopics(...)  // nếu nil → PANIC
```

**Fix:**
```go
consumerAdmin, err := kafka.NewAdminClientFromConsumer(c)
if err != nil { return nil, fmt.Errorf("create admin client: %w", err) }
```

---

### [22] Kafka consumer group hardcoded sai domain
**File:** `cmd/sync.go:108` và `internal/broker/kafka.go:53`

```go
GroupID: "back-in-stock-worker",  // copy-paste từ service khác
"group.id": "myGroup",            // placeholder chưa đổi
```

Nếu share Kafka cluster → 2 services chia partition → cả 2 drop ~50% messages.

**Fix:** Dùng `"es-sync-worker"` hoặc đọc từ config.

---

### [23] `kafkaBrokers` flag chưa được đăng ký trong `cmd/sync.go`
**File:** `cmd/sync.go:103, init()`

`runSyncWorkerCmd` dùng `viper.GetString("kafkaBrokers")` nhưng `init()` không bind flag → chỉ có thể set qua env var. Tương tự `system-watermill-suffix` ở line 107.

---

### [24] `cmd/grpc.go` — MySQL là hard dependency của toàn bộ gRPC server
**File:** `cmd/grpc.go:115-125`

```go
ormReader, err := config.ConnectDatabase(viper.GetString("mysqlReadDsn"))
if err != nil {
    return  // category, product, inventory, customer chết theo
}
```

MySQL down → toàn bộ service không start, kể cả các domain không dùng MySQL.

---

### [NEW-C] `errors.Is` không được dùng để check `redis.Nil` ⭐ NEW
**File:** `internal/services/job/service.go:154`

```go
// HIỆN TẠI — không tương thích với error wrapping
if err == redis.Nil {

// Fix
if errors.Is(err, redis.Nil) {
```

go-redis v8 có thể wrap error. `==` so sánh trực tiếp không hoạt động với wrapped errors.

---

### [NEW-D] `SyncProduct` và `SyncInventory` xóa cùng key `KeyStoreSyncJob` trong defer ⭐ NEW
**File:** `internal/worker/sync.go:92` và `sync.go:233`

```go
// SyncProduct defer:
isSyncingKey := fmt.Sprintf(constant.KeyStoreSyncJob, storeId)
sw.redisConfig.Writer.Del(ctx, isSyncingKey)

// SyncInventory defer — CÙNG KEY:
isSyncingKey := fmt.Sprintf(constant.KeyStoreSyncJob, storeId)
sw.redisConfig.Writer.Del(ctx, isSyncingKey)
```

Nếu cùng 1 store trigger 2 jobs (product + inventory) đồng thời, goroutine xong trước sẽ xóa lock của goroutine kia → logic bảo vệ double-sync bị bypass.

**Fix:** Encode job type vào key: `connect-pos:store:{storeId}:is-syncing:{jobType}`, hoặc dùng Lua script để atomic check-and-delete (chỉ xóa khi value == jobId của mình).

---

### [NEW-E] `fetchInventoryInNetSuite` — return `nil` khi location empty thay vì error ⭐ NEW
**File:** `internal/worker/sync.go:410-413`

```go
if listLocation == nil || len(listLocation.GetItems()) == 0 {
    logger.Info("list location is empty")
    return err  // err == nil → caller nghĩ là success, nhưng 100k+ items KHÔNG được sync!
}
```

Khi `countItems > 100000` mà không có location → sync silently succeeds mà không sync được gì.

**Fix:**
```go
if listLocation == nil || len(listLocation.GetItems()) == 0 {
    return fmt.Errorf("fetchInventoryInNetSuite: no locations found, cannot sync large inventory (count=%d)", result.GetCountItems())
}
```

---

### [NEW-F] `job.Service` dùng value receiver — không nhất quán ⭐ NEW
**File:** `internal/services/job/service.go:43, 145, 230`

```go
// NewService trả *Service nhưng methods dùng value receiver
func (s Service) CreateJob(...)  // copy struct mỗi call
func (s Service) GetJob(...)
func (s Service) CancelJob(...)
```

Vi phạm rule "pointer receiver nhất quán trong 1 struct".

**Fix:** Dùng `(s *Service)` nhất quán cho tất cả methods.

---

### [25] Error không được wrap với context — mất stack trace
**File:** `internal/services/job/service.go` (toàn bộ service)

```go
// HIỆN TẠI
if err != nil { return nil, err }

// Fix
if err != nil { return nil, fmt.Errorf("CreateJob: marshal job info: %w", err) }
```

---

## 🔵 SUGGEST (7 issues)

### [26] `ioutil.ReadAll` / `ioutil.NopCloser` deprecated (Go 1.16+)
**File:** `cmd/rest.go:107-108, 122`

```go
// Thay
ioutil.ReadAll   → io.ReadAll
ioutil.NopCloser → io.NopCloser
```

---

### [27] `grpc.WithInsecure()` deprecated (gRPC-Go v1.27+)
**Files:** `cmd/rest.go:59`, `internal/worker/sync.go:122, 267, 363`

```go
// Thay
grpc.WithInsecure()
// → grpc.WithTransportCredentials(insecure.NewCredentials())
```

---

### [28] `model.JobStatusRunning = "processing"` — tên và value mâu thuẫn
**File:** `internal/model/sync_job.go:27`

```go
JobStatusRunning JobStatus = "processing"  // "Running" vs "processing"
```

Chọn một: đổi constant name → `JobStatusProcessing` hoặc đổi value → `"running"`.

---

### [29] `UpsertInventoryWorker` — logEntry stacking trong loop
**File:** `internal/services/inventory/worker.go:35`

```go
for {
    logEntry = logEntry.WithField("filter", filter)  // sau N vòng = N "filter" fields chồng
}
```

**Fix:** Move `WithField` ra ngoài loop hoặc tạo fresh entry mỗi vòng.

---

### [30] Duplicate constants — `constant.JobSync*` và `model.JobType*`
**Files:** `internal/constant/job.go` và `internal/model/sync_job.go`

```go
// constant/job.go
JobSyncProduct = "sync_product"

// model/sync_job.go  — TRÙNG
JobTypeSyncProduct JobType = "sync_product"
```

Xóa `constant.JobSync*`, dùng `model.JobType*` làm source of truth.

---

### [31] `InitLogger` — options bị ignore silently
**File:** `pkg/cpos_log/log.go:74`

```go
for _, option := range options {
    option(&o)  // modify o
}
Logger = NewLogger(logrusConf)  // ← dùng logrusConf gốc, KHÔNG phải o.OverrideLogrusConf!
```

Custom formatter/hooks/level truyền vào bị silently ignored.

**Fix:** `Logger = NewLogger(o.OverrideLogrusConf)`

---

### [32] Two Kafka client libraries trong cùng binary
**Files:** `cmd/sync.go` (segmentio/kafka-go) và `internal/broker/kafka.go` (confluent-kafka-go)

2 Kafka clients với 2 connection pools. Nếu `internal/broker/kafka.go` không còn được dùng, nên xóa để giảm binary size và dependency complexity.

---

## 🔴 MUST FIX — v3 additions (13 issues)

### [v3-1] `cmd/grpc.go` — `panic` trong goroutine + không gọi `GracefulStop` ⭐ NEW
**File:** `cmd/grpc.go:167-179`

```go
go func() {
    lis, err := net.Listen("tcp", address)
    if err != nil {
        panic(err)  // goroutine panic → crash toàn process, không thể recover
    }
    // ...
    if err = grpcServer.Serve(lis); err != nil {
        panic(err)
    }
}()
<-c
// grpcServer.GracefulStop() KHÔNG được gọi → in-flight RPCs bị drop
```

**Fix:**
```go
go func() {
    lis, err := net.Listen("tcp", address)
    if err != nil {
        cpos_log.Logger.WithError(err).Fatal("failed to listen on gRPC port")
    }
    if err := grpcServer.Serve(lis); err != nil {
        cpos_log.Logger.WithError(err).Error("grpc server error")
    }
}()
<-c
grpcServer.GracefulStop()
```

---

### [v3-2] `cmd/grpc.go` — `mysqlReadDsn` / `mysqlWriteDsn` / `kafkaBrokers` không được register vào CLI flags ⭐ NEW
**File:** `cmd/grpc.go:115-127`

```go
// grpcCmd.init() THIẾU binding → viper.GetString() luôn trả ""
ormReader, err := config.ConnectDatabase(viper.GetString("mysqlReadDsn"))  // "" → fail
ormWriter, err := config.ConnectDatabase(viper.GetString("mysqlWriteDsn")) // "" → fail
kafkaBrokers := strings.Split(viper.GetString("kafkaBrokers"), ",")        // [""]
```

Kết quả: gRPC server **không bao giờ start** khi deploy bằng CLI flags (DSN empty → `ConnectDatabase` error → early return).

**Khác với [23] (cmd/sync.go):** Đây là `grpcCmd.init()` độc lập, chưa được flag ở bất kỳ đâu.

**Fix:** Thêm vào `grpcCmd.init()`:
```go
grpcCmd.Flags().StringP("mysqlReadDsn",  "", "", "MySQL reader DSN")
grpcCmd.Flags().StringP("mysqlWriteDsn", "", "", "MySQL writer DSN")
grpcCmd.Flags().StringP("kafkaBrokers",  "", "127.0.0.1:9092", "Kafka brokers")
_ = viper.BindPFlag("mysqlReadDsn",  grpcCmd.Flags().Lookup("mysqlReadDsn"))
_ = viper.BindPFlag("mysqlWriteDsn", grpcCmd.Flags().Lookup("mysqlWriteDsn"))
_ = viper.BindPFlag("kafkaBrokers",  grpcCmd.Flags().Lookup("kafkaBrokers"))
```

---

### [v3-3] `cmd/cron.go` — unbuffered signal channel + `cronJob.Stop()` không được gọi ⭐ NEW
**File:** `cmd/cron.go:78, 137-138`

```go
c := make(chan os.Signal)  // ← unbuffered! grpc.go/rest.go/sync.go đều dùng buffer=1
// ...
<-c
ctx, cancel := context.WithCancel(ctx)
defer cancel()
// cronJob.Stop() KHÔNG BAO GIỜ được gọi → tất cả scheduled goroutines chạy mãi
```

Go docs: `signal.Notify` sẽ drop signal nếu channel không có buffer. Cron jobs không được Stop → goroutine/resource leak sau mỗi deployment.

**Fix:**
```go
c := make(chan os.Signal, 1)
// ...
<-c
cronJob.Stop()
cancel()
```

---

### [v3-4] `internal/migration/migration.go` — không có version tracking, mọi migration re-run mỗi lần ⭐ NEW
**File:** `internal/migration/migration.go:8-13`

```go
func Migrate(db *gorm.DB) error {
    if err := versions.Version20260121140000(db); err != nil {
        return err
    }
    return nil
    // Không có bảng schema_migrations, không track gì
}
```

Mỗi lần `./bin/es-service migrate` → tất cả versions re-execute. GORM AutoMigrate additive-safe hiện tại, nhưng bất kỳ future migration destructive nào sẽ chạy lại mỗi lần → data corruption.

**Fix:** Dùng `pressly/goose` hoặc tương đương để track applied versions.

---

### [v3-5] `internal/worker/sync.go` — `retries` chỉ reset cho Shopify, tích lũy cross-page với non-Shopify stores ⭐ NEW
**File:** `internal/worker/sync.go:162-164`

```go
// SyncProduct — conditional reset
if storeconnect.IsShopify(store.GetType()) {
    retries = 0  // non-Shopify: retries KHÔNG reset sau mỗi page thành công
}

// SyncInventory — unconditional (đúng)
retries = 0
```

BigCommerce/Magento store: page 3 gặp 4 transient errors → chỉ còn 6 retries cho toàn bộ sync → loop exit sớm, data bị cắt giữa chừng mà không có error rõ ràng.

**Fix:**
```go
retries = 0  // unconditional, nhất quán với SyncInventory
```

---

### [v3-6] `internal/services/inventory/processor.go` — inverted condition logs EVERY item ⭐ NEW
**File:** `internal/services/inventory/processor.go:120`

```go
// HIỆN TẠI — true với hầu hết items (non-nil OR empty = almost always true)
if item.BinNumbers != nil || len(item.BinNumbers) == 0 {
    logEntry.Info("fetch bin numbers success")
}
// Fix: log chỉ khi THỰC SỰ có bin numbers
if len(item.BinNumbers) > 0 {
    logEntry.Info("fetch bin numbers success")
}
```

1000 items per page × N pages → N×1000 log lines per sync run → severe performance degradation từ log I/O, log aggregation bị flood.

---

### [v3-7] `product/service.go` — `Get()` trả `empty Product + nil` khi ES fail ⭐ NEW
**File:** `internal/services/product/service.go:47-58`

```go
result, err := esConnection.Product.Get(r.GetId())
if nil != err {
    cpos_log.Logger.WithFields(...).WithError(err).Error("search product failed")
    return &prodpb.Product{}, nil  // ES down → caller nhận HTTP 200 với empty product
}
```

POS client không thể phân biệt "product not found" với "ES infrastructure down". Tương tự pattern ở `GetList` (`internal/services/product/service.go:88`).

**Fix:** Return gRPC error code thay vì false success:
```go
return nil, status.Errorf(codes.Internal, "failed to get product %s: %v", r.GetId(), err)
```

---

### [v3-8] `product/service.go` — `Delete` loop: partial failures silently succeed ⭐ NEW
**File:** `internal/services/product/service.go:153-165`

```go
for _, ID := range IDs {
    err := esConnection.Product.Delete(ID)
    if nil != err {
        logEntry.WithError(err).Error("delete document failed")
        // ← không accumulate, không return error
    }
}
return &ppb.SyncProductResponse{}, nil  // luôn success dù tất cả Delete đều fail
```

Products đã xóa trên Shopify sẽ tiếp tục tồn tại trong ES index vô thời hạn.

**Fix:** Accumulate và return failures:
```go
var failedIDs []string
for _, ID := range IDs {
    if err := esConnection.Product.Delete(ID); err != nil {
        logEntry.WithField("product_id", ID).WithError(err).Error("delete document failed")
        failedIDs = append(failedIDs, ID)
        continue
    }
    deleteProductsWorker(esConnection, ID)
}
if len(failedIDs) > 0 {
    return nil, status.Errorf(codes.Internal, "failed to delete products: %v", failedIDs)
}
```

---

### [v3-9] `product/service.go` — gRPC Dial failure trong SalesChannel mapping trả false success ⭐ NEW
**File:** `internal/services/product/service.go:175-183`

```go
conn, err := grpc.Dial(...)
if nil != err {
    logEntry.WithError(err).Error("init connection failed")
    return &ppb.SyncProductResponse{}, nil  // platform unreachable → HTTP 200
}
```

Platform service down → products sync vào ES **không có** channel mapping data. POS serves incorrectly mapped products silently.

**Fix:**
```go
if err != nil {
    return nil, status.Errorf(codes.Unavailable, "platform service unavailable: %v", err)
}
```

---

### [v3-10] `inventory/processor.go` — Redis checkpoint errors discarded → silent full re-sync loop ⭐ NEW
**File:** `internal/services/inventory/processor.go:78, 161, 236`

```go
// line 78: error bị discard hoàn toàn
updatedAtMin, _ := p.redisClient.Reader.Get(context.Background(), key).Result()

// line 161: return value của Set không được check
p.redisClient.Writer.Set(context.Background(), key, timeToFetch, redis.KeepTTL)
```

Redis down → `updatedAtMin = ""` → incremental sync trở thành full re-index. Checkpoint không được lưu → mỗi run sau đó cũng full re-index. Không có log nào.

**Fix:**
```go
updatedAtMin, err := p.redisClient.Reader.Get(ctx, key).Result()
if err != nil && !errors.Is(err, redis.Nil) {
    logEntry.WithError(err).Error("failed to read last fetch timestamp — performing full sync")
}

if err := p.redisClient.Writer.Set(ctx, key, timeToFetch, redis.KeepTTL).Err(); err != nil {
    logEntry.WithError(err).Error("failed to persist sync checkpoint — next run will full re-index")
}
```

---

### [v3-11] `inventory/processor.go` — `Fetch()` không có `ctx` parameter — uninterruptible ⭐ NEW
**File:** `internal/services/inventory/processor.go:57`

```go
func (p *Processor) Fetch() {  // không có ctx
    err := p.fetchInventoryInNetSuite(context.Background())  // không thể cancel
    result, err := platformService.GetList(context.Background(), filter)  // không thể cancel
```

Large store inventory fetch có thể chạy hàng giờ. SIGTERM không thể interrupt. Graceful shutdown chờ vô tận.

**Fix:**
```go
func (p *Processor) Fetch(ctx context.Context) {
    err := p.fetchInventoryInNetSuite(ctx)
    result, err := platformService.GetList(ctx, filter)
```

---

### [v3-12] `cmd/rest.go` — HTTP `ListenAndServe` error bị swallow → zombie process ⭐ NEW
**File:** `cmd/rest.go:78-84`

```go
go func() {
    fmt.Println("server started")  // log trước khi thực sự serve
    err := srv.ListenAndServe()
    if nil != err {
        fmt.Println(err)  // stdout only — không vào log aggregation
        // process tiếp tục! không exit
    }
}()
```

Port đã bị bind → ListenAndServe error → `fmt.Println` → process alive nhưng không serve → health check pass → load balancer routes traffic vào zombie.

**Fix:**
```go
go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        cpos_log.Logger.WithField("addr", addr).WithError(err).Fatal("HTTP server failed to start")
    }
}()
cpos_log.Logger.WithField("addr", addr).Info("REST server started")
```

---

### [v3-13] `cmd/rest.go:107,122` — `io.ReadAll` errors discarded → request body corruption ⭐ NEW
**File:** `cmd/rest.go:107-108, 122`

```go
buf, _ := ioutil.ReadAll(r.Body)   // error discarded → buf partial/empty
respBody, _ := ioutil.ReadAll(...) // error discarded → empty response logged
```

**Khác với [26] (chỉ flag deprecated):** Đây là bug nghiêm trọng hơn — error bị discard (`_`). Client disconnect mid-request → `buf` chứa truncated body → JSON unmarshal nhận payload sai → request được xử lý với data corrupt.

**Fix:**
```go
buf, err := io.ReadAll(r.Body)
if err != nil {
    cpos_log.Logger.WithError(err).Error("failed to read request body")
    http.Error(w, "failed to read request body", http.StatusBadRequest)
    return
}
```

---

## 🟡 SHOULD FIX — v3 additions (6 issues)

### [v3-14] `inventory/builder.go` — `buildListInventoryItemFilter` mutates caller's `*Request` ⭐ NEW
**File:** `internal/services/inventory/builder.go:29`

```go
func buildListInventoryItemFilter(r *ppb.ListInventoryItemRequest) interface{} {
    r.Limit = constant.LIMIT1000  // ← silently mutates caller's original request!
```

Sau khi gọi builder, `r.Limit` của gRPC handler bị override thành 1000. Response `MaxPage` và `Limit` trả về client tính theo limit sai.

**Fix:** Dùng local variable, không mutate input pointer.

---

### [v3-15] `cmd/grpc.go` — `Hornet` flag registered nhưng không được đọc vào `platform` struct ⭐ NEW
**File:** `cmd/grpc.go:59, 79, 135-143`

```go
// init() — flag registered
grpcCmd.Flags().StringP("hornet", ...)

// runGrpcCmd — BỊ BỎ SÓT
platform := &storeconnect.Platform{
    Hermes:  viper.GetString("hermes"),
    // ...
    // Hornet: viper.GetString("hornet")  ← missing!
}
```

Stores routed qua Hornet sẽ dùng zero-value address → connection refused. Tương tự trong `cmd/sync.go` và `cmd/cron.go`.

---

### [v3-16] `internal/model/store.go` — 200+ production store IDs hardcoded trong source code ⭐ NEW
**File:** `internal/model/store.go:1-282`

Toàn bộ scheduling config của 200+ production stores (ID, cron schedule, store name) được compile vào binary. Thêm/xóa store cần code change + rebuild + redeploy. Store IDs là PII-adjacent.

---

### [v3-17] `repositories/respositories.go` — `UpdateStatus` nhận `string` thay vì `model.JobStatus` ⭐ NEW
**File:** `internal/repositories/respositories.go:12`

```go
// Type system bypass tại mutation boundary quan trọng nhất
UpdateStatus(ctx context.Context, jobID string, status string) error

// Fix: enforce domain type
UpdateStatus(ctx context.Context, jobID string, status model.JobStatus) error
```

---

### [v3-18] `dto/sync_job.go` — `Status string`, `Page/Limit int32` allow negative ⭐ NEW
**File:** `internal/dto/sync_job.go:4-5`

```go
type GetListJobsRequest struct {
    Status  string  // nên là model.JobStatus
    Page    int32   // nên là uint32
    Limit   int32   // nên là uint32
}
```

---

### [v3-19] `product/service.go` — two `grpc.Dial` + `defer conn.Close()` same function scope ⭐ NEW
**File:** `internal/services/product/service.go:172-196`

Hai connections trong cùng 1 function với `defer conn.Close()` theo LIFO. Error-prone — nên dùng explicit `conn.Close()` thay vì `defer` trong conditional blocks.

---

## 🔵 SUGGEST — v3 additions (5 issues)

### [v3-20] `services/job/service.go:NewService` — không có nil guards ⭐ NEW

```go
// Fix: fail fast tại startup thay vì panic muộn tại runtime
func NewService(...) (*Service, error) {
    if syncJobRepo == nil { return nil, errors.New("syncJobRepo must not be nil") }
    if redisConfig == nil { return nil, errors.New("redisConfig must not be nil") }
    if writer == nil { return nil, errors.New("kafka writer must not be nil") }
    return &Service{...}, nil
}
```

---

### [v3-21] `model/sync_job.go` — `Type:""` possible tại construction, không validate ⭐ NEW

`syncJob.Type = ""` trước switch trong `CreateJob`. Nếu switch miss case → DB `not null` constraint fail tại GORM layer (hard error) thay vì clean application error. Thêm validation trước `Create`.

---

### [v3-22] `model/sync_job.go:98` — Redis snapshot emit cả `metadata` và `metadata_json` ⭐ NEW

`json.Marshal(syncJob)` emit 2 fields. Redis round-trip qua `json.Unmarshal` không gọi `AfterFind` → `Metadata` map luôn nil sau khi đọc từ Redis.

---

### [v3-23] `config/redis.go` — không có constructor, `Writer`/`Reader` fields public ⭐ NEW

Nil invariant không được enforce tại construction. Fix: thêm `NewRedisClient(writerAddr, readerAddr string) (*RedisClient, error)`.

---

### [v3-24] `model/sync_job.go:43` — `CanceledAt` là dead schema field ⭐ NEW

`CanceledAt *time.Time` trong schema nhưng `CancelJob` là `panic("implement me")`. Schema và code không đồng bộ — confuse DBA khi xem bảng.

---

## Prioritized Fix Order (Updated)

| Priority | Issues | Lý do |
|----------|--------|-------|
| **P0 — Immediate** | [3] credentials, rotate now | Production passwords in git history |
| **P1 — Crash** | [1] CancelJob panic, [v3-1] grpc panic, [v3-12] zombie REST | Server crash / process death |
| **P2 — Startup broken** | [5] migrate.go init(), [v3-2] grpcCmd missing flags | Services không start được |
| **P3 — Feature broken** | [6] Total=1000, [7] DB dead, [8] stubs | Core Sync Job API vô nghĩa |
| **P4 — Race conditions** | [9] TOCTOU, [10] Kafka/Redis ordering | Data integrity |
| **P5 — Silent data loss** | [NEW-B] Bulk swallowed, [v3-7] Get false success, [v3-8] Delete silently success | Data loss không detect được |
| **P6 — Shutdown** | [12] ctx.Err(), [13] SIGTERM, [v3-3] cron.Stop(), [v3-11] Fetch ctx | Kubernetes graceful shutdown |
| **P7 — Performance** | [NEW-A] per-request ES conn, [v3-6] inverted condition flood logs | Memory leak + log storm |
| **P8 — Data quality** | [v3-5] retries non-Shopify, [v3-10] Redis checkpoint discarded | Partial/stale data silently |

---

## Deploy Order (sau khi fix tất cả P0-P6)

```
1. Rotate credentials bị leak (immediate, trước khi merge)
2. Deploy pbtypes (MR#566 — additive only, không breaking)
3. Run DB migration: ./bin/es-service migrate --mysqlWriteDsn=...
   (sau khi fix [5] migrate.go init() và [v3-4] migration version tracking)
4. Deploy es-service (MR#30 — gRPC + REST + Worker + Cron)
5. Apply k8s-staging (MR#31 — config & env vars, bao gồm MySQL/Kafka/Hornet DSN)
```

---

*Final report — merged from v1 (30 issues) + v2 (18 issues, skill.md checklist) + v3 (24 issues, silent-failure-hunter + type-design-analyzer + feature-dev:code-reviewer). 61 unique issues total.*
