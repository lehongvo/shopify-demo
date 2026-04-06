# Code Review Report — POSIOS-10770 (v2)

**Date:** 2026-04-06  
**Branch:** `POSIOS-10770 vs main`  
**Reviewer:** Claude (Go Expert — theo skill.md checklist)  
**Files reviewed:** `internal/services/job/service.go`, `internal/worker/sync.go`, `internal/repositories/sync_job/repository.go`, `internal/services/product/service.go`, `internal/services/inventory/service.go`, `internal/services/inventory/worker.go`, `internal/config/elastic_search.go`, `internal/config/database.go`, `cmd/rest.go`, `cmd/sync.go`, `cmd/grpc.go`, `cmd/migrate.go`, `pkg/context/signal.go`, `pkg/cpos_log/log.go`, `internal/broker/kafka.go`, `internal/model/sync_job.go`

---

## Tổng quan

| Loại | Số lượng |
|------|----------|
| 🔴 MUST FIX | 7 |
| 🟡 SHOULD FIX | 7 |
| 🔵 SUGGEST | 4 |
| **Tổng** | **18** |

**Đánh giá chung:** Branch refactor cấu trúc đúng hướng, nhưng feature sync job bị broken hoàn toàn (progress không track, DB không persist), có security violation nghiêm trọng (credentials hardcoded), và nhiều lỗi idiomatic Go theo checklist.

---

## 🔴 MUST FIX

### [1] `CancelJob` panic — crash gRPC server khi bị gọi

**File:** `internal/services/job/service.go:230-233`

**Vấn đề:** Method đã được register vào gRPC server (`cmd/grpc.go:165`). Bất kỳ client nào gọi = crash toàn process.

```go
// Code hiện tại
func (s Service) CancelJob(ctx context.Context, request *jobPb.GetJobRequest) (*jobPb.Job, error) {
    //TODO implement me
    panic("implement me")
}
```

```go
// Đề xuất
func (s *Service) CancelJob(ctx context.Context, request *jobPb.GetJobRequest) (*jobPb.Job, error) {
    return nil, status.Error(codes.Unimplemented, "CancelJob not yet implemented")
}
```

---

### [2] Hardcoded production credentials trong test files — security violation

**File:** `internal/services/inventory/service_test.go:14-18`  
**File:** `internal/services/product/service_test.go:14-19`

**Vấn đề:** Password và URL production thật được commit vào git history — tồn tại mãi mãi kể cả sau khi xóa file.

```go
// inventory/service_test.go — SAI
service := NewService(&model.ESConfig{           // model.ESConfig không tồn tại → compile error
    Addresses: []string{"http://localhost:9200"},
    Password:  "da9df3f&4a36fesgb3",             // hardcoded password
})

// product/service_test.go — SAI
service := NewService(&model.ESConfig{           // model.ESConfig không tồn tại → compile error
    Addresses: []string{"https://acd.posify.io"}, // production URL
    Password:  "82ARtK-Dq1EUbT1XiOtW",            // hardcoded production password
}, nil)
```

**Đề xuất:**
1. Xóa cả 2 file này khỏi branch (và rewrite git history nếu cần)
2. Nếu giữ, phải dùng `//go:build integration`, đọc từ env var, không hardcode:

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

### [3] `context.Background()` trong business logic — không propagate cancellation

**File:** `internal/worker/sync.go:145` (SyncProduct), `sync.go:290` (SyncInventory)

**Vấn đề:** Vi phạm rule "không dùng `context.Background()` trong business logic". Khi SIGTERM, các gRPC call này không bị cancel → worker không graceful shutdown.

```go
// Code hiện tại — SAI
result, err := platformService.GetList(context.Background(), filter)
```

```go
// Đề xuất — dùng ctx đã có trong scope
result, err := platformService.GetList(ctx, filter)
```

---

### [4] Handler loop không check `ctx.Err()` — spin CPU vô tận khi shutdown

**File:** `internal/worker/sync.go:59-83`

**Vấn đề:** Vi phạm rule "goroutine phải có exit condition rõ ràng". Khi context bị cancel (SIGTERM), `ReadMessage(ctx)` trả về error nhưng loop `continue` → log spam + CPU burn + goroutine never exits.

```go
// Code hiện tại — SAI
for {
    msg, err := sw.reader.ReadMessage(ctx)
    if err != nil {
        logger.WithField("msg", msg).WithError(err).Error("...")
        continue  // ← không bao giờ thoát khi ctx cancelled
    }
}
```

```go
// Đề xuất
for {
    msg, err := sw.reader.ReadMessage(ctx)
    if err != nil {
        if ctx.Err() != nil {
            logger.Info("context cancelled, stopping handler")
            return
        }
        logger.WithError(err).Error("failed to read message")
        continue
    }
}
```

