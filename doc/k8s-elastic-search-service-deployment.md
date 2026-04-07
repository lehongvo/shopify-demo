# k8s-staging — Elastic Search Service Deployment Guide

> Tài liệu này mô tả cách `elastic-search-service` được deploy trên Kubernetes (staging + dev), bao gồm ConfigMap, các mode pod, CronJob, và sự khác biệt giữa 2 môi trường.

---

## 1. Tổng quan

`elastic-search-service` (ess) được deploy thành **nhiều pod riêng biệt**, mỗi pod chạy 1 subcommand khác nhau từ **cùng 1 image**:

```
elastic-search-service <subcommand>
  ├── serve grpc     → gRPC API server (query ES)
  ├── serve rest     → REST API server
  ├── cron           → Scheduler dài hạn (chạy mãi)
  ├── worker sync    → Kafka consumer (reactive sync)
  └── pull           → One-shot batch pull (K8s CronJob hoặc manual)
```

---

## 2. ConfigMap — Connection thực tế

### Dev (`spos-dev`)

| Key | Value |
|---|---|
| `ESADDRESS` | `http://10.0.28.208:9200` |
| `ESUSERNAME` | `elastic` |
| `REDISADDRESSWRITE` | `redis-master.redis.svc.cluster.local:6379` |
| `REDISADDRESSREAD` | `redis-master.redis.svc.cluster.local:6379` |
| `KAFKABROKERS` | `kafka-cluster-kafka-brokers.strimzi:9092` |
| `HERMES` | `hermes-grpc:9899` |
| `DRAGONFLY` | `dragonfly-grpc:9110` |
| `SETTING` | `setting-grpc:9055` |

### Staging (`spos-stag`)

| Key | Value |
|---|---|
| `ESADDRESS` | `http://10.0.85.19:9200` |
| `REDISADDRESSWRITE` | `redis-stag-master.redis.svc.cluster.local:6379` |
| `COCOON` | `cocoon-grpc:9535` ← **thêm so với dev** |

> Staging có thêm `COCOON` endpoint — dev chưa kết nối Cocoon.

---

## 3. Các Pod được deploy

### 3.1 `pos-elastic-search/pod.yaml` — API Server

Chạy **2 container** trong 1 pod:

```yaml
# Container 1: gRPC server
command: [elastic-search-service, serve, grpc]
containerPort: 9035

# Container 2: REST server
command: [elastic-search-service, serve, rest]
containerPort: 9038
```

Dùng để: nhận query từ các service khác tìm product/inventory/customer trong ES.

### 3.2 `pos-elastic-search/worker.yaml` (dev only) — Kafka Consumer

```yaml
command: [elastic-search-service, worker, sync]
```

Lắng nghe Kafka topic → khi có `JobEvent` (SyncProduct/SyncInventory) → pull ngay lập tức cho store đó.

### 3.3 `pos-elastic-search-cron/pod.yaml` — Cron Daemon (dev)

```yaml
command: [elastic-search-service, cron]
env:
  PARTITIONS: "-1"   # partition đặc biệt (store ưu tiên)
```

Chạy mãi, tick theo schedule trong `storeListByPartitions[-1]`.

**Pod Anti-Affinity** — 8 cron pod được schedule trên các node khác nhau:
```
pos-esearch-cron, pos-esearch-cron-2, ..., pos-esearch-cron-7, pos-esearch-cron-inventory
```
→ Mỗi pod 1 node riêng, tránh 1 node gánh hết.

### 3.4 `pos-elastic-search-cron/marina-cronjob.yaml` — K8s CronJob (production)

```yaml
kind: CronJob
schedule: "*/10 0-15,17-23 * * *"   # mỗi 10 phút, trừ 11PM GMT+7
command:
  - elastic-search-service
  - pull
  - --storeId=7702
  - --all=false
```

Khác với cron daemon: đây là **K8s CronJob** — mỗi lần tick tạo 1 pod mới, chạy xong thì xóa. Dùng cho store cụ thể cần lịch riêng.

