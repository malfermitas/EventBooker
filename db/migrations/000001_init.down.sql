DROP INDEX IF EXISTS uq_bookings_event_user_active;
DROP INDEX IF EXISTS idx_bookings_expires_at_pending;
DROP INDEX IF EXISTS idx_bookings_event_id_status;

DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS users;
