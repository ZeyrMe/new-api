package operation_setting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type AffiliateSetting struct {
	Enabled                   bool    `json:"enabled"`
	AttributionWindowDays     int     `json:"attribution_window_days"`
	MinTopupQuota             int     `json:"min_topup_quota"`
	SettlementDelayDays       int     `json:"settlement_delay_days"`
	FirstRewardEnabled        bool    `json:"first_reward_enabled"`
	FirstRewardRate           float64 `json:"first_reward_rate"`
	FirstRewardCapQuota       int     `json:"first_reward_cap_quota"`
	RecurringRewardEnabled    bool    `json:"recurring_reward_enabled"`
	RecurringRewardRate       float64 `json:"recurring_reward_rate"`
	RecurringWindowDays       int     `json:"recurring_window_days"`
	RecurringMaxCount         int     `json:"recurring_max_count"`
	RecurringCapPerInvitee    int     `json:"recurring_cap_per_invitee"`
	IncludeSubscriptionOrders bool    `json:"include_subscription_orders"`
}

var affiliateSetting = AffiliateSetting{
	Enabled:                   false,
	AttributionWindowDays:     30,
	MinTopupQuota:             0,
	SettlementDelayDays:       0,
	FirstRewardEnabled:        true,
	FirstRewardRate:           0.1,
	FirstRewardCapQuota:       0,
	RecurringRewardEnabled:    false,
	RecurringRewardRate:       0.02,
	RecurringWindowDays:       90,
	RecurringMaxCount:         0,
	RecurringCapPerInvitee:    0,
	IncludeSubscriptionOrders: true,
}

func init() {
	config.GlobalConfig.Register("affiliate_setting", &affiliateSetting)
}

func GetAffiliateSetting() *AffiliateSetting {
	return &affiliateSetting
}

func AffiliateRewardRequiresCompliance(s AffiliateSetting) bool {
	return s.Enabled &&
		((s.FirstRewardEnabled && s.FirstRewardRate > 0) ||
			(s.RecurringRewardEnabled && s.RecurringRewardRate > 0))
}

func WouldAffiliateSettingRequireCompliance(key string, value string) bool {
	const prefix = "affiliate_setting."
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	next := affiliateSetting
	configKey := strings.TrimPrefix(key, prefix)
	trimmedValue := strings.TrimSpace(value)
	switch configKey {
	case "enabled":
		next.Enabled, _ = strconv.ParseBool(trimmedValue)
	case "first_reward_enabled":
		next.FirstRewardEnabled, _ = strconv.ParseBool(trimmedValue)
	case "recurring_reward_enabled":
		next.RecurringRewardEnabled, _ = strconv.ParseBool(trimmedValue)
	case "first_reward_rate":
		next.FirstRewardRate, _ = strconv.ParseFloat(trimmedValue, 64)
	case "recurring_reward_rate":
		next.RecurringRewardRate, _ = strconv.ParseFloat(trimmedValue, 64)
	}
	return AffiliateRewardRequiresCompliance(next)
}

func ValidateAffiliateSettingOption(key string, value string) error {
	const prefix = "affiliate_setting."
	if !strings.HasPrefix(key, prefix) {
		return nil
	}
	configKey := strings.TrimPrefix(key, prefix)
	trimmedValue := strings.TrimSpace(value)

	switch configKey {
	case "enabled", "first_reward_enabled", "recurring_reward_enabled", "include_subscription_orders":
		if _, err := strconv.ParseBool(trimmedValue); err != nil {
			return fmt.Errorf("%s 必须是布尔值", key)
		}
	case "attribution_window_days", "min_topup_quota", "settlement_delay_days",
		"first_reward_cap_quota", "recurring_window_days", "recurring_max_count",
		"recurring_cap_per_invitee":
		intValue, err := strconv.Atoi(trimmedValue)
		if err != nil {
			return fmt.Errorf("%s 必须是整数", key)
		}
		if intValue < 0 {
			return fmt.Errorf("%s 不能为负数", key)
		}
	case "first_reward_rate", "recurring_reward_rate":
		floatValue, err := strconv.ParseFloat(trimmedValue, 64)
		if err != nil {
			return fmt.Errorf("%s 必须是数字", key)
		}
		if floatValue < 0 {
			return fmt.Errorf("%s 不能为负数", key)
		}
	default:
		return fmt.Errorf("未知的推荐返利配置项: %s", key)
	}
	return nil
}
