package networking

import (
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"net"
	"os"
)

func AddRouteTable(logger *zap.Logger, ruleTable int, iface string, destinations []string) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	for _, dst := range destinations {
		_, ipNet, err := net.ParseCIDR(dst)
		if err != nil {
			logger.Error(err.Error())
			return err
		}

		if err = netlink.RouteAdd(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Scope:     netlink.SCOPE_LINK,
			Dst:       ipNet,
			Table:     ruleTable,
		}); err != nil && !os.IsExist(err) {
			logger.Error("failed to add route", zap.String("interface", iface), zap.String("dst", ipNet.String()), zap.Error(err))
			return err
		}
	}
	return nil
}
