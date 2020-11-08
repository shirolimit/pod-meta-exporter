# pod-meta-exporter

Utility that tracks pods on a node and exports pod metadata to files.

Its main purpose is to overcome limitation of current verson of `kubernetes` Fluent Bit filter.
At the moment `kubernetes` Fluent Bit plugin can't work in a watch mode,
it only fetches pod metadata via Kubernetes REST API when log record from a pod is processed.
As a result it sometimes fails to fetch Kubernetes metadata for short-lived pods.

Fluent Bit `kubernetes` filter plugin can load pod metadata from a filesystem and this functionality
can be used to overcome the limitation described above.
A sidecar container can be responsible for putting pod metadata to a directory accessible by Fluent Bit.

`pod-meta-exporter` tracks pods on a node using Kubernetes `watch` API and exports pod metadata as files
named `<namespace>_<pod_name>.meta`.

## How to build

The easiest way is to use `make`:

```bash
$ make build
```

## How to run

`pod-meta-exporter` accepts configuration parameters from both command line arguments and environment variables.

| Command line arg | Environment variable | Description | Default value |
| --- | --- | --- | --- |
| `node-name` | `EXPORTER_NODE_NAME` | The name of a node to track pods on. | `HOSTNAME` |
| `destination-dir` | `EXPORTER_DESTINATION_DIR` | The path to a directory to put files to. | `/var/kube_meta` |
| `retention-period` | `EXPORTER_RETENTION_PERIOD` | The amount of time to keep a file with metadata after a pod was deleted. | `1m` |

Example:

```bash
$ pod-meta-exporter --node-name 'my-node' --destination-dir '/var/kube_meta' --retention-period 5m
```

## How to use with Fluent Bit

First of all Fluent Bit `kubernetes` filter should be configured to load metadata from local files.
Here is an example:

```
[FILTER]
    Name                            kubernetes
    Match                           kube.*
    Kube_URL                        https://kubernetes.default.svc:443
    Kube_Tag_Prefix                 kube.var.log.containers.  
    Merge_Log                       On
    Keep_Log                        Off
    Labels                          On
    Annotations                     On
    Kube_meta_preload_cache_dir     /var/kube_meta
```

Then a sidecar container should be added to Fluent Bit pod:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluent-bit
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: fluent-bit
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app.kubernetes.io/name: fluent-bit
    spec:
      containers:
        - name: fluent-bit
          image: fluent/fluent-bit:latest
          command: 
            - "/fluent-bit/bin/fluent-bit"
          args: 
            - "-c"
            - "/fluent-bit/etc/fluent-bit.conf"
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - name: varlog
              mountPath: /var/log
            - name: varlibdockercontainers
              mountPath: /var/lib/docker/containers
              readOnly: true
            - name: k8s-meta
              mountPath: /var/kube_meta
              readOnly: true
        - name: pod-meta-exporter
          image: shirolimit/pod-meta-exporter:latest
          imagePullPolicy: IfNotPresent
          env:
            - name: EXPORTER_NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: EXPORTER_RETENTION_PERIOD
              value: "5m"
            - name: EXPORTER_DESTINATION_DIR
              value: "/var/kube_meta"
          volumeMounts:
            - name: k8s-meta
              mountPath: /var/kube_meta
      volumes:
        - name: varlog
          hostPath:
            path: /var/log
        - name: varlibdockercontainers
          hostPath:
            path: /var/lib/docker/containers
        - name: k8s-meta
          emptyDir: {}
      serviceAccountName: fluent-bit-service-account
```
