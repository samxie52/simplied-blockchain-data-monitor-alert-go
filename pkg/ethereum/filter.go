package ethereum

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// FilterType represents different types of filters
type FilterType string

const (
	FilterTypeAddress     FilterType = "address"
	FilterTypeValue       FilterType = "value"
	FilterTypeGasPrice    FilterType = "gasPrice"
	FilterTypeGasUsed     FilterType = "gasUsed"
	FilterTypeBlockNumber FilterType = "blockNumber"
	FilterTypeTopics      FilterType = "topics"
	FilterTypeMethod      FilterType = "method"
	FilterTypeContract    FilterType = "contract"
	FilterTypeCustom      FilterType = "custom"
)

// FilterOperator represents filter comparison operators
type FilterOperator string

const (
	FilterOpEqual              FilterOperator = "eq"
	FilterOpNotEqual           FilterOperator = "ne"
	FilterOpGreaterThan        FilterOperator = "gt"
	FilterOpGreaterThanOrEqual FilterOperator = "gte"
	FilterOpLessThan           FilterOperator = "lt"
	FilterOpLessThanOrEqual    FilterOperator = "lte"
	FilterOpContains           FilterOperator = "contains"
	FilterOpStartsWith         FilterOperator = "startsWith"
	FilterOpEndsWith           FilterOperator = "endsWith"
	FilterOpRegex              FilterOperator = "regex"
	FilterOpIn                 FilterOperator = "in"
	FilterOpNotIn              FilterOperator = "notIn"
)

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Type     FilterType     `json:"type"`
	Operator FilterOperator `json:"operator"`
	Value    interface{}    `json:"value"`
	Field    string         `json:"field,omitempty"` // For custom filters
}

// FilterRule represents a complete filter rule with multiple conditions
type FilterRule struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Conditions  []*FilterCondition `json:"conditions"`
	Logic       string             `json:"logic"` // "AND" or "OR"
	Enabled     bool               `json:"enabled"`
	Priority    int                `json:"priority"`
}

// EventFilter manages filtering of blockchain events
type EventFilter struct {
	rules  map[string]*FilterRule
	logger *logrus.Entry
}

// NewEventFilter creates a new event filter
func NewEventFilter() *EventFilter {
	return &EventFilter{
		rules:  make(map[string]*FilterRule),
		logger: logrus.WithField("component", "event_filter"),
	}
}

// AddRule adds a new filter rule
func (ef *EventFilter) AddRule(rule *FilterRule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}
	
	if err := ef.validateRule(rule); err != nil {
		return fmt.Errorf("invalid rule: %v", err)
	}
	
	ef.rules[rule.ID] = rule
	ef.logger.WithField("rule_id", rule.ID).Info("Filter rule added")
	
	return nil
}

// RemoveRule removes a filter rule
func (ef *EventFilter) RemoveRule(ruleID string) error {
	if _, exists := ef.rules[ruleID]; !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}
	
	delete(ef.rules, ruleID)
	ef.logger.WithField("rule_id", ruleID).Info("Filter rule removed")
	
	return nil
}

// GetRule returns a filter rule by ID
func (ef *EventFilter) GetRule(ruleID string) (*FilterRule, bool) {
	rule, exists := ef.rules[ruleID]
	return rule, exists
}

// GetAllRules returns all filter rules
func (ef *EventFilter) GetAllRules() map[string]*FilterRule {
	result := make(map[string]*FilterRule)
	for id, rule := range ef.rules {
		result[id] = rule
	}
	return result
}

// FilterBlock filters a block based on all active rules
func (ef *EventFilter) FilterBlock(header *types.Header) []*FilterMatch {
	var matches []*FilterMatch
	
	for _, rule := range ef.rules {
		if !rule.Enabled {
			continue
		}
		
		if ef.matchesBlockRule(header, rule) {
			matches = append(matches, &FilterMatch{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				EventType: "block",
				Data:      header,
				Priority:  rule.Priority,
			})
		}
	}
	
	return matches
}

// FilterTransaction filters a transaction based on all active rules
func (ef *EventFilter) FilterTransaction(tx *types.Transaction) []*FilterMatch {
	var matches []*FilterMatch
	
	for _, rule := range ef.rules {
		if !rule.Enabled {
			continue
		}
		
		if ef.matchesTransactionRule(tx, rule) {
			matches = append(matches, &FilterMatch{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				EventType: "transaction",
				Data:      tx,
				Priority:  rule.Priority,
			})
		}
	}
	
	return matches
}

