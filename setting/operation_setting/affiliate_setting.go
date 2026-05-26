package operation_setting

import (
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
	switch configKey {
	case "enabled":
		next.Enabled = strings.EqualFold(value, "true")
	case "first_reward_enabled":
		next.FirstRewardEnabled = strings.EqualFold(value, "true")
	case "recurring_reward_enabled":
		next.RecurringRewardEnabled = strings.EqualFold(value, "true")
	case "first_reward_rate":
		next.FirstRewardRate, _ = strconv.ParseFloat(strings.TrimSpace(value), 64)
	case "recurring_reward_rate":
		next.RecurringRewardRate, _ = strconv.ParseFloat(strings.TrimSpace(value), 64)
	}
	return AffiliateRewardRequiresCompliance(next)
}