---

### [5] `pkg/context/signal.go` chỉ catch `os.Interrupt`, thiếu `SIGTERM`

**File:** `pkg/context/signal.go:18`

**Vấn đề:** Kubernetes pod termination gửi `SIGTERM`, không phải `SIGINT`. `WithInterruptSignal` không cancel context → `syncWorker.Handler(ctx)` không dừng gracefully.

```go
// Code hiện tại — SAI
signal.Notify(c, os.Interrupt)
```

```go
// Đề xuất
signal.Notify(c, os.Interrupt, syscall.SIGTERM)
```

---

### [6] `NewESConnection` tạo mới ES client per-request — performance bottleneck nghiêm trọng

**File:** `internal/config/elastic_search.go:19`, gọi từ `internal/services/product/service.go:43,88,151`, `internal/services/inventory/service.go:24`, `internal/worker/sync.go:114,251`

**Vấn đề:** Vi phạm principle "không allocate trong hot path". Mỗi gRPC request tạo mới `elasticsearch.Config` và `connection.Connection` (bao gồm HTTP transport pool). Đây là bottleneck nghiêm trọng với high-traffic service.

```go
// Product.GetList() — tạo mới mỗi request
func (s *Service) GetList(ctx context.Context, r *ppb.ListProductRequest) ... {
    esConnection := config.NewESConnection(s.esConfig, merchantID)  // ← per request!
```

**Đề xuất:** Cache `*connection.Connection` theo `merchantID` bằng `sync.Map`:
```go
type Service struct {
    esConfig    *config.ESConfig
    platform    *storeconnect.Platform
    connCache   sync.Map // key: merchantID, value: *connection.Connection
}

func (s *Service) getConn(merchantID string) *connection.Connection {
    if v, ok := s.connCache.Load(merchantID); ok {
        return v.(*connection.Connection)
    }
    conn := config.NewESConnection(s.esConfig, merchantID)
    s.connCache.Store(merchantID, conn)
    return conn
}
```

---

### [7] `product.Service.Sync` — silently ignores Bulk error, logs "success" khi thất bại

**File:** `internal/services/product/service.go:227-233`

**Vấn đề:** Vi phạm rule "không swallow error". Nếu `Bulk` fail, function vẫn log "sync product success" và return nil error → caller không biết data không được sync.

```go
// Code hiện tại — SAI
err := esConnection.Product.Bulk(updateItems)
if nil != err {
    logEntry.WithError(err).Error("bulk index failed")
    // ← KHÔNG return, tiếp tục xuống dưới!
}

logEntry.Info("sync product success")  // ← log "success" kể cả khi Bulk fail
return &ppb.SyncProductResponse{}, nil  // ← return nil error kể cả khi Bulk fail
```

```go
// Đề xuất
err := esConnection.Product.Bulk(updateItems)
if nil != err {
    logEntry.WithError(err).Error("bulk index failed")
    return &ppb.SyncProductResponse{}, fmt.Errorf("sync product: bulk index: %w", err)
}

logEntry.Info("sync product success")
return &ppb.SyncProductResponse{}, nil
```

---

## 🟡 SHOULD FIX

### [8] `errors.Is` không được dùng để check `redis.Nil`

**File:** `internal/services/job/service.go:154`

**Vấn đề:** So sánh `err == redis.Nil` thay vì `errors.Is(err, redis.Nil)`. Không tương thích với error wrapping trong go-redis v8.

```go
// Code hiện tại
if err == redis.Nil {

// Đề xuất
if errors.Is(err, redis.Nil) {
```

---

### [9] `SyncProduct` và `SyncInventory` đều xóa cùng key `KeyStoreSyncJob` trong defer

**File:** `internal/worker/sync.go:92` và `sync.go:233`

**Vấn đề:** Cả 2 functions xóa `connect-pos:store:{storeId}:is-syncing` trong defer. Nếu cùng 1 store trigger 2 jobs khác nhau (product + inventory), goroutine nào xong trước sẽ xóa lock của goroutine kia → logic bảo vệ double-sync bị bypass.

**Đề xuất:** Encode job type vào key: `connect-pos:store:{storeId}:is-syncing:{jobType}`, hoặc dùng `DEL` chỉ khi value khớp với jobId (dùng Lua script để atomic check-and-delete).

---

### [10] `fetchInventoryInNetSuite` — return `err == nil` khi location empty thay vì return lỗi mô tả rõ

**File:** `internal/worker/sync.go:410-413`

**Vấn đề:** Khi `countItems > 100000` mà không có location nào → sync silently succeeds mà không sync được gì. `return err` ở đây trả về `nil` (err từ GetList thành công trước đó).