// FilterLog filters a log entry based on all active rules
func (ef *EventFilter) FilterLog(log *types.Log) []*FilterMatch {
	var matches []*FilterMatch
	
	for _, rule := range ef.rules {
		if !rule.Enabled {
			continue
		}
		
		if ef.matchesLogRule(log, rule) {
			matches = append(matches, &FilterMatch{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				EventType: "log",
				Data:      log,
				Priority:  rule.Priority,
			})
		}
	}
	
	return matches
}

// FilterMatch represents a filter match result
type FilterMatch struct {
	RuleID    string      `json:"rule_id"`
	RuleName  string      `json:"rule_name"`
	EventType string      `json:"event_type"`
	Data      interface{} `json:"data"`
	Priority  int         `json:"priority"`
	Timestamp int64       `json:"timestamp"`
}

// validateRule validates a filter rule
func (ef *EventFilter) validateRule(rule *FilterRule) error {
	if len(rule.Conditions) == 0 {
		return fmt.Errorf("rule must have at least one condition")
	}
	
	if rule.Logic != "AND" && rule.Logic != "OR" {
		return fmt.Errorf("logic must be 'AND' or 'OR'")
	}
	
	for i, condition := range rule.Conditions {
		if err := ef.validateCondition(condition); err != nil {
			return fmt.Errorf("condition %d: %v", i, err)
		}
	}
	
	return nil
}

// validateCondition validates a filter condition
func (ef *EventFilter) validateCondition(condition *FilterCondition) error {
	if condition.Type == "" {
		return fmt.Errorf("condition type cannot be empty")
	}
	
	if condition.Operator == "" {
		return fmt.Errorf("condition operator cannot be empty")
	}
	
	if condition.Value == nil {
		return fmt.Errorf("condition value cannot be nil")
	}
	
	// Validate operator compatibility with type
	switch condition.Type {
	case FilterTypeAddress, FilterTypeContract:
		if !ef.isStringOperator(condition.Operator) {
			return fmt.Errorf("invalid operator for address/contract type: %s", condition.Operator)
		}
	case FilterTypeValue, FilterTypeGasPrice, FilterTypeGasUsed, FilterTypeBlockNumber:
		if !ef.isNumericOperator(condition.Operator) {
			return fmt.Errorf("invalid operator for numeric type: %s", condition.Operator)
		}
	case FilterTypeTopics:
		if condition.Operator != FilterOpContains && condition.Operator != FilterOpEqual {
			return fmt.Errorf("invalid operator for topics type: %s", condition.Operator)
		}
	}
	
	return nil
}

// isStringOperator checks if an operator is valid for string types
func (ef *EventFilter) isStringOperator(op FilterOperator) bool {
	switch op {
	case FilterOpEqual, FilterOpNotEqual, FilterOpContains, FilterOpStartsWith, FilterOpEndsWith, FilterOpRegex, FilterOpIn, FilterOpNotIn:
		return true
	default:
		return false
	}
}

// isNumericOperator checks if an operator is valid for numeric types
func (ef *EventFilter) isNumericOperator(op FilterOperator) bool {
	switch op {
	case FilterOpEqual, FilterOpNotEqual, FilterOpGreaterThan, FilterOpGreaterThanOrEqual, FilterOpLessThan, FilterOpLessThanOrEqual, FilterOpIn, FilterOpNotIn:
		return true
	default:
		return false
	}
}

// matchesBlockRule checks if a block matches a rule
func (ef *EventFilter) matchesBlockRule(header *types.Header, rule *FilterRule) bool {
	results := make([]bool, len(rule.Conditions))
	
	for i, condition := range rule.Conditions {
		results[i] = ef.matchesBlockCondition(header, condition)
	}
	
	return ef.evaluateLogic(results, rule.Logic)
}

// matchesTransactionRule checks if a transaction matches a rule
func (ef *EventFilter) matchesTransactionRule(tx *types.Transaction, rule *FilterRule) bool {
	results := make([]bool, len(rule.Conditions))
	
	for i, condition := range rule.Conditions {
		results[i] = ef.matchesTransactionCondition(tx, condition)
	}
	
	return ef.evaluateLogic(results, rule.Logic)
}

