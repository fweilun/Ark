# Flutter ↔ Ark API 整合交接文件

這份文件交接給負責 zoozoo Flutter 前端的同事。目的是把 Flutter app 接到 Ark Go 後端的 Phase 1：Flutter emulator / 實機呼叫跑在筆電上的 API，完成 Firebase Auth 端到端流程。

- 後端 repo：[`Ark`](https://github.com/anderso3952/Ark)（本 repo）
- 前端 repo：`zoozoo`
- 後端本地啟動方式見 [`README_DEV.md`](../README_DEV.md)

**Phase 1 Exit Criteria**：Flutter emulator 登入後成功呼叫 `GET /api/me` 並顯示使用者資料。

---

## 前置條件

1. **同一個 Firebase project**：後端用的是 `zoozoo-v1`（service account JSON 在 `FIREBASE_CREDENTIALS_JSON` env var）。Flutter `firebase_options.dart` 的 `projectId` 必須一致，否則 token 過不了 `VerifyIDToken`。
2. **筆電已經啟動後端**：參考 `README_DEV.md` 的步驟把 Postgres、Redis、API 都跑起來，`curl http://localhost:8080/health` 要回 `{"status":"ok","version":"0.1.0"}`。
3. **實機測試時筆電跟手機在同一個 Wi-Fi**，防火牆放行 8080（macOS 第一次 `go run` 會跳允許視窗）。

---

## API 連線資訊

### Base URL（依執行環境切換）

| 執行環境            | base URL                        |
| ------------------- | ------------------------------- |
| Android emulator    | `http://10.0.2.2:8080`          |
| iOS simulator       | `http://localhost:8080`         |
| 實機（Android/iOS） | `http://<筆電 LAN IP>:8080`     |

用 `--dart-define=API_BASE_URL=...` 從外面塞進來，程式碼裡不要 hardcode。

```bash
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

### 認證

除了以下 endpoint，其他都要帶 `Authorization: Bearer <Firebase ID Token>`：

- `GET /health`
- `GET /readyz`
- `GET /swagger/*`

**`POST /api/users` 也需要 token**：handler 會從 verified token 拿 Firebase UID 當作新使用者的 `user_id`，所以呼叫前 Flutter 必須先完成 Firebase sign-in、拿到一個有效 idToken，再帶著它打這支 API。

Token 從 Flutter 的 Firebase SDK `user.getIdToken()` 拿；**有效期 1 小時**，interceptor 要自動 refresh。

### API 文件

Swagger UI：<http://localhost:8080/swagger/index.html>（OpenAPI spec 原始檔在 `zooark-openapi.yaml`）

### 金額單位

**所有金額欄位都是 TWD 的 minor unit（cents，1/100 元）**。例：`estimated_fee: 15000` 代表 150 元。UI 顯示時要 ÷100。欄位：`estimated_fee` / `actual_fee` / `incentive_bonus`。

---

## Task 1.2 — 建立 Flutter API Layer

路徑都以 `zoozoo/` 為 root。

### 1.2.1 加依賴

`pubspec.yaml`：

```yaml
dependencies:
  dio: ^5.4.0
  firebase_auth: ^x.y.z   # 版本沿用目前專案的
```

### 1.2.2 `lib/core/network/api_client.dart`

責任：

- `baseUrl` 從 `const String.fromEnvironment('API_BASE_URL')` 讀取；未設定時 fallback 到 dev 預設值（例如 Android emulator 的 `http://10.0.2.2:8080`）。
- 設定 `connectTimeout` / `receiveTimeout`（10–15s）。
- 加一個 `AuthInterceptor`：
  - 每個 request 前呼叫 `FirebaseAuth.instance.currentUser?.getIdToken()` 取得 token，塞進 `Authorization: Bearer <token>` header。
  - 遇到 401 時試著 `getIdToken(true)` 強制 refresh 一次、重發一次；還是 401 就拋出登出事件。
- 加一個 `ErrorInterceptor`：把 Dio 的 `DioException` 統一轉成下面的 `ApiException`，UI 層只面對 `ApiException`。

### 1.2.3 `lib/core/network/api_exception.dart`

建議形狀：

```dart
sealed class ApiException implements Exception {
  final String message;
  const ApiException(this.message);
}

class NetworkException     extends ApiException { /* 連線失敗、timeout */ }
class UnauthorizedException extends ApiException { /* 401 */ }
class NotFoundException     extends ApiException { /* 404 */ }
class ValidationException   extends ApiException { /* 4xx with field errors */ }
class ServerException       extends ApiException { /* 5xx */ }
class UnknownApiException   extends ApiException { }
```

後端錯誤回應格式都是 `{"error": "<message>"}`（見 `internal/httpx/error.go`），parse 這個欄位當作 message。

---

## Task 1.3 — 對齊 OrderStatus enum

後端完整 11 種狀態（`internal/modules/order/model.go:13-25`；不含 `none`，`none` 是 pre-create sentinel）：

| 狀態          | 意義                                                |
| ------------- | --------------------------------------------------- |
| `scheduled`   | 預約單已建立，等司機認領                            |
| `waiting`     | 即時單已建立，等媒合                                |
| `assigned`    | 司機已接單，尚未出發（scheduled 認領後也進這個）    |
| `approaching` | 司機前往上車點中                                    |
| `arrived`     | 司機已到上車點                                      |
| `driving`     | 乘客上車、行程中                                    |
| `payment`     | 行程結束、等付款                                    |
| `complete`    | 訂單完成                                            |
| `cancelled`   | 取消（乘客或司機都可能觸發）                        |
| `denied`      | 司機拒絕媒合（單子會回 waiting）                    |
| `expired`     | 媒合逾時，訂單過期                                  |

Flutter 端的 `OrderStatus` enum 要完整列出這 11 種（建議也加 `none` 代表「尚未建立」以方便表示 optional state），所有 UI 判斷（button 顯示、顏色、文案）都要對齊這個 enum，原本 5 種狀態的 switch/if 要補完缺的 case。**務必加 `default` / exhaustive check**，之後後端再加狀態時編譯器會提醒。

合法狀態轉移見 `AllowedTransitions`（`internal/modules/order/model.go:83`）。前端基本上不需要自己驗 transition，直接照 API 回傳的 `status` 走就好，但顯示可取消按鈕等 UI 判斷可以參考。

---

## Task 1.4 — 對齊 Order model

後端 `Order` struct（`internal/modules/order/model.go:34-67`）JSON shape：

```jsonc
{
  "id": "string",
  "passenger_id": "string",
  "driver_id": "string | null",
  "status": "waiting",
  "status_version": 0,
  "pickup": { "lat": 25.03, "lng": 121.56 },
  "dropoff": { "lat": 25.04, "lng": 121.57 },
  "ride_type": "string",
  "estimated_fee": 15000,              // TWD cents
  "actual_fee": 15000,                 // TWD cents, nullable
  "created_at": "2026-04-22T12:34:56Z",
  "matched_at": "…",                   // nullable
  "accepted_at": "…",                  // nullable
  "started_at": "…",                   // nullable
  "completed_at": "…",                 // nullable
  "cancelled_at": "…",                 // nullable
  "cancel_reason": "string",           // nullable

  // 預約單專用；即時單這幾個欄位會是空/null
  "order_type": "instant | scheduled",
  "scheduled_at": "…",                 // nullable
  "schedule_window_mins": 15,          // nullable int
  "cancel_deadline_at": "…",           // nullable
  "incentive_bonus": 0,                // TWD cents
  "assigned_at": "…"                   // nullable
}
```

### `order_model.dart` 要補的欄位

- `status_version` — **重要**，之後做 optimistic concurrency（PATCH 時帶上版本號避免 race）會用到；現在先存著就好。
- `order_type` — `"instant"` 或 `"scheduled"`。
- `scheduled_at` / `schedule_window_mins` / `cancel_deadline_at` — 預約單才有值。
- `incentive_bonus` — 預約單的司機加碼，TWD cents。

### 純 UI 欄位

`vehicleEmoji`、任何顯示用的圖示/顏色/文案都**不從 API 讀**，改為 local computed property（從 `ride_type` / `status` 推出來）。

### 金額顯示

所有金額欄位（`estimated_fee`、`actual_fee`、`incentive_bonus`）後端都是 TWD cents。Flutter 顯示時統一走一個 helper：`int cents → '\$${(cents / 100).toStringAsFixed(0)}'`。

---

## Task 1.5 — 實作 Auth 串接

### Firebase OTP 成功後呼叫 `POST /api/users`

Request shape（見 `internal/http/handlers/user_handler.go:29`）：

```json
{
  "name": "string",       // required
  "email": "string",      // required
  "phone": "string",      // optional
  "user_type": "rider | driver"   // required
}
```

- **此 endpoint 需要帶 `Authorization: Bearer <idToken>`**；body 裡**不要**塞 `user_id`，server 會直接用 token 裡驗證過的 Firebase UID。
- Response 201 回完整的 `UserResponse`（跟 `GET /api/me` 同一個 DTO）。
- 順序很關鍵：Firebase sign-in 成功 → `getIdToken()` 拿到 token → interceptor 接手 → 呼叫 `POST /api/users`。如果 token 還沒拿到就先打這支 API 會 401。

### `completeRegistration()` in `auth_bloc.dart`

流程：

1. Firebase OTP 成功 → 取得 `User` 物件。
2. 呼叫 `POST /api/users` 建立後端 profile。
3. 呼叫 `GET /api/me` 確認後端查得到（同時拿到完整 user DTO 塞進 app state）。
4. 成功 → state 轉成 `Authenticated(user)`；失敗要能 retry 或登出重來。

### 同場加映：`GET /api/me`

- 只需要 `Authorization` header，沒有其他 param。
- 404 代表使用者還沒在後端建立（應該要把 user 踢回「完成註冊」畫面）。
- 401 代表 token 過期或無效，interceptor 應該會自動 refresh；若仍 401 → 登出。

---

## Task 1.6 — Smoke Test

驗證整條鏈。

### Checklist

1. [ ] 後端 running，`curl http://localhost:8080/health` → `{"status":"ok","version":"0.1.0"}`。
2. [ ] Flutter emulator 跑起來，`--dart-define=API_BASE_URL=http://10.0.2.2:8080`（Android）或 `http://localhost:8080`（iOS）。
3. [ ] Firebase 登入（OTP 流程）成功。
4. [ ] `POST /api/users` 回 201（記得帶 `Authorization: Bearer <idToken>`）。後端 log 應該看到一筆 `POST /api/users` 201。
5. [ ] `GET /api/me` 回 200、payload 是剛剛建立的使用者。
6. [ ] 把 app 殺掉重開，自動 login 後 `GET /api/me` 還是回 200（token refresh 正常）。

