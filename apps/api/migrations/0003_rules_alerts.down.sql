-- Reverse of 0003_rules_alerts. Drop alerts first (it references rules).
DROP INDEX IF EXISTS alerts_status_idx;
DROP INDEX IF EXISTS alerts_created_at_id_idx;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS rules;
