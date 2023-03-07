# Spider Plugins

[![Run E2E Kind Test](https://github.com/spidernet-io/plugins/actions/workflows/e2e-test.yaml/badge.svg?branch=main)](https://github.com/spidernet-io/plugins/actions/workflows/e2e-test.yaml)
[![Auto Golang Lint And Unitest](https://github.com/spidernet-io/plugins/actions/workflows/lint-golang.yaml/badge.svg?branch=main)](https://github.com/spidernet-io/plugins/actions/workflows/lint-golang.yaml)

Some legacy applications expect to be directly connected to the physical network. The Pod has the performance of the physical network and with real source IP. In this type of situation, you maybe use the MacVlan or SR-IoV CNI to achieve this. And if you use MacVlan CNI, you maybe be run into some network communication bugs, As shown the following issues below:

- The pod can't access to service clusterIP.
- The pod can't access to other network's pod.
- The pod can't access to node(The master interface on node and sub-interface cannot communicate directly)

So spider plugins work on solving communication issues in multi-CNI and multi-NIC mode. The list of plugins that have been developed so far is shown below:

- **Veth**: Work for Macvlan CNI、SR-IOV CNI etc. Solve the problem that MacVlan Pod cannot communicate with ClusterIP when it is the default CNI.

> NOTE: Note that it is in an early stage, and while we welcome usage and experimentation, You are welcome to open issue or PR if you could run into bugs.

## How it works

Similar to the way [CNI Meta plugins](https://github.com/containernetworking/plugins/tree/main/plugins/meta) work, It is not used to create interface and is often called after the [CNI Main plugin](https://github.com/containernetworking/plugins/tree/main/plugins/main) call.
For more information, see the [CNI Plugins](https://www.cni.dev/plugins/current/).

`Veth` creates a veth pair and forwards traffic within the cluster through the veth pair device:

![](docs/images/veth.svg)

The diagram is a schematic of how the Veth plugin works: 

- Creates a veth pair device, One end of the veth pair is placed inside a container and the other end resides on the host.
- Hijacks traffic sent from Pod to cluster to be forwarded by veth pair device.

## Quick Start

_**Prerequisites**_:

- You first have installed Kubernetes and have configured a default network(Such as calico)
- Install multus: Refer to [Install multus](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md)
- Install spiderpool(an ipam of kubernetes): Refer to [Install spiderpool](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/basic.md)
- Install cni-plugins: Download the [binary files](https://github.com/containernetworking/plugins/releases) and extract it to the `/opt/cni/bin path` of each node.
- Install spider-plugins:  Download the [binary files]() and extract it to the `/opt/cni/bin path` of each node.

1. Create multus network-attachment-definition crd: `macvlan-conf`:

```shell
cat <<< kubectl apply -f - 
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-standalone",
        "plugins": [
            {
                "type": "macvlan",
                "master": "ens192",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "veth",
                "cluster_cidr": ["10.233.64.0/18"],
                "service_cidr": ["10.233.0.0/18"]
            }
        ]
    }
EOF
```

Note:

- This example uses `ens192` as the master parameter, this master parameter should match the interface name on the hosts in your cluster.
- This example uses `spiderpool` as the `ipam` parameter. More details about spiderpool refer to [Spiderpool](https://spidernet-io.github.io/spiderpool/).
- `cluster_cidr` and `service_cidr` parameters should match the networking cidr on your cluster.

You can see which configurations you've created using kubectl here's how you can do that:

```shell
kubectl get network-attachment-definitions -n kube-system macvlan-conf -o yaml
```

2. Creating a macvlan pod that uses `Network-Attachment-Definition`: `macvlan-conf`

We can specify the default cluster network in pods with the `v1.multus-cni.io/default-network` annotation. In this example, We specify Macvlan CNI as the default cluster CNI for the Pod: 

```shell
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: Pod
metadata:
  name: macvlan-vlan0
  annotations:
    v1.multus-cni.io/default-network: kube-system/macvlan-conf
spec:
  containers:
  - name: dao2048
    image: ghcr.io/daocloud/dao-2048:v1.2.0
EOF
```

3. Verify

`10.6.212.79` is the IP assigned to the Pod through `spiderpool`, which is an underlay type of IP, the same level as the host. The external clients of the cluster can access the Pod directly through this IP.

```shell
root@controller:~# kubectl get po -o wide
NAME                                        READY   STATUS             RESTARTS        AGE     IP               NODE         NOMINATED NODE   READINESS GATES
macvlan-vlan0-65b6cff6f9-qnpkn              1/1     Running            0               1m      10.6.212.79      controller   <none>           <none>
```

```shell
# on the external clients of the cluster
➜  ~ ping 10.6.212.79 -c 1
PING 10.6.212.79 (10.6.212.79): 56 data bytes
64 bytes from 10.6.212.79: icmp_seq=0 ttl=60 time=47.401 ms

--- 10.6.212.79 ping statistics ---
1 packets transmitted, 1 packets received, 0.0% packet loss
round-trip min/avg/max/stddev = 47.401/47.401/47.401/0.000 ms
```

Pods can be access to ClusterIP:

```shell
# access to kubernetes service clusterIP
root@controller:~# kubectl exec  macvlan-vlan0-65b6cff6f9-qnpkn -- curl -k -I https://10.233.0.1
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0
HTTP/2 403
audit-id: f6d1b119-6284-4b56-b33d-0a2f8e474825
cache-control: no-cache, private
content-type: application/json
x-content-type-options: nosniff
x-kubernetes-pf-flowschema-uid: 4477cb05-3e82-4508-b2d2-e54cfa045996
x-kubernetes-pf-prioritylevel-uid: 587c7f5b-2d88-43dc-8954-83156d2ffbfc
content-length: 218
```

Pod can be access to other's network(calico etc.)

```shell
# access to calico pod
root@controller:~# kubectl get po -o wide
NAME                                        READY   STATUS             RESTARTS        AGE     IP               NODE         NOMINATED NODE   READINESS GATES
macvlan-vlan0-65b6cff6f9-qnpkn              1/1     Running            0               1m      10.6.212.79      controller   <none>           <none>
test-794b96cd5b-d92pp                       1/1     Running            0               8d      10.233.119.208   controller   <none>           <none>
root@controller:~# kubectl exec  macvlan-vlan0-65b6cff6f9-qnpkn -- ping 10.233.119.208
PING 10.233.119.208 (10.233.119.208): 56 data bytes
64 bytes from 10.233.119.208: seq=0 ttl=63 time=0.631 ms

--- 10.233.119.208 ping statistics ---
1 packets transmitted, 1 packets received, 0% packet loss
round-trip min/avg/max = 0.631/0.631/0.631 ms
```

## Features

- Support Macvlan/SR-IOV pod can be access ClusterCIDR, ServiceCIDR etc.
- When Pod has multiple interfaces, Support inter-communication between different interfaces (networks) by setting policy route table.
- Support for setting specific mac addresses for Pods.
- Dual-stack enabled.

## Contribution

For any questions about Spider-plugins, feel free to open up a GitHub issue :)