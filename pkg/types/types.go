package types

import (
	"github.com/containernetworking/cni/pkg/types"
	"net"
)

type MoveRouteValue int32

const (
	MoveValueDirectly MoveRouteValue = iota
	MoveValueAuto
	MoveValueNever
)

type Veth struct {
	types.NetConf
	OnlyHardware   bool     `json:"only_hardware,omitempty"`
	HwPrefix       string   `json:"hardware_prefix,omitempty"`
	ClusterCIDR    []string `json:"cluster_cidr,omitempty"`
	ServiceCIDR    []string `json:"service_cidr,omitempty"`
	AdditionalCIDR []string `json:"additional_cidr,omitempty"`
	// RpFilter
	RPFilter   *RPFilter      `json:"rp_filter,omitempty" `
	MoveRoutes MoveRouteValue `json:"move_routes,omitempty"`
	LogOptions *LogOptions    `json:"log_options,omitempty"`
}

type LogOptions struct {
	LogLevel        string `json:"log_level"`
	LogFilePath     string `json:"log_file"`
	LogFileMaxSize  *int   `json:"log_max_size"`
	LogFileMaxAge   *int   `json:"log_max_age"`
	LogFileMaxCount *int   `json:"log_max_count"`
}

type RPFilter struct {
	// setup host rp_filter
	Enable *bool `json:"enabled,omitempty"`
	// the value of rp_filter, must be 0/1/2
	Value int32 `json:"value,omitempty"`
}

// K8sArgs is the valid CNI_ARGS used for Kubernetes
type K8sArgs struct {
	types.CommonArgs
	IP                         net.IP
	K8S_POD_NAME               types.UnmarshallableString //revive:disable-line
	K8S_POD_NAMESPACE          types.UnmarshallableString //revive:disable-line
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString //revive:disable-line
	K8S_POD_UID                types.UnmarshallableString //revive:disable-line
}
