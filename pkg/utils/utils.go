package utils

import (
	"strconv"
	"strings"
)

var (
	overlayRouteTable     = 100
	firstInterfaceName    = "eth0"
	secondInterfacePrefix = "net"
)

// GetRuleNumber return the number of rule table corresponding to the previous interface from the given interface.
// the input format must be 'net+number'
// for example:
// input: net1, output: 100(eth0)
// input: net2, output: 101(net1)
func GetRuleNumber(iface string) int {
	if !strings.HasPrefix(iface, "net") {
		return -1
	}
	numStr := strings.Trim(iface, "net")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return -1
	}
	return overlayRouteTable + num - 1
}

// CompareInterfaceName compare name from given current and prev by directory order
// example:
// net1 > eth0, true
// net2 > net1, true
func CompareInterfaceName(current, prev string) bool {
	if prev == firstInterfaceName {
		return true
	}
	if !strings.HasPrefix(current, secondInterfacePrefix) || !strings.HasPrefix(prev, secondInterfacePrefix) {
		return false
	}
	return current >= prev
}
