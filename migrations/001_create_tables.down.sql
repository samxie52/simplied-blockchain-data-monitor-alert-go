-- 删除视图
DROP VIEW IF EXISTS alert_statistics;
DROP VIEW IF EXISTS user_activity_statistics;
DROP VIEW IF EXISTS transaction_statistics;
DROP VIEW IF EXISTS block_statistics;

-- 删除触发器
DROP TRIGGER IF EXISTS update_subscriptions_updated_at ON subscriptions;
DROP TRIGGER IF EXISTS update_alerts_updated_at ON alerts;
DROP TRIGGER IF EXISTS update_alert_rules_updated_at ON alert_rules;
DROP TRIGGER IF EXISTS update_transaction_logs_updated_at ON transaction_logs;
DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
DROP TRIGGER IF EXISTS update_blocks_updated_at ON blocks;
DROP TRIGGER IF EXISTS update_user_sessions_updated_at ON user_sessions;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- 删除触发器函数
DROP FUNCTION IF EXISTS update_updated_at_column();

-- 删除表（按依赖关系逆序删除）
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS alert_rules;
DROP TABLE IF EXISTS transaction_logs;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS users;
