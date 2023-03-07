package networking

import (
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

// AddToRuleTable
func AddToRuleTable(preInterfaceIPAddress []netlink.Addr, ruleTable int) error {
	for _, ipAddress := range preInterfaceIPAddress {
		rule := netlink.NewRule()
		rule.Table = ruleTable
		rule.Dst = ipAddress.IPNet
		if err := netlink.RuleAdd(rule); err != nil {
			return err
		}
	}
	return nil
}

// AddFromRuleTable add route rule for calico/cilium cidr(ipv4 and ipv6)
// Equivalent to: `ip rule add from <cidr> `
func AddFromRuleTable(logger *zap.Logger, ipAddrs []netlink.Addr, ruleTable int) error {
	logger.Debug("Add FromRule Table in Pod Netns")
	for _, ipAddr := range ipAddrs {
		rule := netlink.NewRule()
		rule.Table = ruleTable
		rule.Src = ipAddr.IPNet
		logger.Debug("Netlink RuleAdd", zap.String("Rule", rule.String()))
		if err := netlink.RuleAdd(rule); err != nil {
			logger.Error(err.Error())
			return err
		}
	}
	// we should add rule route table, just like `ip route add default via 169.254.1.1 table 100`
	// but we don't know what's the default route If it has been deleted.
	// so we should add this route rule table before removing the default route
	return nil
}
