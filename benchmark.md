# MVP Benchmark Tests

> 依照 `test.md` 生成的自動化/半自動測試清單（可作為 benchmark 基準）。
> 執行：`go run ./cmd/bench`（需先啟動 API、Postgres、Redis）

## 1. 環境與啟動

- **Env: Postgres connect**
  測試重點：DB 連線是否正常
- **Env: Redis connect**
  測試重點：Redis 連線是否正常
- **Migration: apply (optional)**
  測試重點：可選套用 `migrations/0001_init.sql`
- **Migration: tables exist**
  測試重點：依 migration 檢查 orders/order_state_events/location_snapshots/pricing_rates 是否存在
- **API: server reachable**
  測試重點：API 是否可回應請求

## 2. 訂單流程（核心）

- **Order: passenger request ride (valid)**
  測試重點：可建立訂單，狀態應為 created
- **Order: passenger request ride (missing fields -> 400)**
  測試重點：缺欄位回 400
- **Order: passenger request ride (duplicate active)**
  測試重點：重複下單應回 409
- **Order: driver accept (valid)**
  測試重點：可接受配對訂單
- **Order: driver accept (invalid state)**
  測試重點：錯誤狀態不得接受
- **Order: driver start trip**
  測試重點：accepted -> in_progress
- **Order: driver complete trip**
  測試重點：in_progress -> completed
- **Order: get order status**
  測試重點：查詢訂單狀態回 200
- **Order: completed cannot transition** (Manual)
  測試重點：完成後不可再進行狀態轉移

## 3. 取消流程

- **Cancel: passenger cancel (created)**
  測試重點：created 可取消
- **Cancel: passenger cancel (accepted/in_progress -> reject)**
  測試重點：進行中不可取消
- **Cancel: driver reject order**
  測試重點：司機拒單行為
- **Cancel: timeout cancel** (Manual)
  測試重點：逾時未接單應自動取消

## 4. 配對流程

- **Matching: driver availability**
  測試重點：司機可設定可用狀態並加入候選池
- **Matching: no nearby driver** (Manual)
  測試重點：無附近司機時不應產生配對
- **Matching: ride type filter** (Manual)
  測試重點：車型/ride_type 濾條件生效
- **Matching: remove from pool after match** (Manual)
  測試重點：配對後候選池移除

## 5. 位置更新

- **Location: update passenger**
  測試重點：乘客位置更新寫入 Redis GEO
- **Location: update driver**
  測試重點：司機位置更新寫入 Redis GEO
- **Location: invalid coords -> 400**
  測試重點：非法座標拒絕
- **Location: throttling** (Manual)
  測試重點：位置更新節流避免 DB 壓力
- **Location: snapshot persisted** (Manual)
  測試重點：位置快照寫入 Postgres

## 6. Pricing

- **Pricing: estimate (valid)**
  測試重點：有費率時可估價
- **Pricing: missing rate**
  測試重點：無費率時應回錯
- **Pricing: distance 0 -> base fare** (Manual)
  測試重點：距離為 0 仍回 base fare

## 7. 資料一致性

- **Consistency: orders/status/events一致** (Manual)
  測試重點：`orders.status` 與 `order_state_events` 一致
- **Consistency: status_version 遞增** (Manual)
  測試重點：狀態轉移版本號遞增
- **Consistency: cancelled cannot complete** (Manual)
  測試重點：取消後不可完成

## 8. 競態與邊界

- **Concurrency: multi accept same order**
  測試重點：同一訂單只允許一位司機成功接受
- **Concurrency: cancel vs accept** (Manual)
  測試重點：同時 cancel/accept 以先到者為準
- **Concurrency: duplicate requests idempotent** (Manual)
  測試重點：重送不造成重複寫入

## 9. 錯誤與回復

- **Error: DB down -> 500** (Manual)
  測試重點：DB 斷線時回 500
- **Error: Redis down -> matching not run** (Manual)
  測試重點：Redis 斷線時 matching 停止或回合理錯誤
- **Error: restart recover orders** (Manual)
  測試重點：重啟後仍能處理既有訂單

## 10. 基本效能（MVP）

- **Perf: location update throughput**
  測試重點：位置更新吞吐
- **Perf: request ride throughput**
  測試重點：下單吞吐

---

## 執行說明

```bash
# 基本執行
go run ./cmd/bench

# 套用 migration 再跑
go run ./cmd/bench -apply-migration

# 自訂參數
go run ./cmd/bench -base-url http://localhost:8080 -dsn "postgres://..." -redis "localhost:6379" -concurrency 50 -duration 15s
```
