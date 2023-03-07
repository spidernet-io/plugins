#！/bin/bash

set -o errexit -o nounset -o pipefail

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT_PATH=$( cd ${CURRENT_DIR_PATH}/../.. && pwd )

[  -z "${IP_FAMILY}" ] &&  echo "must be provide IP_FAMILY by using env IP_FAMILY" && exit 1
[  -z "${DEFAULT_CNI}" ] &&  echo "must be provide DEFAULT_CNI by using env DEFAULT_CNI" && exit 1
[ -z "${INSTALL_TIME_OUT}" ] && INSTALL_TIME_OUT=600s

# Multus config
MACVLAN_MASTER=${MACVLAN_MASTER:-eth0}
MACVLAN_TYPE=${MACVLAN_TYPE:-macvlan-standalone}

case ${IP_FAMILY} in
  ipv4)
    SERVICE_HIJACK_SUBNET="[\"${CLUSTER_SERVICE_SUBNET_V4}\"]"
    OVERLAY_HIJACK_SUBNET="[\"${CLUSTER_POD_SUBNET_V4}\"]"
    ;;
  ipv6)
    SERVICE_HIJACK_SUBNET="[\"${CLUSTER_SERVICE_SUBNET_V6}\"]"
    OVERLAY_HIJACK_SUBNET="[\"${CLUSTER_POD_SUBNET_V6}\"]"
    ;;
  dual)
    SERVICE_HIJACK_SUBNET="[\"${CLUSTER_SERVICE_SUBNET_V4}\",\"${CLUSTER_SERVICE_SUBNET_V6}\"]"
    OVERLAY_HIJACK_SUBNET="[\"${CLUSTER_POD_SUBNET_V4}\",\"${CLUSTER_POD_SUBNET_V6}\"]"
    ;;
  *)
    echo "the value of IP_FAMILY: ipv4 or ipv6 or dual"
    exit 1
esac

git clone https://github.com/k8snetworkplumbingwg/multus-cni.git
cat multus-cni/deployments/multus-daemonset-thick.yml | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

# prepare image
# wait multus-ready
kubectl wait --for=condition=ready -l app=multus --timeout=${INSTALL_TIME_OUT} pod -n kube-system --kubeconfig ${E2E_KUBECONFIG}

if [ "${DEFAULT_CNI}" == "k8s-pod-network" ] ; then
cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
  name: macvlan-standalone-vlan${MACVLAN_VLANID}
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-standalone",
        "plugins": [
            {
                "type": "macvlan",
                "master": "eth0.${MACVLAN_VLANID}",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool",
                }
            },{
                "type": "veth",
                "service_cidr": ${SERVICE_HIJACK_SUBNET},
                "cluster_cidr": ${OVERLAY_HIJACK_SUBNET},
                "hardware_prefix": "0a:0c"
            }
        ]
    }
EOF
fi

echo -e "\033[35m Succeed to install Multus \033[0m"