### 常見失敗

| 症狀                     | 多半原因                                                                 |
| ------------------------ | ------------------------------------------------------------------------ |
| Connection refused       | base URL 錯（emulator 要用 `10.0.2.2`），或 firewall 擋 8080，或後端沒啟 |
| 401 `invalid or expired token` | token 過期、Firebase project 不一致、`FIREBASE_CREDENTIALS_JSON` 沒設 |
| 404 on `/api/me`         | 還沒呼叫過 `POST /api/users`，或上次建立時 token 無效 / UID 不一致         |
| 401 on `/api/users`      | 沒帶 token、token 過期、或 Firebase project 不是 `zoozoo-v1`              |
| 400 on `/api/users`      | `user_type` 沒帶或不是 `rider` / `driver`；`name` / `email` 空            |
| CORS 錯誤                | 只有 Flutter **web** 會遇到；目前後端沒有 CORS middleware，需要另開任務  |

---

## Appendix

### 後續會用到的 endpoint（Phase 2 以後）

叫車／行程相關的完整 endpoint 表見 Swagger UI。重點流程：

- 乘客建單：`POST /api/orders`（即時）／ `POST /api/orders/scheduled`（預約）
- 司機接單：`POST /api/orders/{id}/accept`
- 狀態推進：`arrived` → `meet`（= start driving）→ `complete` → `pay`
- 取消：`POST /api/orders/{id}/cancel`（乘客）／ `POST /api/orders/{id}/driver-cancel`（司機，預約單）

訂單整個狀態機圖 / 解釋見 [`docs/orderflow.md`](./orderflow.md) 與 [`docs/schedule.md`](./schedule.md)。

### 後端聯絡方式 / 問題回報

有疑問先看 Swagger UI 和這份文件。若：

- 某 endpoint response 跟文件對不起來 → 回報 backend，附上 request / response。
- 401 / 403 疑似 auth 問題 → 確認 Firebase projectId、token 沒過期，再回報。
- 需要新 endpoint 或欄位 → 在 repo 開 issue，標 `backend`。
