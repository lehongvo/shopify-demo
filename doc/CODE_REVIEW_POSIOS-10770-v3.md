# Code Review — POSIOS-10770 v3 (Plugin Pass)

**Date:** 2026-04-06  
**Tools:** silent-failure-hunter + type-design-analyzer + feature-dev:code-reviewer  
**Scope:** Issues NOT found in v1 or v2 — new findings only

---

## Tổng quan v3

| Loại | Số lượng mới |
|------|-------------|
| 🔴 MUST FIX | 13 |
| 🟡 SHOULD FIX | 6 |
| 🔵 SUGGEST | 5 |
| **Tổng mới** | **24** |

**Nhận xét:** v1+v2 bỏ sót toàn bộ `cmd/cron.go`, `internal/migration/migration.go`, logic `retries` không nhất quán trong worker, và nhiều "false success" return ở tầng service.

---

## 🔴 MUST FIX

### [v3-1] `cmd/grpc.go` — `panic` trong goroutine + không gọi `GracefulStop`
**File:** `cmd/grpc.go:167-179`

```go
go func() {
    lis, err := net.Listen("tcp", address)
    if err != nil {
        panic(err)  // goroutine panic → crash toàn process, không cleanup
    }
    defer lis.Close()
    err = grpcServer.Serve(lis)
    if err != nil {
        panic(err)  // nếu port bị bind sẵn → crash không log
    }
}()
// ...
<-c
// grpcServer.GracefulStop() KHÔNG BAO GIỜ được gọi → in-flight RPCs bị drop
```

`panic` trong goroutine không thể `recover` từ main goroutine → crash toàn process với stack trace không có context. Quan trọng hơn: `GracefulStop()` không được gọi khi nhận signal, mọi in-flight gRPC request bị kill ngay.

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
// ...
<-c
grpcServer.GracefulStop()  // wait for in-flight RPCs to complete
```

---

### [v3-2] `cmd/grpc.go` — `kafkaBrokers`, `mysqlReadDsn`, `mysqlWriteDsn` không được register vào CLI flags
**File:** `cmd/grpc.go:115-127`

```go
// Dùng viper.GetString nhưng không có flags().StringP + viper.BindPFlag tương ứng trong init()
ormReader, err := config.ConnectDatabase(viper.GetString("mysqlReadDsn"))  // luôn ""
ormWriter, err := config.ConnectDatabase(viper.GetString("mysqlWriteDsn")) // luôn ""
kafkaBrokers := strings.Split(viper.GetString("kafkaBrokers"), ",")        // luôn [""]
```

Khác với `cmd/sync.go:103` (đã flag trong v1/v2): đây là `grpcCmd.init()` riêng biệt, thiếu cả 3 binding → `mysqlReadDsn=""` → `ConnectDatabase("")` → error → gRPC server không bao giờ start khi deploy bằng CLI flags.

**Fix:** Thêm vào `grpcCmd.init()`:
```go
grpcCmd.Flags().StringP("mysqlReadDsn", "", "", "MySQL reader DSN")
grpcCmd.Flags().StringP("mysqlWriteDsn", "", "", "MySQL writer DSN")
grpcCmd.Flags().StringP("kafkaBrokers", "", "127.0.0.1:9092", "Kafka brokers (comma-separated)")
_ = viper.BindPFlag("mysqlReadDsn", grpcCmd.Flags().Lookup("mysqlReadDsn"))
_ = viper.BindPFlag("mysqlWriteDsn", grpcCmd.Flags().Lookup("mysqlWriteDsn"))
_ = viper.BindPFlag("kafkaBrokers", grpcCmd.Flags().Lookup("kafkaBrokers"))
```

---

### [v3-3] `cmd/cron.go` — unbuffered signal channel, `cronJob.Stop()` không được gọi
**File:** `cmd/cron.go:78, 137-138`

```go
// line 78 — unbuffered channel: signal bị DROP nếu không có goroutine đang đọc
c := make(chan os.Signal)  // tất cả cmd khác dùng make(chan os.Signal, 1)

// line 137-138 — sau khi nhận signal:
<-c
ctx, cancel := context.WithCancel(ctx)
defer cancel()
// cronJob.Stop() KHÔNG BAO GIỜ được gọi → scheduled goroutines tiếp tục chạy vô tận
```

Go docs cho `signal.Notify` rõ ràng: channel phải có buffer size >= 1. Unbuffered channel + cron jobs không bao giờ Stop → goroutine leak sau mỗi restart.

**Fix:**
```go
c := make(chan os.Signal, 1)
// ...
<-c
cronJob.Stop()  // blocks until running jobs finish
cancel()
```

---

### [v3-4] `internal/migration/migration.go` — không có version tracking, mọi migration re-run mỗi lần
**File:** `internal/migration/migration.go:8-13`

```go
func Migrate(db *gorm.DB) error {
    if err := versions.Version20260121140000(db); err != nil {
        return err
    }
    return nil
}
```

Không có bảng `schema_migrations` hay tracking nào. Mỗi lần `./bin/es-service migrate` chạy, tất cả version functions đều re-execute. `AutoMigrate` hiện tại chỉ additive (GORM), nhưng bất kỳ future migration nào có raw SQL destructive (rename column, drop index) sẽ chạy lại mỗi lần → data corruption.

**Fix:** Dùng `pressly/goose` hoặc `golang-migrate/migrate`:
```go
import "github.com/pressly/goose/v3"

