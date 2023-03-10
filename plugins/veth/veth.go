// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	pVersion "github.com/spidernet-io/plugins/internal/version"
	"github.com/spidernet-io/plugins/pkg/config"
	"github.com/spidernet-io/plugins/pkg/logging"
	"github.com/spidernet-io/plugins/pkg/networking"
	ptypes "github.com/spidernet-io/plugins/pkg/types"
	ty "github.com/spidernet-io/plugins/pkg/types"
	"github.com/spidernet-io/plugins/pkg/utils"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

var (
	defaultMtu     = 1500
	defaultConVeth = "veth0"
	pluginName     = filepath.Base(os.Args[0])
)

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString(pluginName))
}

func cmdAdd(args *skel.CmdArgs) error {
	startTime := time.Now()

	conf, err := config.ParseVethConfig(args.StdinData)
	if err != nil {
		return err
	}

	if err := logging.InitLogger(conf.LogOptions, pluginName); err != nil {
		return fmt.Errorf("faild to init logger: %v ", err)
	}
	logger := logging.LoggerFile

	logger.Info("Veth starting", zap.String("Version", pVersion.GitCommit()), zap.String("Branch", pVersion.GitBranch()),
		zap.String("Commit", pVersion.GitCommit()),
		zap.String("Build time", pVersion.BuildDate()),
		zap.String("Go Version", pVersion.GoString()))

	k8sArgs := ty.K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		return fmt.Errorf("failed to get pod information, error=%+v \n", err)
	}

	// register some args into logger
	logger = logger.With(zap.String("Action", "Add"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("PodUID", string(k8sArgs.K8S_POD_UID)),
		zap.String("PodName", string(k8sArgs.K8S_POD_NAME)),
		zap.String("PodNamespace", string(k8sArgs.K8S_POD_NAMESPACE)),
		zap.String("IfName", args.IfName))

	ipFamily, err := networking.GetIPFamily(conf.PrevResult)
	if err != nil {
		logger.Error("failed to GetIPFamily", zap.Error(err))
		return err
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		logger.Error(err.Error())
		return fmt.Errorf("failed to GetNS %q: %v", args.Netns, err)
	}
	defer netns.Close()

	if len(conf.HwPrefix) != 0 {
		hwAddr, err := networking.OverrideHwAddress(logger, netns, conf.HwPrefix, args.IfName)
		if err != nil {
			return fmt.Errorf("failed to update hardware address for interface %s, maybe hardware_prefix(%s) is invalid: %v", args.IfName, conf.HwPrefix, err)
		}

		logger.Info("Override hardware address successfully", zap.String("interface", args.IfName), zap.String("hardware address", hwAddr))
		if conf.OnlyHardware {
			logger.Debug("Only override hardware address, ending to call veth")
			return types.PrintResult(conf.PrevResult, conf.CNIVersion)
		}
	}

	isfirstInterface, e := isInterfaceExists(netns, defaultConVeth)
	if e != nil {
		logger.Error("failed to check if is first veth interface", zap.Error(e))
		return fmt.Errorf("failed to check first veth interface: %v", e)
	}

	if !isfirstInterface {
		logger.Info("Calling veth plugin not for the first time", zap.Any("config", conf), zap.String("netns", netns.Path()))
	} else {
		logger.Info("Calling veth plugin for first time", zap.Any("config", conf), zap.String("netns", netns.Path()))
	}

	var hostVethPairName string
	hostVethPairName, err = setupVeth(netns, isfirstInterface, args.ContainerID)
	if err != nil {
		logger.Error("failed to create veth-pair device", zap.Error(err))
		return err
	}

	logger.Debug("Setup veth-pair device successfully", zap.String("hostVethPairName", hostVethPairName))

	// get all ip address on the node
	ipAddressOnNode, err := networking.IPAddressOnNode(logger, ipFamily)
	if err != nil {
		logger.Error("failed to get IPAddressOnNode", zap.Error(err))
		return fmt.Errorf("failed to get IPAddressOnNode: %v", err)
	}

	// get ips of this interface(preInterfaceName) from, including ipv4 and ipv6
	preInterfaceIPAddress, err := networking.IPAddressByName(netns, args.IfName, ipFamily)
	if err != nil {
		logger.Error(err.Error())
		return fmt.Errorf("failed to find ip from chained interface %s : %v", args.IfName, err)
	}

	logger.Info("Get the address of interface successfully", zap.String("interface", args.IfName), zap.Any("preInterfaceIPAddress", preInterfaceIPAddress))

	if ipFamily != netlink.FAMILY_V4 {
		// ensure ipv6 is enable
		if err := networking.EnableIpv6Sysctl(logger, netns); err != nil {
			return err
		}
	}

	if err = setupNeighborhood(logger, netns, hostVethPairName, isfirstInterface, ipAddressOnNode, preInterfaceIPAddress); err != nil {
		logger.Error(err.Error())
		return err
	}

	ruleTable := unix.RT_TABLE_MAIN
	if !isfirstInterface {
		ruleTable = utils.GetRuleNumber(args.IfName)
		if ruleTable < 0 {
			logger.Error("In multi-NIC mode, the first NIC can only be Macvlan/SR-IOV + Veth, Not Calico or CIilum")
			return fmt.Errorf("In multi-NIC mode, the first NIC can only be Macvlan/SR-IOV + Veth, Not Calico or Ciilum")
		}
	}

	if err = setupRoutes(logger, netns, ruleTable, hostVethPairName, ipAddressOnNode, preInterfaceIPAddress, conf); err != nil {
		logger.Error(err.Error())
		return err
	}

	if !isfirstInterface {
		if err = networking.MoveRoutes(logger, netns, args.IfName, preInterfaceIPAddress, conf.MoveRoutes, ruleTable, ipFamily); err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	if err = networking.SysctlRPFilter(netns, conf.RPFilter); err != nil {
		logger.Error("failed to SysctlRPFilter", zap.Any("rp_filter", conf.RPFilter), zap.Error(err))
		return err
	}

	logger.Info("succeeded to call veth-plugin", zap.Int64("Time Cost", time.Since(startTime).Microseconds()))
	return types.PrintResult(conf.PrevResult, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	// nothing to do
	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	// TODO
	return fmt.Errorf("not implement it")
}

// setupVeth sets up a pair of virtual ethernet devices. move one to the host and other
// one to container.
func setupVeth(netns ns.NetNS, firstInvoke bool, containerID string) (string, error) {
	if !firstInvoke {
		return getHostVethName(containerID), nil
	}
	var hostInterface, containerInterface net.Interface
	err := netns.Do(func(hostNS ns.NetNS) error {
		var err error
		hostInterface, containerInterface, err = ip.SetupVethWithName(defaultConVeth, getHostVethName(containerID), defaultMtu, "", hostNS)
		if err != nil {
			return err
		}

		link, err := netlink.LinkByName(containerInterface.Name)
		if err != nil {
			return err
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to set %q UP: %v", containerInterface.Name, err)
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return hostInterface.Name, nil
}

// setupNeighborhood setup neighborhood tables for pod and host.
// equivalent to: `ip neigh add ....`
func setupNeighborhood(logger *zap.Logger, netns ns.NetNS, hostVethPairName string, isfirstInterface bool, ipAddressOnNode, preInterfaceIPAddress []netlink.Addr) error {
	var err error
	hostVethHwAddress, containerVethHwAddress, err := networking.HwAddressByName(netns, hostVethPairName)
	if err != nil {
		return err
	}

	for _, ipAddr := range preInterfaceIPAddress {
		if err = networking.AddNeighborTable(hostVethPairName, ipAddr.IP, containerVethHwAddress); err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	if !isfirstInterface {
		// In the pod, we have already add neighbor table, so exit...
		return nil
	}

	logger.Debug("setupNeighborhood",
		zap.String("hostVethPairName", hostVethPairName),
		zap.String("hostVethHwAddress", hostVethHwAddress.String()),
		zap.String("containerVethHwAddress", containerVethHwAddress.String()))

	err = netns.Do(func(_ ns.NetNS) error {
		for _, ipAddr := range ipAddressOnNode {
			if err := networking.AddNeighborTable(defaultConVeth, ipAddr.IP, hostVethHwAddress); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	return err
}

// setupRoutes setup routes for pod and host
// equivalent to: `ip route add $route`
func setupRoutes(logger *zap.Logger, netns ns.NetNS, ruleTable int, hostVethPairName string, ipAddressOnNode, preInterfaceIPAddress []netlink.Addr, conf *ptypes.Veth) error {
	v4Gw, v6Gw, err := networking.GetGatewayIP(preInterfaceIPAddress)
	if err != nil {
		logger.Error("failed to GetGatewayIP", zap.Error(err))
		return err
	}

	err = netns.Do(func(_ ns.NetNS) error {
		var err error
		// traffic sent to the node is forwarded via veth0
		// eq:  "ip r add <ipAddressOnNode> dev veth0 table <ruleTable> "
		if err = networking.AddRouteTable(logger, ruleTable, netlink.SCOPE_LINK, defaultConVeth, networking.AddrsToString(ipAddressOnNode), nil, nil); err != nil {
			logger.Error("failed to AddRouteTable for ipAddressOnNode", zap.Error(err))
			return fmt.Errorf("failed to AddRouteTable for ipAddressOnNode: %v", err)
		}

		// make sure that veth0 forwards traffic within the cluster
		// eq: ip route add <cluster/service cidr> dev veth0
		localCIDRs := append(conf.ClusterCIDR, conf.ServiceCIDR...)
		localCIDRs = append(localCIDRs, conf.AdditionalCIDR...)
		if err = networking.AddRouteTable(logger, ruleTable, netlink.SCOPE_UNIVERSE, defaultConVeth, localCIDRs, v4Gw, v6Gw); err != nil {
			logger.Error("failed to AddRouteTable for localCIDRs", zap.Error(err))
			return fmt.Errorf("failed to AddRouteTable for localCIDRs: %v", err)
		}

		// As for more than two macvlan interface, we need to add something like below shown:
		// make sure that all traffic to second NIC to lookup table <<ruleTable>>
		// eq: ip rule add to <preInterfaceIPAddress> lookup table <ruleTable>
		if ruleTable != unix.RT_TABLE_MAIN {
			if err = networking.AddToRuleTable(preInterfaceIPAddress, ruleTable); err != nil {
				logger.Error("failed to AddToRuleTable", zap.Error(err))
				return fmt.Errorf("failed to AddToRuleTable: %v", err)
			}
		}
		logger.Debug("AddRouteTable for localCIDRs successfully", zap.Strings("localCIDRs", localCIDRs))
		return nil
	})

	if err != nil {
		return err
	}

	// set routes for host
	// equivalent: ip add  <chainedIPs> dev veth-peer on host
	if err = networking.AddRouteTable(logger, unix.RT_TABLE_MAIN, netlink.SCOPE_UNIVERSE, hostVethPairName, networking.AddrsToString(preInterfaceIPAddress),
		nil, nil); err != nil {
		logger.Error("failed to AddRouteTable for preInterfaceIPAddress", zap.Error(err))
		return fmt.Errorf("failed to AddRouteTable for preInterfaceIPAddress: %v", err)
	}

	return err
}

// isInterfaceExists returns true by checking if the interface exists in the netns
func isInterfaceExists(netns ns.NetNS, iface string) (bool, error) {
	e := netns.Do(func(_ ns.NetNS) error {
		_, err := netlink.LinkByName(iface)
		return err
	})

	if e == nil {
		return false, nil
	}
	if strings.EqualFold(e.Error(), ip.ErrLinkNotFound.Error()) {
		return true, nil
	} else {
		return false, e
	}
}

// getHostVethName select the first 11 characters of the containerID for the host veth.
func getHostVethName(containerID string) string {
	return fmt.Sprintf("veth%s", containerID[:min(len(containerID))])
}

func min(len int) int {
	if len > 11 {
		return 11
	}
	return len
}
