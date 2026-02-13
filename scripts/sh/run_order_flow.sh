#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
PASSENGER_ID=${PASSENGER_ID:-$(python3 - <<'PY'
import os, secrets
print(secrets.token_hex(16))
PY
)}
DRIVER_ID=${DRIVER_ID:-$(python3 - <<'PY'
import secrets
print(secrets.token_hex(16))
PY
)}

request() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local resp body status
  if [ -n "$data" ]; then
    resp=$(curl -s -w "\n%{http_code}" -H "Content-Type: application/json" -X "$method" "$url" -d "$data")
  else
    resp=$(curl -s -w "\n%{http_code}" -X "$method" "$url")
  fi
  body="${resp%$'\n'*}"
  status="${resp##*$'\n'}"
  echo "$status" "$body"
}

json_get() {
  local body="$1" key="$2"
  BODY="$body" KEY="$key" python3 - <<'PY'
import json, os
body=os.environ.get("BODY", "")
key=os.environ.get("KEY")
try:
    data=json.loads(body)
    print(data.get(key, ""))
except Exception:
    print("")
PY
}

printf "BASE_URL=%s\n" "$BASE_URL"
printf "PASSENGER_ID=%s\n" "$PASSENGER_ID"
printf "DRIVER_ID=%s\n\n" "$DRIVER_ID"

# 1) Create order
create_payload=$(cat <<JSON
{
  "passenger_id": "$PASSENGER_ID",
  "pickup_lat": 25.033,
  "pickup_lng": 121.565,
  "dropoff_lat": 25.0478,
  "dropoff_lng": 121.5318,
  "ride_type": "economy"
}
JSON
)

read status body < <(request POST "$BASE_URL/api/orders" "$create_payload")
printf "Create order: status=%s body=%s\n" "$status" "$body"
if [ "$status" != "201" ]; then
  echo "Create order failed" >&2
  exit 1
fi

ORDER_ID=$(json_get "$body" "order_id")
if [ -z "$ORDER_ID" ]; then
  echo "Missing order_id in response" >&2
  exit 1
fi

# 2) Try create again before payment (should conflict)
read status body < <(request POST "$BASE_URL/api/orders" "$create_payload")
printf "Create again (expect 409): status=%s body=%s\n" "$status" "$body"

# 3) Accept
read status body < <(request POST "$BASE_URL/api/orders/$ORDER_ID/accept?driver_id=$DRIVER_ID")
printf "Accept: status=%s body=%s\n" "$status" "$body"

# 4) Arrived
read status body < <(request POST "$BASE_URL/api/orders/$ORDER_ID/arrived")
printf "Arrived: status=%s body=%s\n" "$status" "$body"

# 5) Meet (trip started)
read status body < <(request POST "$BASE_URL/api/orders/$ORDER_ID/meet")
printf "Meet: status=%s body=%s\n" "$status" "$body"

# 6) Complete (enter payment)
read status body < <(request POST "$BASE_URL/api/orders/$ORDER_ID/complete")
printf "Complete: status=%s body=%s\n" "$status" "$body"

# 7) Pay
read status body < <(request POST "$BASE_URL/api/orders/$ORDER_ID/pay")
printf "Pay: status=%s body=%s\n" "$status" "$body"

# 8) Get status
read status body < <(request GET "$BASE_URL/api/orders/$ORDER_ID/status")
printf "Status: status=%s body=%s\n" "$status" "$body"

# 9) Create after payment (should be allowed)
read status body < <(request POST "$BASE_URL/api/orders" "$create_payload")
printf "Create after payment: status=%s body=%s\n" "$status" "$body"
