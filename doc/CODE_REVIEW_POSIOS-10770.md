# Code Review — Branch POSIOS-10770 vs main

**Date:** 2026-04-06  
**Branch:** `POSIOS-10770` vs `main`  
**Repo:** `elastic-search-service` (98 files changed, 5411 insertions, 4522 deletions)  
**Build:** `go build ./...` ✅ PASS  
**Verified:** All issues confirmed by reading actual source files

---

## Tổng quan

| Loại | Số lượng |
|------|----------|
| 🔴 MUST FIX | 14 |
| 🟡 SHOULD FIX | 10 |
| 🔵 SUGGEST | 6 |
| **Tổng** | **30** |

**Kiến trúc:** Refactor flat packages → `internal/` layering là đúng hướng. Tuy nhiên, tính năng core (Sync Job API) có nhiều lỗi nghiêm trọng, không thể merge vào production trong trạng thái hiện tại.

**Kết luận: BLOCKED — Cần fix trước khi merge.**

---

## 🔴 MUST FIX

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
func (s Service) CancelJob(ctx context.Context, request *jobPb.GetJobRequest) (*jobPb.Job, error) {
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

In-flight HTTP requests bị kill ngay khi nhận SIGTERM. Goroutine chạy `ListenAndServe` (line 78-84) cũng không được notify.

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

### [3] gRPC-gateway registration chạy async — requests 404 khi server mới start
**File:** `cmd/rest.go:55-68`

```go
// HIỆN TẠI — gwmux chưa được wired khi srv.ListenAndServe() bắt đầu nhận requests
go func() {
    err := jobPb.RegisterJobServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts)
    if nil != err {
        fmt.Println(err)
        os.Exit(1)  // os.Exit từ goroutine — không thể recover
    }
}()
mux.Handle("/", gwmux)  // line 68 — gwmux chưa sẵn sàng
```

Server bắt đầu nhận request trước khi handler được register. Early requests sẽ nhận 404. `os.Exit(1)` trong goroutine cũng không cho phép cleanup.

**Fix:** Register synchronous trước khi serve:
```go
if err := jobPb.RegisterJobServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts); err != nil {
    return fmt.Errorf("failed to register gateway: %w", err)
}
mux.Handle("/", gwmux)
```

---

### [4] `cmd/migrate.go` thiếu `init()` — command không được đăng ký
**File:** `cmd/migrate.go`

```go
// HIỆN TẠI — migrateCmd được khai báo nhưng KHÔNG có init()
var migrateCmd = &cobra.Command{
    Use:   "migrate",
    Short: "Run the migration",
    Run: func(cmd *cobra.Command, args []string) { ... },
}
// KHÔNG CÓ: func init() { rootCmd.AddCommand(migrateCmd) }
```

`./bin/es-service migrate` sẽ báo "unknown command". Migration không chạy được.

**Fix:** Thêm vào cuối file:
```go
func init() {
    rootCmd.AddCommand(migrateCmd)
}
```

---

### [5] `Total` hardcoded 1000, progress tracking bị comment out hoàn toàn
**Files:** `internal/services/job/service.go:57` + `internal/worker/sync.go:99-103, 193-197`

```go
// service.go:57 — hardcoded, không bao giờ được update thực tế
Total: 1000,

// sync.go:99-103 — comment out hoàn toàn
//var (
//    keyJobTotal   = fmt.Sprintf(constant.KeyStoreSyncTotal, storeId, jobId)
//    keyJobSuccess = fmt.Sprintf(constant.KeyStoreSyncSuccess, storeId, jobId)
//    keyJobFailed  = fmt.Sprintf(constant.KeyStoreSyncFailed, storeId, jobId)
//)

// sync.go:193-197 — cũng comment out
//_, err = sw.redisConfig.Writer.IncrBy(ctx, keyJobSuccess, int64(len(result.GetItems()))).Result()
```

Client polling `GetJob` luôn nhận `Total=1000, Succeeded=0, Failed=0` bất kể sync đã xong hay chưa. Sync Job API hoàn toàn vô nghĩa.

---

### [6] `syncJobRepo.Create()` không bao giờ được gọi — DB persistence dead
**File:** `internal/services/job/service.go:43-142`

`Service.CreateJob` nhận `syncJobRepo` làm dependency nhưng không gọi `syncJobRepo.Create(ctx, syncJob)` ở bất kỳ đâu. Migration tạo bảng `sync_jobs` (version `20260121140000`) nhưng bảng luôn rỗng. Sau `ExpireTime = 8 * time.Hour`, job biến mất vĩnh viễn.

---

### [7] Repository methods là stubs — silently no-op
**File:** `internal/repositories/sync_job/repository.go:26-34`

```go
// HIỆN TẠI — 3/4 methods không làm gì
func (repo *SyncJobRepository) UpdateStatus(ctx context.Context, jobID string, status string) error {
    return nil  // silent no-op
}
func (repo *SyncJobRepository) GetByID(ctx context.Context, jobID string) (*model.SyncJob, error) {
    return nil, nil  // always "not found"
}
func (repo *SyncJobRepository) Find(ctx context.Context, req *dto.GetListJobsRequest) ([]*model.SyncJob, error) {
    return nil, nil  // always empty
}
```

Caller nhận `nil, nil` nhưng nghĩ rằng operation thành công.

---

### [8] TOCTOU race — double-sync cho cùng 1 store
**File:** `internal/services/job/service.go:81-90`

```go
// HIỆN TẠI — không atomic
exists, err := s.redisConfig.Writer.Exists(ctx, keyJobSyncing).Result()
// ← 2 concurrent requests đều thấy exists=0
if exists == 1 {
    return &jobPb.Job{}, errors.New("job syncing already exists")
}
// cả 2 đều tiếp tục → 2 Kafka events được publish → double sync
pipeline.Set(ctx, keyJobSyncing, jobUUID, constant.ExpireTime)
```

**Fix:** Thay `Exists` + `Set` bằng `SetNX` (atomic):
```go
set, err := s.redisConfig.Writer.SetNX(ctx, keyJobSyncing, jobUUID, constant.ExpireTime).Result()
if err != nil { return nil, err }
if !set {
    return nil, errors.New("job syncing already exists")
}
```

---

### [9] Redis set TRƯỚC Kafka publish — lock bị orphan nếu Kafka fail
**File:** `internal/services/job/service.go:104-136`

```go
// Step 1: Set Redis keys (line 104-113)
pipeline.Set(ctx, keyJobSyncing, jobUUID, constant.ExpireTime)  // lock acquired
_, err = pipeline.Exec(ctx)  // ← committed

// Step 2: Publish Kafka (line 128-135) — có thể fail SAU KHI Redis đã set
err = s.writer.WriteMessages(ctx, kafka.Message{...})
if err != nil {
    return nil, err  // ← return error nhưng Redis keyJobSyncing vẫn tồn tại!
}
```

Nếu Kafka fail: Redis có `keyJobSyncing` nhưng không có worker nào pick up event → store bị lock cho đến hết TTL (8 giờ), user không thể trigger sync.

**Fix:** Publish Kafka trước, set Redis sau. Hoặc rollback Redis keys nếu Kafka fail.

---

### [10] `defer` xóa syncing key kể cả khi sync fail — job status không nhất quán
**File:** `internal/worker/sync.go:91-98` (SyncProduct) và `sync.go:233-240` (SyncInventory)

```go
defer func() {
    isSyncingKey := fmt.Sprintf(constant.KeyStoreSyncJob, storeId)
    _, err := sw.redisConfig.Writer.Del(ctx, isSyncingKey).Result()
    // ← luôn xóa, dù sync thành công hay thất bại
}()
```

Khi rate-limit hit sau 10 retries → function return → defer xóa key. Redis nói "not syncing" nhưng data trong ES có thể không đầy đủ, và không có ghi nhận lỗi nào.

---

### [11] `Handler` loop spin CPU khi ctx bị cancel
**File:** `internal/worker/sync.go:59-83`

```go
for {
    msg, err := sw.reader.ReadMessage(ctx)
    if err != nil {
        logger.WithField("msg", msg).WithError(err).Error("failed to read message from broker")
        continue  // ← SIGTERM → ctx cancelled → ReadMessage returns err → loop vô tận
    }
    ...
}
```

Khi nhận SIGTERM, `ReadMessage(ctx)` trả về error do context cancelled, loop tiếp tục spin và log lỗi liên tục.

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

### [12] `pkg/context/signal.go` chỉ catch `os.Interrupt`, thiếu `SIGTERM`
**File:** `pkg/context/signal.go:18`

```go
// HIỆN TẠI — chỉ SIGINT/Ctrl+C
signal.Notify(c, os.Interrupt)
```

Kubernetes gửi **SIGTERM** khi xóa pod (`kubectl delete pod`). `WithInterruptSignal` không cancel context → `syncWorker.Handler(ctx)` không dừng gracefully → pod bị force-killed sau `terminationGracePeriodSeconds`.

**Fix:**
```go
import "syscall"
signal.Notify(c, os.Interrupt, syscall.SIGTERM)
```

---

### [13] `service_test.go` không compile — sai package
**File:** `internal/services/inventory/service_test.go:14`

```go
// HIỆN TẠI — model.ESConfig không tồn tại trong package internal/model
import "git02.smartosc.com/.../internal/model"

service := NewService(&model.ESConfig{
    Addresses: []string{"http://localhost:9200"},
    Username:  "elastic",
    Password:  "da9df3f&4a36fesgb3",  // ← hardcoded credentials
})
```

`ESConfig` nằm ở `internal/config`, không phải `internal/model`. Test này không compile được. Ngoài ra có credential hardcoded trong code.

**Fix:**
```go
import "git02.smartosc.com/.../internal/config"

service := NewService(&config.ESConfig{
    Addresses: []string{"http://localhost:9200"},
})
```

---

### [14] DB pool misconfigured — `MaxIdleConns > MaxOpenConns`
**File:** `internal/config/database.go:33-34`

```go
db.SetMaxIdleConns(15)  // 15 idle connections
db.SetMaxOpenConns(10)  // nhưng max open chỉ có 10 → impossible state
```

`database/sql` sẽ silently cap idle xuống 10. Cấu hình sai, misleading khi debug.

**Fix:**
```go
db.SetMaxOpenConns(20)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(300 * time.Minute)
```

---

## 🟡 SHOULD FIX

### [15] Goroutine fan-out không giới hạn — no backpressure
**File:** `internal/worker/sync.go:79, 81`

```go
case string(model.JobTypeSyncProduct):
    go sw.SyncProduct(ctx, event.StoreId, event.JobId)
case string(model.JobTypeSyncInventory):
    go sw.SyncInventory(ctx, event.StoreId, event.JobId)
```

Mỗi Kafka message tạo 1 goroutine mới, mỗi goroutine giữ 1 gRPC connection và có thể pull hàng trăm nghìn items. Không có semaphore giới hạn số goroutine concurrent.

---

### [16] `context.Background()` trong business logic — không propagate cancellation
**File:** `internal/worker/sync.go:145` (SyncProduct), `sync.go:290` (SyncInventory)

```go
result, err := platformService.GetList(context.Background(), filter)  // ← sai
```

Nên dùng `ctx` được truyền từ caller để SIGTERM có thể cancel in-flight gRPC calls.

---

### [17] `logger.Logger.WithFields(...)` drops context fields
**File:** `internal/worker/sync.go:314, 326`

```go
// HIỆN TẠI — .Logger truy cập raw *logrus.Logger, drop hết context fields
logger.Logger.WithFields(logrus.Fields{
    "filter":        filter,
    "total_changed": len(result.GetItems()),
}).Info("fetch inventory data")
```

`logger` là `*logrus.Entry` với fields `store`, `processor`, `method`. `.Logger` lấy raw logger → mất hết context fields khi log.

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
    logger.Info("upsert inventory worker success")  // ← log "success" kể cả khi có error!
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
if err != nil { return nil, err }  // err có thể là redis.Nil
```

Nếu TTL của counter keys và info key khác nhau → client nhận 500 thay vì empty Job.

**Fix:** Xử lý `redis.Nil` cho tất cả counter keys (treat as 0).

---

### [21] `internal/broker/kafka.go` — `consumerAdmin` không check error → panic
**File:** `internal/broker/kafka.go:60-61`

```go
consumerAdmin, _ := kafka.NewAdminClientFromConsumer(c)  // error ignored
_, err = consumerAdmin.CreateTopics(...)  // nếu consumerAdmin == nil → PANIC
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
// cmd/sync.go:108 — copy-paste từ service khác
GroupID: "back-in-stock-worker",

// broker/kafka.go:53 — placeholder chưa đổi
"group.id": "myGroup",
```

Nếu share Kafka cluster với service dùng cùng group ID → 2 services chia partition, cả 2 drop ~50% messages.

**Fix:** Dùng `"es-sync-worker"` hoặc đọc từ config.

---

### [23] `kafkaBrokers` flag chưa được đăng ký trong `cmd/sync.go`
**File:** `cmd/sync.go:103, init()`

```go
// runSyncWorkerCmd sử dụng:
kafkaBrokers := strings.Split(viper.GetString("kafkaBrokers"), ",")

// Nhưng trong init() KHÔNG có:
// syncWorkerCmd.Flags().StringP("kafkaBrokers", ...)
// viper.BindPFlag("kafkaBrokers", ...)
```

`kafkaBrokers` chỉ có thể set qua env var hoặc config file, không thể dùng CLI flag. Tương tự `system-watermill-suffix` ở line 107.

---

### [24] `cmd/grpc.go` — MySQL là hard dependency của toàn bộ gRPC server
**File:** `cmd/grpc.go:115-125`

```go
ormReader, err := config.ConnectDatabase(viper.GetString("mysqlReadDsn"))
if err != nil {
    logrus.WithError(err).Error("Failed to connect to MySQL Reader")
    return  // ← category, product, inventory, customer cũng chết theo
}
```

Trước đây gRPC server chỉ cần ES + platform endpoints. Giờ MySQL down → toàn bộ service không start được, kể cả các domain không liên quan đến JobService.

---

## 🔵 SUGGEST

### [25] `ioutil.ReadAll` deprecated — Go 1.16+
**File:** `cmd/rest.go:107-108`

```go
buf, _ := ioutil.ReadAll(r.Body)      // deprecated + error discarded
reader := ioutil.NopCloser(...)
```

**Fix:** `io.ReadAll(r.Body)`, `io.NopCloser(...)`

---

### [26] `grpc.WithInsecure()` deprecated
**Files:** `cmd/rest.go:59`, `internal/worker/sync.go:122, 267, 363`

```go
grpc.WithInsecure()  // deprecated since gRPC-Go v1.27
```

**Fix:** `grpc.WithTransportCredentials(insecure.NewCredentials())`

---

### [27] `model.JobStatusRunning = "processing"` — tên và value mâu thuẫn
**File:** `internal/model/sync_job.go:27`

```go
JobStatusRunning JobStatus = "processing"  // "Running" vs "processing"
```

Chọn một vocabulary: đổi constant name thành `JobStatusProcessing` hoặc đổi value thành `"running"`.

---

### [28] `UpsertInventoryWorker` — logEntry stacking trong loop
**File:** `internal/services/inventory/worker.go:35`

```go
for {
    logEntry = logEntry.WithField("filter", filter)  // ← sau N vòng = N "filter" fields
    ...
}
```

**Fix:** Move ra ngoài loop hoặc tạo fresh entry mỗi vòng.

---

### [29] Duplicate constants — `constant.JobSync*` và `model.JobType*`
**Files:** `internal/constant/job.go` và `internal/model/sync_job.go`

Hai file có cùng string values:
```go
// constant/job.go
JobSyncProduct = "sync_product"

// model/sync_job.go
JobTypeSyncProduct JobType = "sync_product"
```

Nên dùng `model.JobType*` làm source of truth, xóa `constant.JobSync*`.

---

### [30] `InitLogger` — options bị ignore
**File:** `pkg/cpos_log/log.go:64-75`

```go
func InitLogger(options ...Option) {
    loggerOnce.Do(func() {
        logrusConf := DefaultLogrusConf()
        o := option{OverrideLogrusConf: logrusConf}
        for _, option := range options {
            option(&o)  // modify o
        }
        Logger = NewLogger(logrusConf)  // ← dùng logrusConf gốc, KHÔNG phải o.OverrideLogrusConf
    })
}
```

Custom formatter/hooks truyền vào bị silently ignored.

**Fix:** `Logger = NewLogger(o.OverrideLogrusConf)`

---

## Summary — Top 5 phải fix trước merge

| Priority | Issue | File |
|----------|-------|------|
| 🔴 P1 | [5,6,7] Job API broken: Total=1000, DB không được ghi, repo stubs | `service.go`, `sync.go`, `repository.go` |
| 🔴 P2 | [8,9] Race conditions: TOCTOU + Kafka/Redis ordering | `service.go` |
| 🔴 P3 | [1] CancelJob panic | `service.go:230` |
| 🔴 P4 | [4] migrate command không chạy được | `migrate.go` |
| 🔴 P5 | [11,12] Worker không shutdown gracefully (SIGTERM) | `sync.go`, `signal.go` |

---

## Deploy Order (sau khi fix)

```
1. Run DB migration:  ./bin/es-service migrate --mysqlWriteDsn=...
                      (sau khi fix issue #4)
2. Deploy pbtypes     (MR#566 — job package mới, không breaking)
3. Deploy es-service  (MR#30 — gRPC + REST + Worker)
4. Apply k8s-staging  (MR#31 — config & env vars)
```

---

*Report generated by code review on 2026-04-06. All issues verified by reading actual source files.*
