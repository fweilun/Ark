-- README: Creates the payments table for the payment module.

CREATE TABLE IF NOT EXISTS payments (
    payment_id     BIGSERIAL PRIMARY KEY,
    trip_id        TEXT NOT NULL REFERENCES orders(id),
    payment_method TEXT NOT NULL CHECK (payment_method IN ('credit_card', 'wallet')),
    amount         BIGINT NOT NULL,
    currency       TEXT NOT NULL DEFAULT 'TWD',
    payment_status TEXT NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('paid', 'pending', 'failed')),
    paid_at        TIMESTAMP
);
