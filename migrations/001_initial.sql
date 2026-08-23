CREATE TABLE IF NOT EXISTS households (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  timezone TEXT NOT NULL,
  monthly_budget_cents BIGINT NOT NULL CHECK (monthly_budget_cents >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS members (
  id BIGSERIAL PRIMARY KEY,
  household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  email TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('owner','operator','viewer')),
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(household_id, email)
);
CREATE TABLE IF NOT EXISTS sessions (
  id UUID PRIMARY KEY,
  member_id BIGINT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS devices (
  id BIGSERIAL PRIMARY KEY,
  household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  external_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('sensor','light','thermostat','meter','lock','controller')),
  state TEXT NOT NULL CHECK (state IN ('pending','paired','enabled','disabled','retired')),
  firmware TEXT NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  last_seen_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(household_id, external_id)
);
CREATE TABLE IF NOT EXISTS device_capabilities (
  device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  capability TEXT NOT NULL,
  PRIMARY KEY(device_id, capability)
);
CREATE TABLE IF NOT EXISTS telemetry (
  id BIGSERIAL PRIMARY KEY,
  device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  sequence BIGINT NOT NULL,
  power_watts NUMERIC(12,3) NOT NULL,
  temperature_c NUMERIC(8,3),
  measured_at TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(device_id, sequence)
);
CREATE TABLE IF NOT EXISTS energy_plans (
  id BIGSERIAL PRIMARY KEY,
  household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('draft','scheduled','running','completed','cancelled')),
  budget_cents BIGINT NOT NULL CHECK (budget_cents >= 0),
  starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS plan_devices (
  plan_id BIGINT NOT NULL REFERENCES energy_plans(id) ON DELETE CASCADE,
  device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
  target_watts NUMERIC(12,3) NOT NULL CHECK (target_watts >= 0),
  PRIMARY KEY(plan_id, device_id)
);
CREATE TABLE IF NOT EXISTS automations (
  id BIGSERIAL PRIMARY KEY,
  household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('draft','active','paused','archived')),
  trigger_kind TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS automation_actions (
  id BIGSERIAL PRIMARY KEY,
  automation_id BIGINT NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
  device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
  action TEXT NOT NULL,
  ordinal INT NOT NULL CHECK (ordinal >= 0),
  UNIQUE(automation_id, ordinal)
);
CREATE TABLE IF NOT EXISTS automation_runs (
  id BIGSERIAL PRIMARY KEY,
  automation_id BIGINT NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('queued','running','succeeded','failed','cancelled')),
  error_text TEXT,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  UNIQUE(automation_id, idempotency_key)
);
CREATE TABLE IF NOT EXISTS alerts (
  id BIGSERIAL PRIMARY KEY,
  household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL,
  severity TEXT NOT NULL CHECK (severity IN ('info','warning','critical')),
  code TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('open','acknowledged','resolved')),
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS audit_events (
  id BIGSERIAL PRIMARY KEY,
  household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  actor_member_id BIGINT REFERENCES members(id) ON DELETE SET NULL,
  request_id TEXT NOT NULL,
  object_type TEXT NOT NULL,
  object_id TEXT NOT NULL,
  action TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS outbox_messages (
  id BIGSERIAL PRIMARY KEY,
  household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  topic TEXT NOT NULL,
  payload JSONB NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_at TIMESTAMPTZ,
  delivered_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_telemetry_device_time ON telemetry(device_id, measured_at);
CREATE INDEX IF NOT EXISTS idx_alerts_household_state ON alerts(household_id, state);
CREATE INDEX IF NOT EXISTS idx_outbox_ready ON outbox_messages(available_at) WHERE delivered_at IS NULL;
