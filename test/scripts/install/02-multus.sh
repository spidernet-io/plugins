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



if [ ${RUN_ON_LOCAL} == false ]; then
  MULTUS_HELM_OPTIONS+=" --set multus.image.registry=ghcr.io \
  --set sriov.images.sriovCni.registry=ghcr.io \
  --set sriov.images.sriovDevicePlugin.registry=ghcr.io "
fi

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
    v1.multus-underlay-cni.io/coexist-types: '["macvlan-standalone"]'
    v1.multus-underlay-cni.io/default-cni: "true"
    v1.multus-underlay-cni.io/instance-type: macvlan_standalone
    v1.multus-underlay-cni.io/underlay-cni: "true"
    v1.multus-underlay-cni.io/vlanId: "${MACVLAN_VLANID}"
  labels:
    v1.multus-underlay-cni.io/instance-status: enable
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
