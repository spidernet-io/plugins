package networking

import (
	"fmt"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/spidernet-io/plugins/pkg/types"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"regexp"
	"strings"
)

var DefaultInterfacesToExclude = []string{
	"docker.*", "cbr.*", "dummy.*",
	"virbr.*", "lxcbr.*", "veth.*", "lo",
	"cali.*", "tunl.*", "flannel.*", "kube-ipvs.*", "cni.*",
}

func GetIPFamily(prevResult cnitypes.Result) (int, error) {
	result, err := current.GetResult(prevResult)
	if err != nil {
		return netlink.FAMILY_ALL, fmt.Errorf("failed to convert prevResult: %v", err)
	}

	if len(result.Interfaces) == 0 {
		return netlink.FAMILY_ALL, fmt.Errorf("can't found any interface from prevResult")
	}

	ipFamily := netlink.FAMILY_V4
	enableIpv4, enableIpv6 := false, false
	for _, v := range result.IPs {
		if v.Address.IP.To4() != nil {
			enableIpv4 = true
		} else {
			enableIpv6 = true
			ipFamily = netlink.FAMILY_V6
		}
	}

	if enableIpv4 && enableIpv6 {
		return netlink.FAMILY_ALL, nil
	}

	return ipFamily, nil
}

// IPAddressByName returns all IP addresses of the given interface
// group by ipFamily
func IPAddressByName(netns ns.NetNS, interfacenName string, ipFamily int) ([]netlink.Addr, error) {
	var err error
	ipAddress := make([]netlink.Addr, 0, 2)
	err = netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(interfacenName)
		if err != nil {
			return err
		}
		ipAddress, err = getAddrs(link, ipFamily)
		return err
	})

	if err != nil {
		return nil, err
	}
	return ipAddress, nil
}

// IPAddressOnNode return all ip addresses on the node, filter by ipFamily
// skipping any interfaces whose name matches any of the exclusion list regexes
func IPAddressOnNode(logger *zap.Logger, ipFamily int) ([]netlink.Addr, error) {
	var err error
	var excludeRegexp *regexp.Regexp
	if excludeRegexp, err = regexp.Compile("(" + strings.Join(DefaultInterfacesToExclude, ")|(") + ")"); err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	links, err := netlink.LinkList()
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	var ipAddress []netlink.Addr
	for _, link := range links {
		exclude := (excludeRegexp != nil) && excludeRegexp.MatchString(link.Attrs().Name)
		if exclude {
			continue
		}

		ipAddress, err = getAddrs(link, ipFamily)
		if err != nil {
			logger.Error(err.Error())
			return nil, err
		}
	}
	logger.Debug("Get IPAddressOnNode", zap.Any("IPAddress", ipAddress))
	return ipAddress, nil
}

func getAddrs(link netlink.Link, ipfamily int) ([]netlink.Addr, error) {
	var ipAddress []netlink.Addr
	addrs, err := netlink.AddrList(link, ipfamily)
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if addr.IP.IsMulticast() || addr.IP.IsLinkLocalUnicast() {
			continue
		}
		if addr.IP.To4() != nil && (ipfamily == netlink.FAMILY_V4 || ipfamily == netlink.FAMILY_ALL) {
			ipAddress = append(ipAddress, addr)
		}
		if addr.IP.To4() == nil && (ipfamily == netlink.FAMILY_V6 || ipfamily == netlink.FAMILY_ALL) {
			ipAddress = append(ipAddress, addr)
		}
	}
	return ipAddress, nil
}

// AddrsToString convert addr to
func AddrsToString(addrs []netlink.Addr) []string {
	addrStrings := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IP.To4() != nil {
			addr.IPNet.Mask = net.CIDRMask(32, 32)
		} else {
			addr.IPNet.Mask = net.CIDRMask(128, 128)
		}
		addrStrings = append(addrStrings, addr.String())
	}
	return addrStrings
}

func EnableIpv6Sysctl(logger *zap.Logger, netns ns.NetNS) error {
	logger.Debug("Setting all interface sysctl 'disable_ipv6' to 0 ", zap.String("NetNs Path", netns.Path()))
	err := netns.Do(func(_ ns.NetNS) error {
		dirs, err := os.ReadDir("/proc/sys/net/ipv6/conf")
		if err != nil {
			logger.Error(err.Error())
			return err
		}

		for _, dir := range dirs {
			// Read current sysctl value
			name := fmt.Sprintf("/net/ipv6/conf/%s/disable_ipv6", dir.Name())
			value, err := sysctl.Sysctl(name)
			if err != nil {
				logger.Error("failed to read current sysctl value", zap.String("name", name), zap.Error(err))
				return fmt.Errorf("failed to read current sysctl %+v value: %v", name, err)
			}
			// make sure value=0
			if value != "0" {
				if _, err = sysctl.Sysctl(name, "0"); err != nil {
					logger.Error("failed to set sysctl value to 0 ", zap.String("name", name), zap.Error(err))
					return fmt.Errorf("failed to read current sysctl %+v value: %v ", name, err)
				}
			}
		}
		return nil
	})
	return err
}

