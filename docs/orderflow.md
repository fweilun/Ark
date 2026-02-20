# Order flow 流程與測試

## Struct Order

```go
type Status string

const (
    StatusNone    Status = "none"
    StatusWaiting Status = "waiting" // user is waiting
    StatusApproaching Status = "approaching" // the driver is heading to the user
    StatusArrived Status = "arrived" // the driver is arrived
    StatusDriving Status = "driving" // ride in progress
    StatusPayment Status = "payment"
    StatusComplete Status = "complete" // the order has been completed
    StatusCancelled Status = "cancelled" // the order has been cancelled
    StatusDenied Status = "denied" // driver denied the order
)

type Order struct {
    ID            types.ID
    PassengerID   types.ID
    DriverID      *types.ID
    Status        Status
    StatusVersion int
    Pickup        types.Point
    Dropoff       types.Point
    RideType      string
    EstimatedFee  types.Money
    ActualFee     *types.Money
    CreatedAt     time.Time
    MatchedAt     *time.Time
    AcceptedAt    *time.Time
    StartedAt     *time.Time
    CompletedAt   *time.Time
    CancelledAt   *time.Time
    CancelReason  *string
}
```

Order設計依照以下workflow，一個訂單會由Request Ride開始。
若 user cancelled 或司機拒絕/取消，狀態會移動到 Ride Denied/Cancelled；司機取消時，App 端會再觸發一次 Request Ride 重新媒合。Ride Denied 與取消都會記錄到 db。

目前的

```mermaid
flowchart TB
  classDef state fill:#ffffff,stroke:#1f2937,stroke-width:1px,rx:8,ry:8,color:#111111;
  classDef terminal fill:#e6fffb,stroke:#0f766e,stroke-width:1.5px,rx:8,ry:8,color:#111111;
  classDef exception fill:#fff1f2,stroke:#be123c,stroke-width:1.5px,rx:8,ry:8,color:#111111;

  subgraph StateManager["Enhanced Order State Machine"]
    direction TB

    %% 初始訂單類型
    ScheduledOrder["Scheduled Order Created<br/>(預約單建立)"]
    RealtimeOrder["Realtime Order Created<br/>(即時單建立)"]

    %% 預約單進入媒合流程
    ScheduledOrder --> AwaitingDriver["Awaiting Driver<br/>(等待司機接單)"]

    %% 媒合中的轉換
    AwaitingDriver --> |driver accepts| AcceptedWaiting["Accepted (Waiting)<br/>已接單待出發"]
    AwaitingDriver --> |driver declines| AwaitingDriver
    AwaitingDriver --> |matching timeout| Expired["Expired<br/>訂單過期"]

    %% 即時單的媒合（直接推送）
    RealtimeOrder --> AwaitingDriverImmediate["Awaiting Driver<br/>(即時等待)"]
    AwaitingDriverImmediate --> |driver accepts| Approaching
    AwaitingDriverImmediate --> |driver declines| AwaitingDriverImmediate
    AwaitingDriverImmediate --> |matching timeout| Expired

    %% 預訂單出發
    AcceptedWaiting --> |driver starts| Approaching

    %% 共同行進路徑
    Approaching --> |driver arrives| Arrived["Arrived<br/>已抵達"]
    Arrived --> |passenger onboard / driver starts trip| Driving["Driving<br/>行程中"]
    Driving --> |drop off /到达目的地| Payment["Payment<br/>支付中"]
    Payment --> |payment success| Complete["Complete<br/>已完成"]

    %% 取消路徑（所有非終止狀態皆可取消）
    AwaitingDriver --> |user cancels| Cancelled
    AwaitingDriverImmediate --> |user cancels| Cancelled
    AcceptedWaiting --> |user cancels| Cancelled
    AcceptedWaiting --> |driver cancels| Cancelled
    Approaching --> |user cancels| Cancelled
    Approaching --> |driver cancels| Cancelled
    Arrived --> |user cancels| Cancelled
    Arrived --> |driver cancels| Cancelled
    Driving --> |driver cancels| Cancelled

    %% 超時例外
    %% Arrived --> |timeout| NoShow["NoShow<br/>乘客未出現"]

    %% 最終終止狀態
    Cancelled --> EndCancelled["終止"]
    Expired --> EndExpired["終止"]
    Blank --> EndNoShow["Blank"]
    Complete --> EndComplete["終止"]
  end

  class ScheduledOrder,RealtimeOrder,AwaitingDriver,AwaitingDriverImmediate,AcceptedWaiting,Approaching,Arrived,Driving,Payment state;
  class Complete terminal;
  class Expired,Cancelled,Blank exception;
```

