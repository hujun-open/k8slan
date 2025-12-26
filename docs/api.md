# API Reference

## Packages
- [lan.k8slan.io/v1beta1](#lank8slaniov1beta1)


## lan.k8slan.io/v1beta1

Package v1beta1 contains API Schema definitions for the lan v1beta1 API group.

### Resource Types
- [LAN](#lan)
- [LANList](#lanlist)



#### LAN



LAN is the Schema for the lans API



_Appears in:_
- [LANList](#lanlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `lan.k8slan.io/v1beta1` | | |
| `kind` _string_ | `LAN` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LANSpec](#lanspec)_ | spec defines the desired state of LAN |  |  |


#### LANList



LANList contains a list of LAN





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `lan.k8slan.io/v1beta1` | | |
| `kind` _string_ | `LANList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LAN](#lan) array_ |  |  |  |


#### LANSpec



LANSpec defines the desired state of LAN



_Appears in:_
- [LAN](#lan)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ns` _string_ | linux namespace name where LAN bridge, vxlan interfaces live in |  |  |
| `bridge` _string_ | LAN bridge interface name |  |  |
| `vxlan` _string_ | LAN VxLAN interface name |  |  |
| `vni` _integer_ | VxLAN VNI |  |  |
| `vxlanGrp` _string_ | VxLAN multicast group address |  |  |
| `defaultVxlanDev` _string_ | default VxLAN device name if not spcified in vxlanDevMap |  |  |
| `vxlanDevMap` _object (keys:string, values:string)_ | a map between k8s worker name and its interface name used as the LAN VxLAN interface's device |  |  |
| `vxlanPort` _integer_ | VxLAN UDP port |  |  |
| `spokes` _string array_ | A list of spoke name |  |  |




