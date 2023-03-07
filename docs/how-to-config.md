# How to config

### A simple example

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
   name: macvlan-standalone
   namespace: kube-system
spec:
   config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-standalone",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool",
                  }
              },{
                  "type": "veth",
                  "service_cidr": ["10.233.64.0/18"],
                  "cluster_cidr": ["10.233.0.0/16"]
              }
          ]
      }
```


### In a dual-stack cluster, Configure IPv4/IPv6 CIDR.

`service_cidr` and `cluster_cidr`: Supports configuration of multiple cidr, including ipv4/ipv6:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
   name: macvlan-standalone
   namespace: kube-system
spec:
   config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-standalone",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool",
                  }
              },{
                  "type": "veth",
                  "service_cidr": ["10.233.0.0/18","fd00:10:96::/112"],
                  "cluster_cidr": ["10.233.64.0/18","fd00:10:244::/112"]
              }
          ]
      }
```

### Configure additional_cidr

By default, all traffic is forwarded directly through eth0(default route in it), except for traffic sent by Pod to the cluster, which is forwarded through the veth pair device. 
If you want traffic to certain destinations to be forwarded through hosts, you can configure `additional_cidr` to achieve this:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
   name: macvlan-standalone
   namespace: kube-system
spec:
   config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-standalone",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool",
                  }
              },{
                  "type": "veth",
                  "service_cidr": ["10.233.0.0/18","fd00:10:96::/112"],
                  "cluster_cidr": ["10.244.0.0/16","fd00:10:244::/112"],
                  "additional_cidr": ["172.20.0.0/16"]
              }
          ]
      }
```

### Configure custom mac prefixes

`hardware_preifx` is a unified mac address prefix, Length is 4 hex digits. Input format like: "1a:2b". If it's be empty, it's means disable this feature.

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
   name: macvlan-standalone
   namespace: kube-system
spec:
   config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-standalone",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool",
                  }
              },{
                  "type": "veth",
                  "service_cidr": ["10.233.0.0/18","fd00:10:96::/112"],
                  "cluster_cidr": ["10.244.0.0/16","fd00:10:244::/112"],
                  "hardware_prefix": "0a:1b"
              }
          ]
      }
```

When pod is created, you see the mac address of pod(eth0) has been changed:

```shell
root@qf-master1:~# kubectl exec -it  macvlan-standalone-vlan0-f4d6d8776-9r9lf sh
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
/ # ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop state DOWN qlen 1000
    link/ipip 0.0.0.0 brd 0.0.0.0
3: eth0@if7: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 0a:1b:0a:14:14:5f brd ff:ff:ff:ff:ff:ff
    inet 10.20.20.95/16 brd 10.20.255.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fd00:10:20::101/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::c94:54ff:fef9:d94a/64 scope link
       valid_lft forever preferred_lft forever
4: veth0@if513: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 86:4c:47:dc:11:dc brd ff:ff:ff:ff:ff:ff
    inet6 fe80::844c:47ff:fedc:11dc/64 scope link
       valid_lft forever preferred_lft forever
```

For only one NIC is created by `calico`, You may only need to change to update its mac address and nothing else, You can refer to the following configuration:

`veth`:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
   name: macvlan-standalone
   namespace: kube-system
spec:
  config: |-
      {
        "name": "calico",
        "cniVersion":"0.3.1",
        "plugins":[
          {
            "datastore_type": "kubernetes",
            "nodename": "controller",
            "type": "calico",
            "log_level": "info",
            "log_file_path": "/var/log/calico/cni/cni.log",
            "ipam": {
              "type": "calico-ipam",
              "assign_ipv6": "true",
              "ipv6_pools": ["fd85:ee78:d8a6:8607::1:0000/112"],
              "ipv4_pools": ["10.233.64.0/18"],
              "assign_ipv4": "true"
            },
            "policy": {
              "type": "k8s"
            },
            "kubernetes": {
              "kubeconfig": "/etc/cni/net.d/calico-kubeconfig"
            }
          },
          {
            "type":"portmap",
            "capabilities": {
              "portMappings": true
            }
          },
          {
            "type":"bandwidth",
            "capabilities": {
              "bandwidth": true
            }
          },
            "type": "veth",
            "hardware_prefix": "0a:1b",
            "only_hardware": true
        ]
      }
```

### Custom log options

`log_options` is used to config logger, as shown following:

```json
              "log_options": {
                "log_level": "debug",
                "log_file": "/var/log/meta-plugins/veth.log"
              }
```

You can config `log_level(default to debug)` and `log_file(default to/var/log/meta-plugins/router.log)`.
