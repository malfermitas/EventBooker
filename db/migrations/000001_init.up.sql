CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE events (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    booking_ttl_seconds INTEGER NOT NULL CHECK (booking_ttl_seconds > 0),
    requires_payment BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE bookings (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES events(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'CONFIRMED', 'CANCELLED', 'EXPIRED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    confirmed_at TIMESTAMPTZ,
    cancel_reason TEXT,
    CONSTRAINT bookings_confirmed_at_status_chk CHECK (
        (status = 'CONFIRMED' AND confirmed_at IS NOT NULL)
        OR (status <> 'CONFIRMED')
    )
);

CREATE INDEX idx_bookings_event_id_status ON bookings(event_id, status);
CREATE INDEX idx_bookings_expires_at_pending ON bookings(expires_at) WHERE status = 'PENDING';

CREATE UNIQUE INDEX uq_bookings_event_user_active
    ON bookings(event_id, user_id)
    WHERE status IN ('PENDING', 'CONFIRMED');