// MoveRoutes make sure that the reply packets accessing the overlay interface are still sent from the overlay interface.
func MoveRoutes(logger *zap.Logger, netns ns.NetNS, routeMoveInterface string, currentInterfaceIPAddress []netlink.Addr, moveValue types.MoveRouteValue, ruleTable, ipFamily int) error {
	/*
			1. if moveValue = 0, do migrate directly
			2. if moveValue = 1, auto migrate route by interface name, if current_interface > last_interface by directory order, do migrate else nothing to do
			3. moveValue = 2 ,not do move
		    4. do move:
			 		a. add rule table by given interface name: ip rule add from <interface>/32 lookup table <ruleTable>
					b. move all route of given defaultInterface to table 100
	*/
	if moveValue == types.MoveValueNever {
		return nil
	}

	// make sure that traffic sent from current interface to lookup table <ruleTable>
	// eq: ip rule add from <currentInterfaceIPAddress> lookup <ruleTable>
	err := netns.Do(func(_ ns.NetNS) error {
		if err := AddFromRuleTable(logger, currentInterfaceIPAddress, ruleTable); err != nil {
			logger.Error("failed to AddFromRuleTable for currentInterfaceIPAddress", zap.Error(err))
			return fmt.Errorf("failed to AddFromRuleTable for currentInterfaceIPAddress: %v", err)
		}
		// move all routes of the specified interface to a new route table
		return moveRouteTable(logger, routeMoveInterface, ruleTable, ipFamily)

	})

	if err != nil {
		logger.Error("failed to moveRouteTable for routeMoveInterface", zap.String("routeMoveInterface", routeMoveInterface), zap.Error(err))
		return err
	}

	return nil
}

// moveRouteTable move all routes of the specified interface to a new route table
// Equivalent: `ip route del <route>` and `ip r route add <route> <table>`
func moveRouteTable(logger *zap.Logger, iface string, ruleTable, ipfamily int) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	routes, err := netlink.RouteList(nil, ipfamily)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	for _, route := range routes {

		// only handle route tables from table main
		if route.Table != unix.RT_TABLE_MAIN {
			continue
		}

		// ingore local link route
		if route.Dst.String() == "fe80::/64" {
			continue
		}

		logger.Debug("Found Route", zap.String("Route", route.String()))

		if route.LinkIndex == link.Attrs().Index {
			if err = netlink.RouteDel(&route); err != nil {
				logger.Error("failed to RouteDel in main", zap.String("route", route.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteDel %s in main table: %+v", route.String(), err)
			}
			logger.Debug("Del the route from main successfully", zap.String("Route", route.String()))

			route.Table = ruleTable
			if err = netlink.RouteAdd(&route); err != nil && os.IsExist(err) {
				logger.Error("failed to RouteAdd in new table ", zap.String("route", route.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteAdd (%+v) to new table: %+v", route, err)
			}
			logger.Debug("MoveRoute to new table successfully", zap.String("Route", route.String()))
		} else {
			// especially for ipv6 default route
			if len(route.MultiPath) == 0 {
				continue
			}

			// get generated default Route for new table
			for _, v := range route.MultiPath {
				if v.LinkIndex == link.Attrs().Index {
					logger.Debug("Found IPv6 Default Route", zap.String("Route", route.String()))
					if err := netlink.RouteDel(&route); err != nil {
						logger.Error("failed to RouteDel for IPv6", zap.String("Route", route.String()), zap.Error(err))
						return fmt.Errorf("failed to RouteDel %v for IPv6: %+v", route.String(), err)
					}

					route.Table = ruleTable
					if err = netlink.RouteAdd(&route); err != nil && !os.IsExist(err) {
						logger.Error("failed to RouteAdd for IPv6 to new table", zap.String("route", route.String()), zap.Error(err))
						return fmt.Errorf("failed to RouteAdd for IPv6 (%+v) to new table: %+v", route.String(), err)
					}
					break
				}
			}
		}
	}
	return nil
}

// SysctlRPFilter set rp_filter value
func SysctlRPFilter(netns ns.NetNS, rp *types.RPFilter) error {
	var err error
	if rp.Enable != nil && *rp.Enable {
		if err = setRPFilter(rp.Value); err != nil {
			return fmt.Errorf("failed to set rp_filter in host : %v", err)
		}
	}
	// set pod rp_filter
	err = netns.Do(func(_ ns.NetNS) error {
		if err := setRPFilter(rp.Value); err != nil {
			return fmt.Errorf("failed to set rp_filter in pod : %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func setRPFilter(v int32) error {
	dirs, err := os.ReadDir("/proc/sys/net/ipv4/conf")
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		name := fmt.Sprintf("/net/ipv4/conf/%s/rp_filter", dir.Name())
		value, err := sysctl.Sysctl(name)
		if err != nil {
			continue
		}
		if value == fmt.Sprintf("%d", v) {
			continue
		}
		if _, e := sysctl.Sysctl(name, fmt.Sprintf("%d", v)); e != nil {
			return e
		}
	}
	return nil
}