```go
// Code hiện tại
if listLocation == nil || len(listLocation.GetItems()) == 0 {
    logger.Info("list location is empty")
    return err  // err == nil → caller nghĩ là success
}
```

```go
// Đề xuất
if listLocation == nil || len(listLocation.GetItems()) == 0 {
    return fmt.Errorf("fetchInventoryInNetSuite: no locations found, cannot sync large inventory (count=%d)", result.GetCountItems())
}
```

---

### [11] Error không được wrap với context (`%w`)

**File:** `internal/services/job/service.go` (toàn bộ service)

**Vấn đề:** Vi phạm rule "wrap error với context". Tất cả `return nil, err` không wrap → stacktrace mất context.

```go
// Code hiện tại
if err != nil {
    return nil, err  // không có context
}
```

```go
// Đề xuất
if err != nil {
    return nil, fmt.Errorf("CreateJob: marshal job info: %w", err)
}
```

---

### [12] `GetJob` không handle `redis.Nil` cho counter keys — return 500 thay vì 0

**File:** `internal/services/job/service.go:175-188`

**Vấn đề:** `keyJobInfo` miss → `nil, nil` (OK). `keyJobTotal/Success/Failed` miss → `nil, redis.Nil` (500). Inconsistent — counter keys có thể expire trước info key.

```go
// Đề xuất: treat redis.Nil as 0
totalStr, err := s.redisConfig.Reader.Get(ctx, keyJobTotal).Result()
if err != nil {
    if !errors.Is(err, redis.Nil) {
        return nil, fmt.Errorf("GetJob: get total: %w", err)
    }
    totalStr = "0"
}
```

---

### [13] `job.Service` dùng value receiver thay vì pointer receiver

**File:** `internal/services/job/service.go:43,145,230`

**Vấn đề:** Vi phạm rule "pointer receiver nhất quán trong 1 struct". `NewService` trả `*Service` nhưng methods dùng value receiver `(s Service)` — không nhất quán, copy struct mỗi call.

```go
// Code hiện tại
func (s Service) CreateJob(...)
func (s Service) GetJob(...)
func (s Service) CancelJob(...)

// Đề xuất
func (s *Service) CreateJob(...)
func (s *Service) GetJob(...)
func (s *Service) CancelJob(...)
```

---

### [14] `logger.Logger.WithFields(...)` trong `SyncInventory` drops context fields

**File:** `internal/worker/sync.go:314, 326`

**Vấn đề:** `logger` là `*logrus.Entry` với fields đã có. `.Logger` truy cập raw `*logrus.Logger` → drop hết entry fields.

```go
// Code hiện tại — SAI
logger.Logger.WithFields(logrus.Fields{"filter": filter}).Info("...")

// Đề xuất
logger.WithFields(logrus.Fields{"filter": filter}).Info("...")
```

---

## 🔵 SUGGEST

### [15] `ioutil.ReadAll` / `ioutil.NopCloser` deprecated (Go 1.16+)

**File:** `cmd/rest.go:107-108, 122`

```go
// Thay ioutil.ReadAll → io.ReadAll
// Thay ioutil.NopCloser → io.NopCloser
```

---

### [16] `model.JobStatusRunning = "processing"` — tên và value mâu thuẫn

**File:** `internal/model/sync_job.go:27`

Đổi `JobStatusRunning` → `JobStatusProcessing` hoặc value `"processing"` → `"running"`.

---

### [17] `UpsertInventoryWorker` — logEntry stacking trong loop

**File:** `internal/services/inventory/worker.go:35`

```go
// Code hiện tại — sau N vòng có N "filter" fields chồng lên nhau
for {
    logEntry = logEntry.WithField("filter", filter)
```

Move `logEntry.WithField` ra ngoài loop hoặc dùng fresh entry mỗi vòng.

---

### [18] Duplicate constants — `constant.JobSync*` và `model.JobType*`

**Files:** `internal/constant/job.go` và `internal/model/sync_job.go`

Hai set constants giống nhau hoàn toàn. Xóa `constant.JobSync*`, dùng `model.JobType*` làm source of truth.

---

## Câu hỏi cần clarify

- [ ] **Repo stubs** (`repository.go:27-34`): `UpdateStatus`, `GetByID`, `Find` đều return `nil, nil` — stub chưa implement hay intentional? Hiện tại `syncJobRepo` được inject nhưng `Create()` không được gọi trong `CreateJob`. Có plan persist job vào DB không?
- [ ] **`internal/broker/kafka.go`**: `NewKafkaConsumer` với `group.id: "myGroup"` — file này còn dùng không hay đã replaced bởi `segmentio/kafka-go` trong `cmd/sync.go`? Nếu không dùng, xóa đi.

---

**→ Chờ confirm trước khi bắt đầu fix.**