// matchesLogRule checks if a log matches a rule
func (ef *EventFilter) matchesLogRule(log *types.Log, rule *FilterRule) bool {
	results := make([]bool, len(rule.Conditions))
	
	for i, condition := range rule.Conditions {
		results[i] = ef.matchesLogCondition(log, condition)
	}
	
	return ef.evaluateLogic(results, rule.Logic)
}

// matchesBlockCondition checks if a block matches a specific condition
func (ef *EventFilter) matchesBlockCondition(header *types.Header, condition *FilterCondition) bool {
	switch condition.Type {
	case FilterTypeBlockNumber:
		return ef.compareNumeric(header.Number, condition.Operator, condition.Value)
	case FilterTypeAddress:
		return ef.compareString(header.Coinbase.Hex(), condition.Operator, condition.Value)
	case FilterTypeGasUsed:
		return ef.compareNumeric(new(big.Int).SetUint64(header.GasUsed), condition.Operator, condition.Value)
	default:
		ef.logger.WithField("type", condition.Type).Warn("Unsupported condition type for block")
		return false
	}
}

// matchesTransactionCondition checks if a transaction matches a specific condition
func (ef *EventFilter) matchesTransactionCondition(tx *types.Transaction, condition *FilterCondition) bool {
	switch condition.Type {
	case FilterTypeAddress:
		if tx.To() != nil {
			return ef.compareString(tx.To().Hex(), condition.Operator, condition.Value)
		}
		return false
	case FilterTypeValue:
		return ef.compareNumeric(tx.Value(), condition.Operator, condition.Value)
	case FilterTypeGasPrice:
		return ef.compareNumeric(tx.GasPrice(), condition.Operator, condition.Value)
	case FilterTypeGasUsed:
		return ef.compareNumeric(new(big.Int).SetUint64(tx.Gas()), condition.Operator, condition.Value)
	case FilterTypeMethod:
		if len(tx.Data()) >= 4 {
			methodID := fmt.Sprintf("0x%x", tx.Data()[:4])
			return ef.compareString(methodID, condition.Operator, condition.Value)
		}
		return false
	case FilterTypeContract:
		if tx.To() == nil {
			// Contract creation
			return condition.Operator == FilterOpEqual && condition.Value == "creation"
		}
		return ef.compareString(tx.To().Hex(), condition.Operator, condition.Value)
	default:
		ef.logger.WithField("type", condition.Type).Warn("Unsupported condition type for transaction")
		return false
	}
}

// matchesLogCondition checks if a log matches a specific condition
func (ef *EventFilter) matchesLogCondition(log *types.Log, condition *FilterCondition) bool {
	switch condition.Type {
	case FilterTypeAddress, FilterTypeContract:
		return ef.compareString(log.Address.Hex(), condition.Operator, condition.Value)
	case FilterTypeTopics:
		return ef.matchesTopics(log.Topics, condition)
	case FilterTypeBlockNumber:
		return ef.compareNumeric(new(big.Int).SetUint64(log.BlockNumber), condition.Operator, condition.Value)
	default:
		ef.logger.WithField("type", condition.Type).Warn("Unsupported condition type for log")
		return false
	}
}