func Migrate(db *gorm.DB) error {
    sqlDB, _ := db.DB()
    return goose.Up(sqlDB, "./migrations")
}
```

---

### [v3-5] `internal/worker/sync.go` — `retries` chỉ reset cho Shopify stores, accumulate cross-page với non-Shopify
**File:** `internal/worker/sync.go:162-164`

```go
// SyncProduct — reset retries chỉ khi là Shopify
if storeconnect.IsShopify(store.GetType()) {
    retries = 0
}

// SyncInventory — reset unconditional (đúng)
retries = 0
```

Với non-Shopify stores (BigCommerce, Magento): page 3 retry 4 lần → page 4 chỉ còn 6 retries → 10 total retry increments → loop exit sớm kể cả khi từng error là transient. Data sync bị cắt giữa chừng mà không có error rõ ràng.

**Fix:**
```go
retries = 0  // unconditional — nhất quán với SyncInventory
```

---

### [v3-6] `inventory/processor.go` — điều kiện inverted logs EVERY item thay vì chỉ items có BinNumbers
**File:** `internal/services/inventory/processor.go:120`

```go
// HIỆN TẠI — true với hầu hết items (non-nil OR empty)
if item.BinNumbers != nil || len(item.BinNumbers) == 0 {
    logEntry.Info("fetch bin numbers success")
}
// Mục đích: chỉ log khi có bin numbers
// Nhưng: (non-nil slice) || (empty slice) = true với slice rỗng VÀ slice có data
```

Log `"fetch bin numbers success"` cho mỗi item trong bulk fetch → N×1000 log lines mỗi sync run. Severe performance degradation do log I/O.

**Fix:**
```go
if len(item.BinNumbers) > 0 {
    logEntry.Info("fetch bin numbers success")
}
```

---

### [v3-7] `product/service.go` — `Get()` trả `empty Product + nil error` khi ES fail — false success
**File:** `internal/services/product/service.go:47-58`

```go
result, err := esConnection.Product.Get(r.GetId())
if nil != err {
    cpos_log.Logger.WithFields(...).WithError(err).Error("search product failed")
    return &prodpb.Product{}, nil  // ← ES down → caller nhận empty product + HTTP 200
}
```

Caller không thể phân biệt "product not found" với "ES infrastructure down". POS client nhận HTTP 200 với empty product.

**Fix:**
```go
if err != nil {
    cpos_log.Logger.WithFields(...).WithError(err).Error("search product failed")
    return nil, status.Errorf(codes.Internal, "failed to get product %s: %v", r.GetId(), err)
}
```

---

### [v3-8] `product/service.go` — Delete loop: partial failures silently succeed
**File:** `internal/services/product/service.go:153-165`

```go
for _, ID := range IDs {
    err := esConnection.Product.Delete(ID)
    if nil == err {
        deleteProductsWorker(esConnection, ID)
        continue
    }
    logEntry.WithError(err).Error("delete document failed")
    // ← không có accumulation, không return error
}
return &ppb.SyncProductResponse{}, nil  // luôn success dù tất cả Delete đều fail
```

Nếu ES down, tất cả deletes fail → caller nhận success → products đã xóa trên Shopify vẫn tồn tại trong ES index vĩnh viễn.

**Fix:**
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

### [v3-9] `product/service.go` — gRPC Dial failure trong SalesChannel mapping trả false success
**File:** `internal/services/product/service.go:175-183`

```go
conn, err := grpc.Dial(...)
if nil != err {
    logEntry.WithError(err).Error("init connection failed")
    return &ppb.SyncProductResponse{}, nil  // ← platform unreachable → success!
}
```

Platform service down → products sync to ES without channel mapping → POS serves incorrectly mapped products. Caller không biết mapping đã fail.

**Fix:**
```go
if err != nil {
    return nil, status.Errorf(codes.Unavailable, "platform service unavailable: %w", err)
}
```

---

### [v3-10] `inventory/processor.go` — Redis checkpoint read/write errors discarded → silent full re-sync
**File:** `internal/services/inventory/processor.go:78, 161, 236`

```go
// line 78: error từ Redis.Get bị discard hoàn toàn
updatedAtMin, _ := p.redisClient.Reader.Get(context.Background(), ...).Result()

