-- 创建用户表
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    full_name VARCHAR(100),
    avatar VARCHAR(500),
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    phone VARCHAR(20),
    telegram_id VARCHAR(50),
    preferences TEXT,
    timezone VARCHAR(50) DEFAULT 'UTC',
    language VARCHAR(10) DEFAULT 'en',
    last_login_at TIMESTAMP WITH TIME ZONE,
    last_login_ip VARCHAR(45),
    failed_login_count INTEGER DEFAULT 0,
    locked_until TIMESTAMP WITH TIME ZONE,
    email_verified BOOLEAN DEFAULT FALSE,
    email_verified_at TIMESTAMP WITH TIME ZONE,
    api_key VARCHAR(64) UNIQUE,
    api_key_created_at TIMESTAMP WITH TIME ZONE
);

-- 创建用户会话表
CREATE TABLE IF NOT EXISTS user_sessions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL,
    session_id VARCHAR(128) UNIQUE NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_active BOOLEAN DEFAULT TRUE
);

-- 创建区块表
CREATE TABLE IF NOT EXISTS blocks (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    number BIGINT UNIQUE NOT NULL,
    hash VARCHAR(66) UNIQUE NOT NULL,
    parent_hash VARCHAR(66) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    miner VARCHAR(42) NOT NULL,
    difficulty VARCHAR(78) NOT NULL,
    total_difficulty VARCHAR(78),
    size BIGINT DEFAULT 0,
    gas_limit BIGINT NOT NULL,
    gas_used BIGINT NOT NULL,
    transaction_count INTEGER DEFAULT 0,
    state_root VARCHAR(66),
    receipts_root VARCHAR(66),
    transactions_root VARCHAR(66),
    extra_data TEXT,
    mix_hash VARCHAR(66),
    nonce VARCHAR(18),
    logs_bloom TEXT,
    base_fee_per_gas VARCHAR(78)
);

-- 创建交易表
CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    hash VARCHAR(66) UNIQUE NOT NULL,
    block_number BIGINT NOT NULL,
    block_hash VARCHAR(66),
    transaction_index INTEGER DEFAULT 0,
    from_address VARCHAR(42) NOT NULL,
    to_address VARCHAR(42),
    value VARCHAR(78) NOT NULL,
    input TEXT,
    gas BIGINT NOT NULL,
    gas_used BIGINT,
    gas_price VARCHAR(78),
    max_fee_per_gas VARCHAR(78),
    max_priority_fee_per_gas VARCHAR(78),
    type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    nonce BIGINT DEFAULT 0,
    v VARCHAR(10),
    r VARCHAR(66),
    s VARCHAR(66),
    cumulative_gas_used BIGINT,
    effective_gas_price VARCHAR(78),
    contract_address VARCHAR(42),
    logs_count INTEGER DEFAULT 0,
    logs_bloom TEXT,
    timestamp TIMESTAMP WITH TIME ZONE
);

-- 创建交易日志表
CREATE TABLE IF NOT EXISTS transaction_logs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    transaction_hash VARCHAR(66) NOT NULL,
    log_index INTEGER NOT NULL,
    address VARCHAR(42) NOT NULL,
    topics TEXT,
    data TEXT,
    block_number BIGINT NOT NULL,
    removed BOOLEAN DEFAULT FALSE
);

-- 创建告警规则表
CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    conditions TEXT NOT NULL,
    threshold DECIMAL(20,8) DEFAULT 0,
    operator VARCHAR(10) NOT NULL,
    time_window INTEGER DEFAULT 60,
    cooldown INTEGER DEFAULT 300,
    notification_channels TEXT,
    notification_template TEXT,
    user_id BIGINT NOT NULL,
    trigger_count BIGINT DEFAULT 0,
    last_triggered TIMESTAMP WITH TIME ZONE,
    last_checked TIMESTAMP WITH TIME ZONE
);

-- 创建告警记录表
CREATE TABLE IF NOT EXISTS alerts (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    rule_id BIGINT NOT NULL,
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    trigger_value DECIMAL(20,8) DEFAULT 0,
    trigger_data TEXT,
    trigger_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending',
    notification_sent BOOLEAN DEFAULT FALSE,
    sent_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0
);

-- 创建订阅表
CREATE TABLE IF NOT EXISTS subscriptions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    config TEXT NOT NULL,
    filters TEXT,
    notification_channels TEXT,
    notification_template TEXT,
    max_notifications_per_hour INTEGER DEFAULT 100,
    notification_count INTEGER DEFAULT 0,
    last_notification_reset TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    total_notifications BIGINT DEFAULT 0,
    last_triggered TIMESTAMP WITH TIME ZONE,
    last_checked TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE
);

