package networking

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"net"
	"os"
)

func AddRouteTable(logger *zap.Logger, ruleTable int, scope netlink.Scope, device string, destinations []string, v4Gw, v6Gw net.IP) error {
	link, err := netlink.LinkByName(device)
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

		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Scope:     scope,
			Dst:       ipNet,
			Table:     ruleTable,
		}

		if ipNet.IP.To4() != nil && v4Gw != nil {
			route.Gw = v4Gw
		}

		if ipNet.IP.To4() == nil && v6Gw != nil {
			route.Gw = v6Gw
		}

		if err = netlink.RouteAdd(route); err != nil && !os.IsExist(err) {
			logger.Error("failed to RouteAdd", zap.String("route", route.String()), zap.Error(err))
			return err
		}
	}
	return nil
}

func GetGatewayIP(addrs []netlink.Addr) (v4Gw, v6Gw net.IP, err error) {
	for _, addr := range addrs {
		routes, err := netlink.RouteGet(addr.IP)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to RouteGet Pod IP(%s): %v", addr.IP.String(), err)
		}

		if len(routes) > 0 {
			if addr.IP.To4() != nil && v4Gw == nil {
				v4Gw = routes[0].Src
			}
			if addr.IP.To4() == nil && v6Gw == nil {
				v6Gw = routes[0].Src
			}
		}
	}
	return
}
