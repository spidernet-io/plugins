package config

import (
	"encoding/json"
	"fmt"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/spidernet-io/cni-plugins/pkg/constant"
	"github.com/spidernet-io/plugins/pkg/logging"
	"github.com/spidernet-io/plugins/pkg/types"
	"k8s.io/utils/pointer"
	"net"
	"regexp"
	"strings"
)

// ParseVethConfig parses the supplied configuration (and prevResult) from stdin.
func ParseVethConfig(stdin []byte) (*types.Veth, error) {
	var err error
	conf := types.Veth{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	if err := version.ParsePrevResult(&conf.NetConf); err != nil {
		return nil, fmt.Errorf("failed to parse prevResult: %v", err)
	}

	if conf.PrevResult == nil {
		return nil, fmt.Errorf("failed to find PrevResult, must be called as chained plugin")
	}

	if err = validateHwPrefix(conf.HwPrefix); err != nil {
		return nil, err
	}

	conf.LogOptions = logging.InitLogOptions(conf.LogOptions)
	if conf.LogOptions.LogFilePath == "" {
		conf.LogOptions.LogFilePath = constant.VethLogDefaultFilePath
	}

	if conf.OnlyHardware {
		return &conf, nil
	}

	if err = ValidateRoutes(conf.ClusterCIDR, conf.ServiceCIDR, conf.AdditionalCIDR); err != nil {
		return nil, err
	}

	// value must be 0/1/2
	// If not, giving default value: RPFilter_Loose(2) to it
	if conf.RPFilter == nil {
		conf.RPFilter = &types.RPFilter{
			Enable: pointer.Bool(true),
			Value:  0,
		}
	} else {
		validateRPFilterConfig(conf.RPFilter)
	}

	return &conf, nil
}

func validateHwPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	// prefix format like: 00:00、0a:1b
	matchRegexp, err := regexp.Compile("^" + "(" + "[a-fA-F0-9]{2}[:-][a-fA-F0-9]{2}" + ")" + "$")
	if err != nil {
		return err
	}
	if !matchRegexp.MatchString(prefix) {
		return fmt.Errorf("mac_prefix format should be match regex: [a-fA-F0-9]{2}[:][a-fA-F0-9]{2}, like '0a:1b'")
	}
	return nil
}

func ValidateRoutes(clusterSubnet, serviceSubnet, other []string) error {
	subnets := append(clusterSubnet, serviceSubnet...)
	subnets = append(subnets, other...)

	err := validateRoutes(subnets)
	if err != nil {
		return err
	}

	return nil
}

func validateRoutes(routes []string) error {
	if len(routes) == 0 {
		return nil
	}
	result := make([]string, len(routes))
	for idx, route := range routes {
		result[idx] = strings.TrimSpace(route)
	}
	for _, route := range result {
		_, _, err := net.ParseCIDR(route)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateRPFilterConfig(rpfilter *types.RPFilter) {
	if rpfilter == nil {
		return
	}
	if rpfilter.Enable == nil {
		rpfilter.Enable = pointer.Bool(true)
		rpfilter.Value = 0
	}

	if *rpfilter.Enable {
		found := false
		for _, value := range []int32{0, 1, 2} {
			if rpfilter.Value == value {
				found = true
				break
			}
		}
		if !found {
			rpfilter.Value = 0
		}
	}
}