-- 创建索引

-- 用户表索引
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

-- 用户会话表索引
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_session_id ON user_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_sessions_is_active ON user_sessions(is_active);

-- 区块表索引
CREATE INDEX IF NOT EXISTS idx_blocks_number ON blocks(number);
CREATE INDEX IF NOT EXISTS idx_blocks_hash ON blocks(hash);
CREATE INDEX IF NOT EXISTS idx_blocks_timestamp ON blocks(timestamp);
CREATE INDEX IF NOT EXISTS idx_blocks_miner ON blocks(miner);
CREATE INDEX IF NOT EXISTS idx_blocks_gas_used ON blocks(gas_used);
CREATE INDEX IF NOT EXISTS idx_blocks_transaction_count ON blocks(transaction_count);
CREATE INDEX IF NOT EXISTS idx_blocks_created_at ON blocks(created_at);

-- 交易表索引
CREATE INDEX IF NOT EXISTS idx_transactions_hash ON transactions(hash);
CREATE INDEX IF NOT EXISTS idx_transactions_block_number ON transactions(block_number);
CREATE INDEX IF NOT EXISTS idx_transactions_from_address ON transactions(from_address);
CREATE INDEX IF NOT EXISTS idx_transactions_to_address ON transactions(to_address);
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_value ON transactions(value);
CREATE INDEX IF NOT EXISTS idx_transactions_gas_used ON transactions(gas_used);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);

-- 复合索引
CREATE INDEX IF NOT EXISTS idx_transactions_block_index ON transactions(block_number, transaction_index);
CREATE INDEX IF NOT EXISTS idx_transactions_from_to ON transactions(from_address, to_address);

-- 交易日志表索引
CREATE INDEX IF NOT EXISTS idx_transaction_logs_transaction_hash ON transaction_logs(transaction_hash);
CREATE INDEX IF NOT EXISTS idx_transaction_logs_address ON transaction_logs(address);
CREATE INDEX IF NOT EXISTS idx_transaction_logs_block_number ON transaction_logs(block_number);
CREATE INDEX IF NOT EXISTS idx_transaction_logs_log_index ON transaction_logs(log_index);

-- 告警规则表索引
CREATE INDEX IF NOT EXISTS idx_alert_rules_user_id ON alert_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_alert_rules_type ON alert_rules(type);
CREATE INDEX IF NOT EXISTS idx_alert_rules_severity ON alert_rules(severity);
CREATE INDEX IF NOT EXISTS idx_alert_rules_status ON alert_rules(status);
CREATE INDEX IF NOT EXISTS idx_alert_rules_last_triggered ON alert_rules(last_triggered);
CREATE INDEX IF NOT EXISTS idx_alert_rules_created_at ON alert_rules(created_at);

-- 告警记录表索引
CREATE INDEX IF NOT EXISTS idx_alerts_rule_id ON alerts(rule_id);
CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(type);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_trigger_time ON alerts(trigger_time);
CREATE INDEX IF NOT EXISTS idx_alerts_notification_sent ON alerts(notification_sent);
CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON alerts(created_at);

-- 订阅表索引
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_type ON subscriptions(type);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_expires_at ON subscriptions(expires_at);
CREATE INDEX IF NOT EXISTS idx_subscriptions_last_triggered ON subscriptions(last_triggered);
CREATE INDEX IF NOT EXISTS idx_subscriptions_created_at ON subscriptions(created_at);

-- 创建触发器函数，用于自动更新 updated_at 字段
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为所有表创建更新时间触发器
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_sessions_updated_at BEFORE UPDATE ON user_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_blocks_updated_at BEFORE UPDATE ON blocks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_transactions_updated_at BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_transaction_logs_updated_at BEFORE UPDATE ON transaction_logs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_alert_rules_updated_at BEFORE UPDATE ON alert_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_alerts_updated_at BEFORE UPDATE ON alerts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_subscriptions_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 创建视图

-- 区块统计视图
CREATE OR REPLACE VIEW block_statistics AS
SELECT 
    number,
    hash,
    timestamp,
    miner,
    gas_limit,
    gas_used,
    ROUND((gas_used::DECIMAL / gas_limit::DECIMAL) * 100, 2) AS gas_utilization_percentage,
    transaction_count,
    size,
    EXTRACT(EPOCH FROM (timestamp - LAG(timestamp) OVER (ORDER BY number))) AS block_time_seconds
FROM blocks
ORDER BY number DESC;