## Cases:

App User flow (for app):
```mermaid
flowchart TB
  classDef action fill:#ffffff,stroke:#1f2937,stroke-width:1px,rx:8,ry:8,color:#111111;
  classDef status fill:#eef2ff,stroke:#4338ca,stroke-width:1px,rx:8,ry:8,color:#111111;
  classDef warning fill:#fff7ed,stroke:#c2410c,stroke-width:1.5px,rx:8,ry:8,color:#111111;
  classDef terminal fill:#ecfeff,stroke:#0f766e,stroke-width:1.5px,rx:8,ry:8,color:#111111;

  Start["User: Request Ride <br>(1. POST /api/orders)"] --> Poll["User: Polling Status <br> (2. GET /api/orders/{id}/status)"]
  
  Poll -->|Status: StatusWaiting| CheckTimeout{Timeout?}
  CheckTimeout -- No --> Poll
  CheckTimeout -- Yes --> Expired["Show: No Driver Found <br> 4. POST /api/orders/:id/cancel"]
  
  Poll -->|Status: StatusApproaching| ShowDriver["Update Map (Show Driver info) <br> 3. call map api"]
  ShowDriver --> Poll
  Poll -->|Status: StatusArrived| OnBoard["Show: On Ride"]
  Poll -->|Status: StatusDriving| OnBoard["Show: On Ride"]
  Poll -->|Status: StatusPayment| Payment["Show: Payment/Rating"]
  Poll -->|Status: StatusComplete| Done["Show: Completed"]

  Poll -->|Status: StatusCancelled| Canceled["End: User Cancelled <br> 4. POST /api/orders/:id/cancel"]

  class Start,Poll,ShowDriver,OnBoard action;
  class CheckTimeout status;
  class Expired,Canceled warning;
  class Payment,Done terminal;
```
* Request Ride
```http
POST {{baseUrl}}/api/orders
Content-Type: application/json

{
    "passenger_id": "1aa2vvdd3",
    "pickup_lat": 25.033,
    "pickup_lng": 121.565,
    "dropoff_lat": 25.0478,
    "dropoff_lng": 121.5318,
    "ride_type": "economy"
}
Expected: Success, Fail
```
* Check status
```http
GET {{baseUrl}}/api/orders/{{order_id}}/status
Expected: All Status
```

* User cancel order
```http
POST {{baseUrl}}/api/orders/{{order_id}}/cancel
```




* Case 1

```txt
User request ride:
User cancelled:
```
* Case 2

```txt
User request ride:
Driver Found:
Driver Accept/ Denied:
User cancelled:
```

* Case 3

```txt
User request ride:
Driver Found:
Driver Accept:
Driver Denied:
```








Driver workflow:
```mermaid
flowchart TB
    Idle[StatusWaiting <br> 1. /api/drivers/driver_id/orders] -->|receive order| Accept{Accept？ <br> 2. /api/orders/:id/accept?driver_id=... <br> 3. /api/orders/:id/deny?driver_id=...}
    Accept -- Yes --> Going[StatusApproaching]
    Accept -- No --> Denied[StatusDenied <br> 3. /api/orders/:id/deny?driver_id=...]
    Denied --> Idle

    Going --> |4. /api/orders/:id/arrived| Arrived[StatusArrived]
    Arrived --> |5. /api/orders/:id/meet| Driving[StatusDriving]
    Driving --> |6. /api/orders/:id/complete| Complete[StatusPayment]
    Complete --> |7. /api/orders/:id/pay| Done[StatusComplete]
    Done --> Idle

    Arrived -->|driver/ user cancelled| Cancelled
    Going --> Cancelled[StatusCancelled <br> /api/orders/:id/cancel]
    Cancelled --> Idle

```
