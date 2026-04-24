# 本地開發啟動流程

## 前置需求

- Go 1.21 以上（本專案的 `go.mod` 指定 `go 1.24`，建議直接裝 1.24 或以上）
- Docker 與 Docker Compose（用於本機跑 Postgres + Redis）
- Firebase service account JSON（需要跑帶 auth 的 API 才需要；僅跑 `/health`、`/readyz`、Swagger UI 可以先跳過）
- Google Gemini API key（`GEMINI_API_KEY`，server 啟動時會強制檢查，沒填會直接 fatal）

## 步驟

1. 複製環境變數範本，填入你自己的值：

   ```bash
   cp .env.example .env
   # 編輯 .env，至少填：
   #   GEMINI_API_KEY=<你的 Gemini key>
   #   FIREBASE_CREDENTIALS_JSON=<貼上整份 service account JSON，單行>
   # 其他值本機可以沿用預設。
   ```

2. 啟動本機依賴（Postgres + Redis）：

   ```bash
   docker compose up -d
   ```

3. 確認容器狀態正常（`STATUS` 應該是 `Up` / `healthy`）：

   ```bash
   docker compose ps
   ```

4. 啟動 API server：

   ```bash
   go run ./cmd/ark-api
   ```

   server 啟動時會自動載入專案根目錄的 `.env`，不用再 `source .env`，直接 `go run` 就能吃到 `GEMINI_API_KEY` 等設定。順利的話會看到 `server listening on :8080`。

5. 開 Swagger UI 看 API 文件：

   <http://localhost:8080/swagger/index.html>

## 從 Flutter 連線到本地 API

`ARK_HTTP_ADDR=:8080` 代表 server 綁在所有介面（等同 `0.0.0.0:8080`），所以同網段的手機／模擬器都可以連進來。根據執行環境用對應的 base URL：

| 執行環境            | base URL                            | 備註                                                          |
| ------------------- | ----------------------------------- | ------------------------------------------------------------- |
| Android emulator    | `http://10.0.2.2:8080`              | emulator 內的 `10.0.2.2` 會被轉發到 host 的 `127.0.0.1`。     |
| iOS simulator       | `http://localhost:8080`             | simulator 跟 host 共用 network stack，直接走 localhost。      |
| 實機（Android/iOS） | `http://<筆電 LAN IP>:8080`         | 手機和筆電必須在同一個 Wi-Fi；防火牆要放行 8080。             |

查筆電 LAN IP（macOS）：

```bash
ipconfig getifaddr en0   # Wi-Fi 介面；有線網路通常是 en1
```

連線前可以先從手機的瀏覽器打 `http://<base-url>/health`，看到 `{"status":"ok"}` 就代表 socket 通了。

> ⚠ 實機測試時要確認 macOS 防火牆沒擋 8080（系統設定 → 網路 → 防火牆）。如果擋了，第一次 `go run` 時系統會跳詢問視窗，按「允許」即可。

## 測試需要 Auth 的 API

除了 `GET /health`、`GET /readyz`、`GET /swagger/*` 之外，所有 endpoint 都需要帶
`Authorization: Bearer <Firebase ID Token>`。特別注意 `POST /api/users`（建立後端使用者資料列）也需要帶 token — handler 會把驗證過的 Firebase UID 當作新使用者的 `user_id`，`GET /api/me` 之後才能查得到。取得 token 的兩種方式：

### 方式 A：Firebase Console（最簡單，適合臨時手動測試）

1. 打開 Firebase Console → Authentication → Users。
2. 找到要用的測試帳號，點旁邊的 ⋮ → 取得或重設 ID Token（依主控台語系用字略有差異）。
3. 複製那串 `eyJ...` 開頭的 JWT。

### 方式 B：Firebase Auth REST API（適合寫腳本、反覆測試）

使用 [Identity Toolkit `signInWithPassword`](https://firebase.google.com/docs/reference/rest/auth#section-sign-in-email-password)：

```bash
curl -X POST \
  "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=<WEB_API_KEY>" \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "test@example.com",
    "password": "your-password",
    "returnSecureToken": true
  }'
```

回應裡的 `idToken` 就是要用的 Bearer token（有效期 1 小時，過期要重打一次）。

### 把 token 丟進 Swagger UI

1. 打開 <http://localhost:8080/swagger/index.html>。
2. 右上角點 **Authorize** 按鈕。
3. 在 `FirebaseAuth` 欄位貼上 `Bearer <剛才拿到的 idToken>`（**注意前面要有 `Bearer ` 包含空白**）。
4. 按 Authorize → Close。

之後在 Swagger UI 點任何 endpoint 的 **Try it out**，都會自動帶上 `Authorization` header，不用每次手貼。