---

## 4. Sự khác biệt Dev vs Staging

| | `spos-dev` | `spos-stag` |
|---|---|---|
| **API server** | ✓ (pod.yaml) | ✓ (pod.yaml) |
| **Worker Kafka** | ✓ (worker.yaml) | ✗ không có |
| **Cron daemon** | ✓ (8 pod, PARTITIONS=-1~8) | ✗ không có |
| **K8s CronJob** | ✓ (marina + per-store jobs) | ✗ chưa cấu hình |
| **COCOON endpoint** | ✗ | ✓ |
| **ES endpoint** | `10.0.28.208:9200` | `10.0.85.19:9200` |
| **Redis** | `redis-master` | `redis-stag-master` |

---

## 5. Cách update image mới

Jenkins build xong → tự commit manifest vào repo này → ArgoCD sync lên cluster (~3 phút).

Để update thủ công:
```yaml
# Sửa image tag trong pod.yaml:
image: 647911573570.dkr.ecr.eu-north-1.amazonaws.com/pos-elastic-search-service:<new-tag>
```
Commit + push → ArgoCD tự apply.

---

## 6. Partition mapping trên K8s

Mỗi cron pod set `PARTITIONS=N` khác nhau:

| Pod | PARTITIONS | Store list |
|---|---|---|
| `pos-esearch-cron` | `-1` | Store ưu tiên (PosConnect, grahambrown) |
| `pos-esearch-cron-2` | `2` | SchoolUniformMe, BakedByMelissa, ... |
| `pos-esearch-cron-3` | `3` | BirdsNest, PulPeria, Polyalkemi, ... |
| ... | ... | ... |
| `pos-esearch-cron-inventory` | inventory only | Chỉ sync inventory |

Xem full mapping: `elastic-search-service/internal/model/store.go`.

---

## 7. Debug nhanh trên K8s

```bash
# Xem log cron pod
kubectl logs -n spos-dev deployment/pos-esearch-cron -f

# Xem log worker
kubectl logs -n spos-dev deployment/pos-esearch -c pos-es-sync-worker -f

# Xem log API server
kubectl logs -n spos-dev deployment/pos-esearch -c pos-esearch-gr -f

# Manual pull 1 store (tạo pod tạm)
kubectl run ess-pull --rm -it --image=647911573570.dkr.ecr.eu-north-1.amazonaws.com/pos-elastic-search-service:dev-8bb7c41 \
  --restart=Never -n spos-dev \
  --env="HERMES=hermes-grpc:9899" \
  --env="ESADDRESS=http://10.0.28.208:9200" \
  --env="REDISADDRESSWRITE=redis-master.redis.svc.cluster.local:6379" \
  --env="SETTING=setting-grpc:9055" \
  -- elastic-search-service pull --storeId=5981 --all=true

# Check Redis watermark
kubectl exec -n redis redis-master-0 -- redis-cli -a Dtba82xJ0KNr7 GET "stores.5981.last.fetch.inventory"
```

---

## 8. File structure trong repo k8s-staging

```
k8s-staging/
├── spos-dev/
│   ├── pos-elastic-search/
│   │   ├── configmap.yaml    ← connection strings
│   │   ├── pod.yaml          ← serve grpc + serve rest
│   │   ├── svc.yaml          ← K8s Service expose port 9035
│   │   └── worker.yaml       ← worker sync (Kafka)
│   └── pos-elastic-search-cron/
│       ├── pod.yaml          ← cron PARTITIONS=-1
│       ├── pod2.yaml ~ pod8.yaml  ← cron PARTITIONS=2~8
│       ├── pod-inventory.yaml    ← inventory-only cron
│       ├── marina-cronjob.yaml   ← K8s CronJob store 7702
│       └── job.yaml              ← one-shot Job
├── spos-stag/
│   └── pos-elastic-search/
│       ├── configmap.yaml    ← staging connections (khác ES IP, thêm COCOON)
│       ├── pod.yaml          ← serve grpc only
│       └── svc.yaml
```
