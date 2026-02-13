# Order flow 流程與測試

## Struct Order

```go
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

  subgraph StateManager["Overall workflow"]
    direction TB
    RequestRide["User: Request Ride"] --> DriverFound["Server: Driver Found"] --> RideAccepted["Driver: Ride Accepted"] --> TripStarted["Trip Started"] --> TripComplete["Trip Complete"] --> Payment["Payment"]
    RequestRide --> Cancelled["User: Cancelled"]
    DriverFound --> RideDenied["Driver: Ride Denied"]
    RideAccepted --> RideDenied["Driver: Ride Denied"]
  end

  class RequestRide,DriverFound,RideAccepted,TripStarted,TripComplete state;
  class Payment terminal;
  class Cancelled,RideDenied exception;
```

## Cases:

App User flow:
```mermaid
flowchart TB
  classDef action fill:#ffffff,stroke:#1f2937,stroke-width:1px,rx:8,ry:8,color:#111111;
  classDef status fill:#eef2ff,stroke:#4338ca,stroke-width:1px,rx:8,ry:8,color:#111111;
  classDef warning fill:#fff7ed,stroke:#c2410c,stroke-width:1.5px,rx:8,ry:8,color:#111111;
  classDef terminal fill:#ecfeff,stroke:#0f766e,stroke-width:1.5px,rx:8,ry:8,color:#111111;

  Start["User: Request Ride <br>(1. POST /api/orders)"] --> Poll["User: Polling Status <br> (2. GET /api/orders/{id}/status)"]
  
  Poll -->|Status: WAITING| CheckTimeout{Timeout?}
  CheckTimeout -- No --> Poll
  CheckTimeout -- Yes --> Expired["Show: No Driver Found <br> 4. POST /api/orders/:id/cancel"]
  
  Poll -->|Status: MATCHED| ShowDriver["Update Map (Show Driver info) <br> 3. call map api"]
  ShowDriver --> Poll
  Poll -->|Status: IN_PROGRESS| OnBoard["Show: On Ride"]
  Poll -->|Status: COMPLETED| Payment["Show: Payment/Rating"]

  Poll -->|Status: CANCELLED| Canceled["End: User Cancelled <br> 4. POST /api/orders/:id/cancel"]

  class Start,Poll,ShowDriver,OnBoard action;
  class CheckTimeout status;
  class Expired,Canceled warning;
  class Payment terminal;
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
POST {{baseUrl}}/api/orders/{{order_id}}/status
Expected: WAITING | MATCHED | IN_PROGRESS | COMPLETED | CANCELLED
```

* User cancel order
```http
POST {{baseUrl}}/api/orders/{{order_id}}/cancelled
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
