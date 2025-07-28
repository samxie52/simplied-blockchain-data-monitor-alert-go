package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator struct {
	// 验证器
	validator *validator.Validate
}

// NewValidator 创建配置验证器
func NewValidator() *Validator {
	v := validator.New()

	// 注册自定义验证规则
	v.RegisterValidation("oneof", validateOneOf)

	return &Validator{
		validator: v,
	}
}

// Validate 验证配置
func (v *Validator) Validate(cfg *Config) error {
	if err := v.validator.Struct(cfg); err != nil {
		return v.formatValidationError(err)
	}

	// 自定义验证逻辑
	return v.customValidation(cfg)
}

// formatValidationError 格式化验证错误
func (v *Validator) formatValidationError(err error) error {
	var errors []string

	for _, err := range err.(validator.ValidationErrors) {
		field := err.Field()
		tag := err.Tag()
		param := err.Param()

		var message string
		switch tag {
		case "required":
			message = fmt.Sprintf("%s is required", field)
		case "min":
			message = fmt.Sprintf("%s must be at least %s", field, param)
		case "max":
			message = fmt.Sprintf("%s must be at most %s", field, param)
		case "oneof":
			message = fmt.Sprintf("%s must be one of: %s", field, param)
		case "url":
			message = fmt.Sprintf("%s must be a valid URL", field)
		default:
			message = fmt.Sprintf("%s validation failed for tag '%s'", field, tag)
		}

		errors = append(errors, message)
	}

	return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, "; "))
}

// customValidation 自定义验证逻辑
func (v *Validator) customValidation(cfg *Config) error {
	// 验证数据库连接池配置
	if cfg.Database.MaxIdleConns > cfg.Database.MaxOpenConns {
		return fmt.Errorf("database max_idle_conns cannot be greater than max_open_conns")
	}

	// 验证以太坊网络和链ID匹配
	if err := v.validateEthereumConfig(&cfg.Ethereum); err != nil {
		return err
	}

	// 验证日志配置
	if cfg.Logging.Output == "file" && cfg.Logging.FilePath == "" {
		return fmt.Errorf("log_file_path is required when log_output is 'file'")
	}

	// 验证安全配置
	if len(cfg.Security.JWTSecret) < 32 {
		return fmt.Errorf("jwt_secret must be at least 32 characters long")
	}

	return nil
}

// validateEthereumConfig 验证以太坊配置
func (v *Validator) validateEthereumConfig(cfg *EthereumConfig) error {
	networkChainMap := map[string]int64{
		"mainnet": 1,
		"goerli":  5,
		"sepolia": 11155111,
	}

	expectedChainID, exists := networkChainMap[cfg.Network]
	if !exists {
		return fmt.Errorf("unsupported ethereum network: %s", cfg.Network)
	}

	if cfg.ChainID != expectedChainID {
		return fmt.Errorf("chain_id %d does not match network %s (expected %d)",
			cfg.ChainID, cfg.Network, expectedChainID)
	}

	return nil
}

// validateOneOf 自定义oneof验证器
func validateOneOf(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	param := fl.Param()

	options := strings.Split(param, " ")
	for _, option := range options {
		if value == option {
			return true
		}
	}

	return false
}
