package networking

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"math/big"
	"net"
	"net/netip"
	"os"
	"regexp"
)

// AddNeighborTable add static neighborhood table
func AddNeighborTable(iface string, dstIP net.IP, hwAddress net.HardwareAddr) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("failed to get link: %v", err)
	}

	neigh := &netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		State:        netlink.NUD_PERMANENT,
		Type:         netlink.NDA_LLADDR,
		IP:           dstIP,
		HardwareAddr: hwAddress,
	}

	if err := netlink.NeighAdd(neigh); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to add neigh table: %v ", err)
	}

	return nil
}

// OverrideHwAddress override the hardware address of the specified interface.
func OverrideHwAddress(logger *zap.Logger, netns ns.NetNS, macPrefix, iface string) (string, error) {
	ips, err := IPAddressByName(netns, iface, netlink.FAMILY_ALL)
	if err != nil {
		logger.Error("failed to get IPAddressByName", zap.String("interface", iface), zap.Error(err))
		return "", err
	}

	// we only focus on first element
	nAddr, err := netip.ParseAddr(ips[0].IP.String())
	if err != nil {
		logger.Error("failed to ParsePrefix", zap.Error(err))
		return "", err
	}

	suffix, err := inetAton(nAddr)
	if err != nil {
		logger.Error("failed to inetAton", zap.Error(err))
		return "", err
	}

	// newmac = xx:xx + xx:xx:xx:xx
	hwAddr := macPrefix + ":" + suffix
	err = netns.Do(func(netNS ns.NetNS) error {
		link, err := netlink.LinkByName(iface)
		if err != nil {
			logger.Error(err.Error())
			return err
		}
		return netlink.LinkSetHardwareAddr(link, parseMac(hwAddr))
	})

	if err != nil {
		logger.Error("failed to OverrideHwAddress", zap.String("hardware address", hwAddr), zap.Error(err))
		return "", err
	}
	return hwAddr, nil
}

// parseMac parse hardware addr from given string
func parseMac(s string) net.HardwareAddr {
	hardwareAddr, err := net.ParseMAC(s)
	if err != nil {
		panic(err)
	}
	return hardwareAddr
}

// inetAton converts an IP Address (IPv4 or IPv6) netip.addr object to a hexadecimal representation.
func inetAton(ip netip.Addr) (string, error) {
	if ip.AsSlice() == nil {
		return "", fmt.Errorf("invalid ip address")
	}

	ipInt := big.NewInt(0)
	// 32 bit -> 4 B
	hexCode := make([]byte, hex.EncodedLen(ip.BitLen()))
	ipInt.SetBytes(ip.AsSlice()[:])
	hex.Encode(hexCode, ipInt.Bytes())

	if ip.Is6() {
		// for ipv6: 128 bit = 32 hex
		// take the last 8 hex as the hardware address
		return convertHex2Mac(hexCode[24:]), nil
	}

	return convertHex2Mac(hexCode), nil
}

// convertHex2Mac convert hexcode to 4B hardware address
// convert ip(hex) to "xx:xx:xx:xx"
func convertHex2Mac(hexCode []byte) string {
	regexSpilt, err := regexp.Compile(".{2}")
	if err != nil {
		panic(err)
	}
	return string(bytes.Join(regexSpilt.FindAll(hexCode, 4), []byte(":")))
}

func HwAddressByName(netns ns.NetNS, hostVethPairName string) (net.HardwareAddr, net.HardwareAddr, error) {
	hostVethLink, err := netlink.LinkByName(hostVethPairName)
	if err != nil {
		return nil, nil, err
	}

	var containerVethHwAddree net.HardwareAddr
	err = netns.Do(func(netNS ns.NetNS) error {
		containerVethLink, err := netlink.LinkByName("veth0")
		if err != nil {
			return err
		}
		containerVethHwAddree = containerVethLink.Attrs().HardwareAddr
		return nil
	})
	return hostVethLink.Attrs().HardwareAddr, containerVethHwAddree, nil

}
