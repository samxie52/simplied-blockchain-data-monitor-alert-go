-- 删除索引
DROP INDEX IF EXISTS idx_alerts_rule_id;
DROP INDEX IF EXISTS idx_alert_rules_user_id;
DROP INDEX IF EXISTS idx_users_telegram_id;

-- 删除表
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS alert_rules;
DROP TABLE IF EXISTS users;