// matchesTopics checks if log topics match the condition
func (ef *EventFilter) matchesTopics(topics []common.Hash, condition *FilterCondition) bool {
	switch condition.Operator {
	case FilterOpContains:
		targetTopic := condition.Value.(string)
		for _, topic := range topics {
			if strings.EqualFold(topic.Hex(), targetTopic) {
				return true
			}
		}
		return false
	case FilterOpEqual:
		expectedTopics, ok := condition.Value.([]string)
		if !ok {
			return false
		}
		
		if len(topics) != len(expectedTopics) {
			return false
		}
		
		for i, topic := range topics {
			if !strings.EqualFold(topic.Hex(), expectedTopics[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// compareString compares string values based on the operator
func (ef *EventFilter) compareString(actual string, operator FilterOperator, expected interface{}) bool {
	expectedStr := fmt.Sprintf("%v", expected)
	
	switch operator {
	case FilterOpEqual:
		return strings.EqualFold(actual, expectedStr)
	case FilterOpNotEqual:
		return !strings.EqualFold(actual, expectedStr)
	case FilterOpContains:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(expectedStr))
	case FilterOpStartsWith:
		return strings.HasPrefix(strings.ToLower(actual), strings.ToLower(expectedStr))
	case FilterOpEndsWith:
		return strings.HasSuffix(strings.ToLower(actual), strings.ToLower(expectedStr))
	case FilterOpRegex:
		matched, err := regexp.MatchString(expectedStr, actual)
		if err != nil {
			ef.logger.WithError(err).Warn("Invalid regex pattern")
			return false
		}
		return matched
	case FilterOpIn:
		expectedList, ok := expected.([]interface{})
		if !ok {
			return false
		}
		for _, item := range expectedList {
			if strings.EqualFold(actual, fmt.Sprintf("%v", item)) {
				return true
			}
		}
		return false
	case FilterOpNotIn:
		expectedList, ok := expected.([]interface{})
		if !ok {
			return true
		}
		for _, item := range expectedList {
			if strings.EqualFold(actual, fmt.Sprintf("%v", item)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// compareNumeric compares numeric values based on the operator
func (ef *EventFilter) compareNumeric(actual *big.Int, operator FilterOperator, expected interface{}) bool {
	var expectedBig *big.Int
	
	switch v := expected.(type) {
	case string:
		var ok bool
		expectedBig, ok = new(big.Int).SetString(v, 0)
		if !ok {
			ef.logger.WithField("value", v).Warn("Invalid numeric string")
			return false
		}
	case int64:
		expectedBig = big.NewInt(v)
	case uint64:
		expectedBig = new(big.Int).SetUint64(v)
	case *big.Int:
		expectedBig = v
	case float64:
		expectedBig = big.NewInt(int64(v))
	default:
		ef.logger.WithField("type", fmt.Sprintf("%T", expected)).Warn("Unsupported numeric type")
		return false
	}
	
	cmp := actual.Cmp(expectedBig)
	
	switch operator {
	case FilterOpEqual:
		return cmp == 0
	case FilterOpNotEqual:
		return cmp != 0
	case FilterOpGreaterThan:
		return cmp > 0
	case FilterOpGreaterThanOrEqual:
		return cmp >= 0
	case FilterOpLessThan:
		return cmp < 0
	case FilterOpLessThanOrEqual:
		return cmp <= 0
	case FilterOpIn:
		expectedList, ok := expected.([]interface{})
		if !ok {
			return false
		}
		for _, item := range expectedList {
			if ef.compareNumeric(actual, FilterOpEqual, item) {
				return true
			}
		}
		return false
	case FilterOpNotIn:
		expectedList, ok := expected.([]interface{})
		if !ok {
			return true
		}
		for _, item := range expectedList {
			if ef.compareNumeric(actual, FilterOpEqual, item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// evaluateLogic evaluates the logical combination of condition results
func (ef *EventFilter) evaluateLogic(results []bool, logic string) bool {
	if len(results) == 0 {
		return false
	}
	
	switch logic {
	case "AND":
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	case "OR":
		for _, result := range results {
			if result {
				return true
			}
		}
		return false
	default:
		ef.logger.WithField("logic", logic).Warn("Unknown logic operator")
		return false
	}
}

// GetStats returns filter statistics
func (ef *EventFilter) GetStats() map[string]interface{} {
	enabledRules := 0
	totalConditions := 0
	
	for _, rule := range ef.rules {
		if rule.Enabled {
			enabledRules++
		}
		totalConditions += len(rule.Conditions)
	}
	
	return map[string]interface{}{
		"total_rules":       len(ef.rules),
		"enabled_rules":     enabledRules,
		"total_conditions":  totalConditions,
		"rules_by_priority": ef.getRulesByPriority(),
	}
}

// getRulesByPriority returns rules grouped by priority
func (ef *EventFilter) getRulesByPriority() map[int]int {
	priorityCount := make(map[int]int)
	
	for _, rule := range ef.rules {
		priorityCount[rule.Priority]++
	}
	
	return priorityCount
}

// ExportRules exports all rules to JSON
func (ef *EventFilter) ExportRules() ([]byte, error) {
	return json.MarshalIndent(ef.rules, "", "  ")
}

// ImportRules imports rules from JSON
func (ef *EventFilter) ImportRules(data []byte) error {
	var rules map[string]*FilterRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("failed to unmarshal rules: %v", err)
	}
	
	// Validate all rules before importing
	for _, rule := range rules {
		if err := ef.validateRule(rule); err != nil {
			return fmt.Errorf("invalid rule %s: %v", rule.ID, err)
		}
	}
	
	// Clear existing rules and import new ones
	ef.rules = rules
	ef.logger.WithField("count", len(rules)).Info("Rules imported successfully")
	
	return nil
}
