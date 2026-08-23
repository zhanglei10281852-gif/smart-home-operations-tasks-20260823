DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM devices WHERE kind NOT IN ('sensor','light','thermostat','meter','lock','controller')) THEN
    RAISE EXCEPTION 'cannot add device kind constraint: conflicting rows exist';
  END IF;
  IF EXISTS (SELECT 1 FROM devices WHERE state NOT IN ('pending','paired','enabled','disabled','retired')) THEN
    RAISE EXCEPTION 'cannot add device state constraint: conflicting rows exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='devices_kind_check' AND conrelid='devices'::regclass) THEN
    ALTER TABLE devices ADD CONSTRAINT devices_kind_check CHECK (kind IN ('sensor','light','thermostat','meter','lock','controller'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='devices_state_check' AND conrelid='devices'::regclass) THEN
    ALTER TABLE devices ADD CONSTRAINT devices_state_check CHECK (state IN ('pending','paired','enabled','disabled','retired'));
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM energy_plans WHERE state NOT IN ('draft','scheduled','running','completed','cancelled')) THEN
    RAISE EXCEPTION 'cannot add energy plan state constraint: conflicting rows exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='energy_plans_state_check' AND conrelid='energy_plans'::regclass) THEN
    ALTER TABLE energy_plans ADD CONSTRAINT energy_plans_state_check CHECK (state IN ('draft','scheduled','running','completed','cancelled'));
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM automations WHERE state NOT IN ('draft','active','paused','archived')) THEN
    RAISE EXCEPTION 'cannot add automation state constraint: conflicting rows exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='automations_state_check' AND conrelid='automations'::regclass) THEN
    ALTER TABLE automations ADD CONSTRAINT automations_state_check CHECK (state IN ('draft','active','paused','archived'));
  END IF;
  IF EXISTS (SELECT 1 FROM automation_actions WHERE ordinal < 0) THEN
    RAISE EXCEPTION 'cannot add automation action ordinal constraint: conflicting rows exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='automation_actions_ordinal_check' AND conrelid='automation_actions'::regclass) THEN
    ALTER TABLE automation_actions ADD CONSTRAINT automation_actions_ordinal_check CHECK (ordinal >= 0);
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM automation_runs WHERE state NOT IN ('queued','running','succeeded','failed','cancelled')) THEN
    RAISE EXCEPTION 'cannot add automation run state constraint: conflicting rows exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='automation_runs_state_check' AND conrelid='automation_runs'::regclass) THEN
    ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_state_check CHECK (state IN ('queued','running','succeeded','failed','cancelled'));
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM alerts WHERE severity NOT IN ('info','warning','critical')) THEN
    RAISE EXCEPTION 'cannot add alert severity constraint: conflicting rows exist';
  END IF;
  IF EXISTS (SELECT 1 FROM alerts WHERE state NOT IN ('open','acknowledged','resolved')) THEN
    RAISE EXCEPTION 'cannot add alert state constraint: conflicting rows exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='alerts_severity_check' AND conrelid='alerts'::regclass) THEN
    ALTER TABLE alerts ADD CONSTRAINT alerts_severity_check CHECK (severity IN ('info','warning','critical'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='alerts_state_check' AND conrelid='alerts'::regclass) THEN
    ALTER TABLE alerts ADD CONSTRAINT alerts_state_check CHECK (state IN ('open','acknowledged','resolved'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_automation_runs_queue ON automation_runs(state,id) WHERE state='queued';
CREATE INDEX IF NOT EXISTS idx_audit_request ON audit_events(request_id);