-- 交易统计视图
CREATE OR REPLACE VIEW transaction_statistics AS
SELECT 
    DATE_TRUNC('hour', timestamp) AS hour,
    COUNT(*) AS transaction_count,
    SUM(gas_used) AS total_gas_used,
    AVG(gas_used) AS avg_gas_used,
    COUNT(CASE WHEN status = 'success' THEN 1 END) AS successful_transactions,
    COUNT(CASE WHEN status = 'failed' THEN 1 END) AS failed_transactions,
    COUNT(CASE WHEN to_address IS NULL THEN 1 END) AS contract_creations
FROM transactions
WHERE timestamp >= CURRENT_TIMESTAMP - INTERVAL '24 hours'
GROUP BY DATE_TRUNC('hour', timestamp)
ORDER BY hour DESC;

-- 用户活动统计视图
CREATE OR REPLACE VIEW user_activity_statistics AS
SELECT 
    u.id,
    u.username,
    u.email,
    u.role,
    u.status,
    u.created_at,
    u.last_login_at,
    COUNT(ar.id) AS alert_rules_count,
    COUNT(s.id) AS subscriptions_count,
    COUNT(CASE WHEN ar.status = 'active' THEN 1 END) AS active_alert_rules,
    COUNT(CASE WHEN s.status = 'active' THEN 1 END) AS active_subscriptions
FROM users u
LEFT JOIN alert_rules ar ON u.id = ar.user_id
LEFT JOIN subscriptions s ON u.id = s.user_id
GROUP BY u.id, u.username, u.email, u.role, u.status, u.created_at, u.last_login_at;

-- 告警统计视图
CREATE OR REPLACE VIEW alert_statistics AS
SELECT 
    DATE_TRUNC('day', trigger_time) AS day,
    type,
    severity,
    COUNT(*) AS alert_count,
    COUNT(CASE WHEN notification_sent = true THEN 1 END) AS notifications_sent,
    COUNT(CASE WHEN status = 'failed' THEN 1 END) AS failed_alerts,
    AVG(retry_count) AS avg_retry_count
FROM alerts
WHERE trigger_time >= CURRENT_TIMESTAMP - INTERVAL '30 days'
GROUP BY DATE_TRUNC('day', trigger_time), type, severity
ORDER BY day DESC, type, severity;

-- 添加注释
COMMENT ON TABLE users IS '用户表，存储系统用户信息';
COMMENT ON TABLE user_sessions IS '用户会话表，存储用户登录会话信息';
COMMENT ON TABLE blocks IS '区块表，存储以太坊区块数据';
COMMENT ON TABLE transactions IS '交易表，存储以太坊交易数据';
COMMENT ON TABLE transaction_logs IS '交易日志表，存储交易事件日志';
COMMENT ON TABLE alert_rules IS '告警规则表，存储用户定义的告警规则';
COMMENT ON TABLE alerts IS '告警记录表，存储触发的告警信息';
COMMENT ON TABLE subscriptions IS '订阅表，存储用户订阅的监控项目';

COMMENT ON COLUMN users.api_key IS 'API访问密钥，用于API认证';
COMMENT ON COLUMN users.telegram_id IS 'Telegram用户ID，用于发送告警消息';
COMMENT ON COLUMN users.preferences IS 'JSON格式的用户偏好设置';
COMMENT ON COLUMN users.failed_login_count IS '连续失败登录次数，用于账户锁定';

COMMENT ON COLUMN blocks.difficulty IS '区块难度，十六进制字符串格式';
COMMENT ON COLUMN blocks.total_difficulty IS '总难度，十六进制字符串格式';
COMMENT ON COLUMN blocks.base_fee_per_gas IS 'EIP-1559基础费用，Wei单位';

COMMENT ON COLUMN transactions.value IS '交易金额，Wei单位，字符串格式';
COMMENT ON COLUMN transactions.gas_price IS '传统交易的Gas价格，Wei单位';
COMMENT ON COLUMN transactions.max_fee_per_gas IS 'EIP-1559最大费用，Wei单位';
COMMENT ON COLUMN transactions.max_priority_fee_per_gas IS 'EIP-1559最大优先费用，Wei单位';

COMMENT ON COLUMN alert_rules.conditions IS 'JSON格式的告警条件配置';
COMMENT ON COLUMN alert_rules.notification_channels IS 'JSON格式的通知渠道配置';
COMMENT ON COLUMN alert_rules.time_window IS '时间窗口，秒为单位';
COMMENT ON COLUMN alert_rules.cooldown IS '冷却时间，秒为单位';

COMMENT ON COLUMN subscriptions.config IS 'JSON格式的订阅配置参数';