// line 161: return value của Set bị discard
p.redisClient.Writer.Set(context.Background(), ..., lastFetchTime, redis.KeepTTL)
```

Redis down → `updatedAtMin = ""` → incremental sync trở thành full re-index. Checkpoint không được lưu → mỗi run sau đó cũng full re-index. Không có log nào.

**Fix:**
```go
updatedAtMin, err := p.redisClient.Reader.Get(ctx, key).Result()
if err != nil && !errors.Is(err, redis.Nil) {
    logEntry.WithError(err).Error("failed to read last fetch timestamp — performing full sync")
}

if err := p.redisClient.Writer.Set(ctx, key, lastFetchTime, redis.KeepTTL).Err(); err != nil {
    logEntry.WithError(err).Error("failed to persist sync checkpoint — next run will full re-index")
}
```

---

### [v3-11] `inventory/processor.go` — `Fetch()` không có context parameter, không thể cancel
**File:** `internal/services/inventory/processor.go:57`

```go
func (p *Processor) Fetch() {  // không có ctx
    ...
    err := p.fetchInventoryInNetSuite(context.Background())  // không thể cancel
    result, err := platformService.GetList(context.Background(), filter)  // không thể cancel
```

`Fetch()` không thể bị interrupt bằng SIGTERM hay context cancellation. Mỗi inventory fetch cho large stores có thể chạy hàng giờ. Graceful shutdown không hoạt động.

**Fix:**
```go
func (p *Processor) Fetch(ctx context.Context) {
    ...
    err := p.fetchInventoryInNetSuite(ctx)
    result, err := platformService.GetList(ctx, filter)
```

---

### [v3-12] `cmd/rest.go` — HTTP `ListenAndServe` error bị swallow, process zombie
**File:** `cmd/rest.go:78-84`

```go
go func() {
    fmt.Println("server started")  // log trước khi thực sự serve
    err := srv.ListenAndServe()
    if nil != err {
        fmt.Println(err)  // stdout only — không vào log aggregation (ELK, Datadog)
        // process tiếp tục, không exit → zombie: alive nhưng không serve gì
    }
}()
```

Port đã bị bind → `ListenAndServe` error → `fmt.Println` → process vẫn chạy → health check pass → load balancer gửi traffic vào zombie process.

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

### [v3-13] `cmd/rest.go:107,122` — `ioutil.ReadAll` errors discarded → request body corruption
**File:** `cmd/rest.go:107-108, 122`

```go
buf, _ := ioutil.ReadAll(r.Body)   // error discarded → buf partial/empty
respBody, _ := ioutil.ReadAll(...)  // error discarded → empty response logged
```

Khác với v1/v2 chỉ flag `ioutil` deprecated: đây là bug nghiêm trọng hơn — error bị discard (`_`). Client disconnect mid-request → `buf` chứa truncated body → JSON unmarshal nhận payload sai → xử lý request với data corrupt, không có lỗi nào được raise.

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

## 🟡 SHOULD FIX

### [v3-14] `inventory/builder.go` — `buildListInventoryItemFilter` mutates caller's `*Request`
**File:** `internal/services/inventory/builder.go:29`

```go
func buildListInventoryItemFilter(r *ppb.ListInventoryItemRequest) interface{} {
    r.Limit = constant.LIMIT1000  // ← silently mutates caller's request pointer!
```

Caller ở `listInventoryWorker` truyền `r` từ gRPC handler. Sau khi gọi builder, `r.Limit` bị override thành 1000. Response `MaxPage` và `Limit` trả về client sẽ tính theo limit sai (1000 thay vì client-requested value).

**Fix:** Dùng local variable, không mutate input:
```go
func buildListInventoryItemFilter(r *ppb.ListInventoryItemRequest) interface{} {
    limit := int32(constant.LIMIT1000)
    from := (r.GetPage() - 1) * limit
    // dùng `limit` và `from` locally, không ghi vào r
```

---

### [v3-15] `cmd/grpc.go` — `Hornet` flag được register nhưng không được đọc vào `platform` struct
**File:** `cmd/grpc.go:59, 79, 135-143`

```go
// init() — flag được register
grpcCmd.Flags().StringP("hornet", ...)
_ = viper.BindPFlag("hornet", ...)

// runGrpcCmd — KHÔNG có trong platform struct
platform := &storeconnect.Platform{
    Hermes:  viper.GetString("hermes"),
    // ...
    Firefly: viper.GetString("firefly"),
    // Hornet: viper.GetString("hornet") ← bị bỏ sót
}
```

Bất kỳ store nào route qua Hornet sẽ dùng zero-value address → connection refused. Tương tự bị bỏ sót ở `cmd/sync.go` và `cmd/cron.go`.

---

### [v3-16] `internal/model/store.go` — 200+ production store IDs hardcoded trong source code
**File:** `internal/model/store.go:1-282`

Toàn bộ cấu hình scheduling của 200+ production stores (ID, cron schedule, store name) được compile vào binary. Thêm/xóa store cần code change + rebuild + redeploy. Store IDs là PII-adjacent. Nên drive bằng database hoặc config file.

---

### [v3-17] `repositories/respositories.go` — `UpdateStatus` nhận `string` thay vì `model.JobStatus`
**File:** `internal/repositories/respositories.go:12`

```go
// Interface: bypass type system ở mutation boundary quan trọng nhất
UpdateStatus(ctx context.Context, jobID string, status string) error

// Fix
UpdateStatus(ctx context.Context, jobID string, status model.JobStatus) error
```

---

### [v3-18] `dto/sync_job.go` — `Status string` thay vì `model.JobStatus`, `Page/Limit int32` allow negative
**File:** `internal/dto/sync_job.go:4-5`

```go
// Hiện tại
type GetListJobsRequest struct {
    Status  string  // nên là model.JobStatus
    Page    int32   // nên là uint32
    Limit   int32   // nên là uint32
}
```

---

### [v3-19] `product/service.go:172-196` — two `grpc.Dial` + `defer conn.Close()` same scope, fragile
**File:** `internal/services/product/service.go:172-196`

Hai connections được open trong cùng 1 function với `defer conn.Close()` — LIFO defer order. Nếu cả hai conditions đều true, defers close theo thứ tự ngược, và `platformService` (giữ reference đến conn) vẫn được dùng bên trong loop. Nên explicit `conn.Close()` thay vì `defer` trong conditional blocks.

---

## 🔵 SUGGEST

### [v3-20] `services/job/service.go:NewService` — không có nil guards
**File:** `internal/services/job/service.go:31-41`

```go
// Fix: return error thay vì panic muộn
func NewService(...) (*Service, error) {
    if syncJobRepo == nil { return nil, errors.New("syncJobRepo must not be nil") }
    if redisConfig == nil { return nil, errors.New("redisConfig must not be nil") }
    if writer == nil { return nil, errors.New("kafka writer must not be nil") }
    return &Service{...}, nil
}
```

---

### [v3-21] `model/sync_job.go` — `Type: ""` possible tại construction, BeforeCreate không validate
**File:** `internal/services/job/service.go:55`, `internal/model/sync_job.go:BeforeCreate`

`syncJob.Type = ""` trước switch. Nếu switch miss case → DB `not null` constraint fail tại layer GORM (hard error) thay vì application-level error với message rõ ràng.

---

### [v3-22] `model/sync_job.go:98` — Redis snapshot chứa cả `metadata` và `metadata_json`
**File:** `internal/services/job/service.go:98`

`json.Marshal(syncJob)` emit cả 2 fields (`"metadata":{}` và `"metadata_json":"{}"`). Sau Redis round-trip, `json.Unmarshal` vào `syncJob` không gọi `AfterFind` → `Metadata` map luôn nil khi đọc từ Redis.

---

### [v3-23] `config/redis.go` — không có constructor, fields public — nil invariant không được enforce
**File:** `internal/config/redis.go`

```go
// Fix:
func NewRedisClient(writerAddr, readerAddr string, db int) (*RedisClient, error) {
    if writerAddr == "" { return nil, errors.New("redis writer address required") }
    // ...
}
```

---

### [v3-24] `model/sync_job.go:43` — `CanceledAt *time.Time` là dead schema field
**File:** `internal/model/sync_job.go:43`

`CanceledAt` có trong schema nhưng `CancelJob` là `panic("implement me")`. Schema và code không đồng bộ — có thể gây confusion khi DBA xem bảng.

---

## Summary — Top new issues cần fix

| Priority | Issue | File |
|----------|-------|------|
| 🔴 P1 | [v3-1] grpcServer panic + no GracefulStop | `cmd/grpc.go:167` |
| 🔴 P2 | [v3-2] MySQL/Kafka flags không register → gRPC không start | `cmd/grpc.go:115-127` |
| 🔴 P3 | [v3-3] cron.go: unbuffered signal + cronJob.Stop() missing | `cmd/cron.go:78,137` |
| 🔴 P4 | [v3-6] Inverted condition logs mọi inventory item | `processor.go:120` |
| 🔴 P5 | [v3-7,8,9] Product service false-success masking | `product/service.go` |
| 🔴 P6 | [v3-10] Redis checkpoint errors discarded → full re-sync loop | `processor.go:78,161` |
| 🔴 P7 | [v3-5] retries không reset cho non-Shopify stores | `worker/sync.go:162` |

---

*v3 generated by: silent-failure-hunter + type-design-analyzer + feature-dev:code-reviewer on 2026-04-06*
