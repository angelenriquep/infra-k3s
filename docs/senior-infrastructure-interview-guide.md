# Senior/Staff Infrastructure Interview Guide

Deep technical answers to advanced Kubernetes, networking, security, and infrastructure questions.

---

## Table of Contents

1. [Fine-Grained Service Discovery (Envoy/Istio)](#1-fine-grained-service-discovery-across-1000-microservices)
2. [eBPF + Cilium vs Traditional CNIs](#2-ebpf--cilium-vs-traditional-cnis)
3. [Multi-Cloud Routing, IAM, and Secret Syncing](#3-multi-cloud-routing-iam-and-secret-syncing)
4. [systemd Failures on EKS Nodes](#4-intermittent-systemd-unit-failures-on-eks-nodes)
5. [Custom AMI Vetting Strategy](#5-custom-ami-vetting-strategy)
6. [Advanced Kube-Probes for Business Logic](#6-advanced-kube-probes-for-business-logic-failures)
7. [DNS-Level Outages in a Service Mesh](#7-dns-level-outages-inside-a-service-mesh)
8. [Terraform remote_state Timeout Recovery](#8-terraform-remote_state-backend-timeout-recovery)

---

## 1. Fine-Grained Service Discovery Across 1000+ Microservices

**Problem:** At 1000+ services, you can't push the full service catalog to every sidecar — it explodes memory and xDS push latency.

### Strategy: Scoped xDS + Sidecar Resource

```yaml
# Istio: Limit what each sidecar knows about
apiVersion: networking.istio.io/v1beta1
kind: Sidecar
metadata:
  name: backend-api
  namespace: payments
spec:
  egress:
    - hosts:
        - "istio-system/*"           # control plane
        - "payments/*"               # same namespace
        - "billing/*.billing-svc"    # specific cross-ns service
        - "~/*.googleapis.com"       # external
```

**Without this:** Each Envoy gets endpoints for ALL 1000+ services → huge memory, slow config pushes.

### Architecture at Scale

```
                    ┌─────────────────────┐
                    │      Istiod          │
                    │  (control plane)     │
                    └──────┬──────────────┘
                           │ xDS (gRPC streaming)
              ┌────────────┼────────────────┐
              ▼            ▼                ▼
        ┌──────────┐ ┌──────────┐    ┌──────────┐
        │ Envoy    │ │ Envoy    │    │ Envoy    │
        │ sidecar  │ │ sidecar  │    │ sidecar  │
        │ (scoped) │ │ (scoped) │    │ (scoped) │
        └──────────┘ └──────────┘    └──────────┘
         payments      billing         shipping
```

### Key Techniques

| Technique | What It Does |
|---|---|
| **Sidecar CRD** | Limits each proxy's visibility to only needed services |
| **Namespace isolation** | `exportTo: ["."]` prevents services leaking across namespaces |
| **Incremental xDS (Delta)** | Istio 1.22+ sends only changes, not full state |
| **Headless services** | For gRPC — client-side LB directly to pod IPs |
| **Locality-aware routing** | Route to same-zone pods first, reduce cross-AZ latency |

### Detection of Stale Endpoints

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: payments-svc
spec:
  host: payments-svc
  trafficPolicy:
    outlierDetection:
      consecutive5xxErrors: 3       # eject after 3 failures
      interval: 10s
      baseEjectionTime: 30s
      maxEjectionPercent: 50
    connectionPool:
      http:
        h2UpgradePolicy: UPGRADE   # use HTTP/2 for all gRPC
```

> **Note:** Netflix doesn't use Istio. They use **Eureka** (custom service registry) + client-side discovery. Envoy/Istio is the Kubernetes-native equivalent.

---

## 2. eBPF + Cilium vs Traditional CNIs

### How Traditional CNIs Work (Flannel — used in k3s)

```
Packet: Pod A → Pod B

Pod A → veth → bridge → iptables (MANY rules) → VXLAN → eth0 → network
                              │
                    kube-proxy adds MORE iptables rules for Services
                              │
                    Calico/Flannel adds MORE for NetworkPolicy

Total: Hundreds to thousands of iptables rules, evaluated SEQUENTIALLY
```

### How eBPF + Cilium Works

```
Packet: Pod A → Pod B

Pod A → eBPF program (kernel hook) → decision in O(1) → Pod B
                  │
                  └── No iptables, no kube-proxy, no bridge
                      Everything happens in kernel space
```

### iptables vs eBPF Performance

```
iptables (Flannel/Calico):              eBPF (Cilium):
┌─────────────────────┐                ┌─────────────────────┐
│ Rule 1: match? no   │                │                     │
│ Rule 2: match? no   │                │  BPF hash map       │
│ Rule 3: match? no   │                │  lookup: O(1)       │
│ ...                  │                │  → allow/deny       │
│ Rule 4782: match? YES│                │                     │
│                      │                │  (single kernel     │
│ O(n) per packet      │                │   program)          │
└─────────────────────┘                └─────────────────────┘
    Slow at scale                          Fast at any scale
```

### Cilium NetworkPolicy (L3/L4/L7)

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: backend-api-policy
spec:
  endpointSelector:
    matchLabels:
      app: backend-api
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: envoy-gateway
      toPorts:
        - ports:
            - port: "8080"
          rules:
            http:                          # L7 — iptables CAN'T do this
              - method: "GET"
                path: "/api/v1/.*"
              - method: "POST"
                path: "/api/v1/orders"
  egress:
    - toFQDNs:                             # DNS-aware — iptables CAN'T do this
        - matchName: "postgres-cnpg.database.svc.cluster.local"
      toPorts:
        - ports:
            - port: "5432"
```

### Feature Comparison

| Feature | Flannel + iptables | Calico (iptables) | **Cilium (eBPF)** |
|---|---|---|---|
| **L3/L4 policy** | ❌ (needs Calico) | ✅ | ✅ |
| **L7 policy** (HTTP path/method) | ❌ | ❌ | ✅ |
| **FQDN-based egress** | ❌ | Partial | ✅ |
| **Performance at 1000+ rules** | Degrades | Degrades | Constant |
| **kube-proxy replacement** | ❌ | ❌ | ✅ |
| **Observability (Hubble)** | ❌ | ❌ | ✅ (full flow logs) |
| **Transparent encryption** | ❌ | WireGuard | WireGuard/IPsec |
| **Kernel dependency** | None | None | Linux 5.10+ |

### When to Choose What

- **Flannel** (our k3s) → simple, works, fine for homelab/small clusters
- **Calico** → need NetworkPolicy but don't need L7
- **Cilium** → production at scale, need L7 policies, observability, performance

---

## 3. Multi-Cloud Routing, IAM, and Secret Syncing

### Cross-Cloud Routing

```
                    ┌──── Global DNS (Route53 / Cloud DNS) ────┐
                    │         Latency-based routing             │
                    ▼                                           ▼
            ┌──────────────┐                          ┌──────────────┐
            │   AWS Region  │                          │   GCP Region  │
            │  us-east-1    │◄── VPN/Interconnect ──► │  us-central1  │
            │               │    (dedicated link)      │               │
            │  ┌──────────┐ │                          │  ┌──────────┐ │
            │  │ Istio    │ │   mTLS + Locality LB     │  │ Istio    │ │
            │  │ Gateway  │ │◄────────────────────────►│  │ Gateway  │ │
            │  └──────────┘ │                          │  └──────────┘ │
            └──────────────┘                          └──────────────┘
                    │                                           │
              East-West mesh                             East-West mesh
```

| Layer | Solution |
|---|---|
| **DNS routing** | Global LB (Route53 weighted/latency) or Envoy global LB |
| **Network connectivity** | Cloud Interconnect / VPN / Tailscale for hybrid |
| **Service mesh** | Istio multi-cluster (shared trust domain, different networks) |
| **Traffic splitting** | Istio VirtualService → route 90% AWS, 10% GCP |

### Cross-Cloud IAM: Workload Identity Federation

```
┌─────────────┐      OIDC token      ┌─────────────────┐
│ K8s Pod      │ ──────────────────► │ AWS STS /        │
│ (GCP or AWS) │     (ServiceAccount) │ GCP Workload     │
└─────────────┘                      │ Identity         │
                                     │ Federation       │
                                     └────────┬────────┘
                                              │
                                     Short-lived cloud
                                     credentials (no
                                     long-lived keys!)
```

```yaml
# AWS: IRSA (IAM Roles for Service Accounts)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: backend-api
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456:role/backend-api

# GCP: Workload Identity
    iam.gke.io/gcp-service-account: backend-api@project.iam.gserviceaccount.com
```

### Secret Syncing with External Secrets Operator

```
┌────────────────┐         ┌────────────────┐
│ AWS Secrets     │         │ GCP Secret      │
│ Manager         │         │ Manager         │
└───────┬────────┘         └───────┬────────┘
        │                          │
        ▼                          ▼
┌─────────────────────────────────────────┐
│     External Secrets Operator (ESO)      │
│     (runs in both clusters)              │
└───────────────────┬─────────────────────┘
                    │
                    ▼
            K8s Secret (synced)
```

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: db-credentials
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager    # or gcp-secret-manager
  target:
    name: db-credentials
  data:
    - secretKey: password
      remoteRef:
        key: prod/db/password
```

---

## 4. Intermittent systemd Unit Failures on EKS Nodes

### Common Failures and Root Causes

| systemd Unit | Symptom | Root Cause |
|---|---|---|
| `kubelet.service` | Pods not scheduling | OOM, certificate expiry, disk pressure |
| `containerd.service` | Pods stuck `ContainerCreating` | Disk full, socket crash |
| `systemd-resolved` | DNS failures in pods | resolv.conf loop |
| `amazon-ssm-agent` | Can't SSH / manage node | Agent crash |

### Detection

```bash
# Node-level monitoring (DaemonSet)
journalctl -u kubelet --since "10 min ago" | grep -i "failed\|error\|oom"
systemctl list-units --state=failed
```

### Automated Detection + Self-Healing with Node Problem Detector

```
┌─────────────────────────┐
│   Node Problem Detector  │  ← DaemonSet, detects kernel, systemd, disk issues
│   (k8s.io/node-problem-  │
│    detector)             │
└───────────┬─────────────┘
            │ sets Node Condition:
            │ "KubeletUnhealthy=True"
            ▼
┌─────────────────────────┐
│   Kured / Draino /       │  ← watches unhealthy conditions
│   Cluster Autoscaler     │
└───────────┬─────────────┘
            │
            ▼
     Cordon → Drain → Terminate node → ASG replaces it
```

### EKS-Specific Approach (Immutable Infrastructure)

```
EKS Managed Node Group
         │
         ▼
    ASG (Auto Scaling Group)
         │
         ├── EC2 health check (system-level)
         ├── ELB health check (app-level)
         └── Custom: Node Problem Detector
                     │
                     ▼
              Node marked NotReady
                     │
                     ▼
              Cluster Autoscaler terminates
              ASG launches fresh node
              (immutable infrastructure — don't fix, REPLACE)
```

### Quick Manual Recovery

```bash
# SSH to the node
ssh ec2-user@<node-ip>

# Check failed units
systemctl list-units --state=failed

# Restart kubelet
sudo systemctl restart kubelet

# Check containerd
sudo systemctl status containerd
sudo journalctl -u containerd --since "5 min ago"

# If disk pressure
df -h /var/lib/containerd
sudo crictl rmi --prune  # clean unused images
```

---

## 5. Custom AMI Vetting Strategy

### Pipeline

```
Developer requests    Packer builds     Automated          Promotion
custom AMI           AMI               testing            gates
     │                    │                 │                  │
     ▼                    ▼                 ▼                  ▼
┌─────────┐      ┌──────────────┐   ┌────────────────┐  ┌──────────┐
│ Packer   │ ──► │ AMI artifact  │──►│ Test pipeline   │──►│ Approved │
│ template │     │ (us-east-1)   │   │                 │  │ AMI      │
└─────────┘      └──────────────┘   │ 1. CIS benchmark│  └──────────┘
                                     │ 2. CVE scan     │
                                     │ 3. Kernel test  │
                                     │ 4. Runtime test │
                                     │ 5. K8s soak     │
                                     └────────────────┘
```

### Testing Layers

| Layer | Tool | What It Checks |
|---|---|---|
| **Kernel** | `kexec` boot test, `modprobe` checks | Required modules (overlay, br_netfilter, eBPF) |
| **Security** | CIS-CAT, `lynis`, Inspector | Hardening baseline compliance |
| **CVE** | Trivy (filesystem mode), Qualys | Known vulnerabilities in OS packages |
| **Container runtime** | `ctr`/`crictl` smoke test | containerd starts, can pull + run image |
| **Kubernetes** | Spin up single-node k3s | kubelet starts, pod scheduling works |
| **Soak test** | Run in staging node group 48h | Memory leaks, kernel panics, OOM events |

### Packer Post-Build Validation Script

```bash
#!/bin/bash
set -euo pipefail

# Kernel modules required for Kubernetes
for mod in overlay br_netfilter ip_tables xt_conntrack; do
  modprobe "$mod" || { echo "FAIL: missing kernel module $mod"; exit 1; }
done

# sysctl requirements
sysctl net.bridge.bridge-nf-call-iptables  | grep "= 1" || exit 1
sysctl net.ipv4.ip_forward                 | grep "= 1" || exit 1

# containerd is functional
systemctl start containerd
ctr image pull docker.io/library/busybox:1.36
ctr run --rm docker.io/library/busybox:1.36 test-container echo "OK"

# Trivy filesystem scan
trivy fs --exit-code 1 --severity CRITICAL /
```

### Golden AMI Lifecycle

```
Week 0: Base AMI (Amazon Linux 2023)
   │
   ├── + hardening (CIS L2)
   ├── + required packages (containerd, kubelet)
   ├── + monitoring agents
   │
   ▼
Week 0: Golden AMI v1.0 → scan → test → promote to prod
Week 2: Security patch → Golden AMI v1.1 → scan → test → promote
Week 4: Kernel update → Golden AMI v1.2 → scan → test → promote

Old AMIs: deregister after 90 days (enforce with AWS Config rule)
```

---

## 6. Advanced Kube-Probes for Business Logic Failures

### The Problem

```
Standard probe:                  Reality:
GET /healthz → 200 OK ✅        App returns 200 but:
                                  - Database connection pool exhausted
                                  - Message queue backed up 100k messages
                                  - Payment API returning errors
                                  - Cache hit rate dropped to 0%
```

### Solution: Multi-Layer Probe Configuration

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: backend-api
      # Liveness: "Is the process alive?" — restarts if fails
      livenessProbe:
        httpGet:
          path: /healthz/live
          port: 8080
        initialDelaySeconds: 10
        periodSeconds: 10
        failureThreshold: 3

      # Readiness: "Can this pod serve traffic?" — removes from Service
      readinessProbe:
        httpGet:
          path: /healthz/ready
          port: 8080
        periodSeconds: 5
        failureThreshold: 2

      # Startup: "Has the app finished initializing?"
      startupProbe:
        httpGet:
          path: /healthz/startup
          port: 8080
        failureThreshold: 30
        periodSeconds: 2
```

### Business Logic Health Endpoint (Go Example)

```go
// /healthz/ready — checks ALL dependencies
func readinessHandler(w http.ResponseWriter, r *http.Request) {
    checks := map[string]func() error{
        "database":      checkDB,
        "redis":         checkRedis,
        "message_queue": checkNATS,
        "disk_space":    checkDisk,
        "business":      checkBusinessLogic,
    }

    failures := make(map[string]string)
    for name, check := range checks {
        if err := check(); err != nil {
            failures[name] = err.Error()
        }
    }

    if len(failures) > 0 {
        w.WriteHeader(http.StatusServiceUnavailable) // 503
        json.NewEncoder(w).Encode(map[string]any{
            "status":   "unhealthy",
            "failures": failures,
        })
        return
    }
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func checkBusinessLogic() error {
    // Check error rate from last 60 seconds
    errorRate := metrics.GetErrorRate("last_60s")
    if errorRate > 0.5 { // >50% errors
        return fmt.Errorf("error rate too high: %.2f%%", errorRate*100)
    }

    // Check processing latency
    p99 := metrics.GetP99Latency("last_60s")
    if p99 > 5*time.Second {
        return fmt.Errorf("p99 latency too high: %v", p99)
    }

    // Check circuit breaker state
    if circuitBreaker.State() == "open" {
        return fmt.Errorf("circuit breaker open for payment-api")
    }

    return nil
}
```

### gRPC Health Probe

```yaml
readinessProbe:
  grpc:
    port: 50051
    service: "myapp.BackendService"  # per-service health in gRPC
  periodSeconds: 5
```

### Exec Probe (Complex Logic)

```yaml
readinessProbe:
  exec:
    command:
      - /bin/sh
      - -c
      - |
        # Check queue depth
        DEPTH=$(curl -s localhost:8080/metrics | grep queue_depth | awk '{print $2}')
        if [ "$DEPTH" -gt 10000 ]; then exit 1; fi

        # Check upstream dependency
        curl -sf --max-time 2 http://payment-api:8080/healthz || exit 1
```

---

## 7. DNS-Level Outages Inside a Service Mesh

### The Problem

```
Pod → CoreDNS → ??? (CoreDNS pods crashed or overloaded)
  │
  └── All service discovery breaks
      Even though the mesh (Envoy) is healthy
```

### Why This Is Tricky

Istio/Envoy uses **EDS** (Endpoint Discovery Service) from the control plane, NOT DNS. So pod-to-pod via the mesh should still work. But:

- External services (`googleapis.com`) need DNS
- Services outside the mesh need DNS
- Some apps resolve DNS at startup and cache it

### Mitigation Strategies

| Strategy | How |
|---|---|
| **NodeLocal DNSCache** | DaemonSet caches DNS on each node — survives CoreDNS outage |
| **Istio ServiceEntry** | Pre-register external services so Envoy doesn't need DNS |
| **DNS failover** | CoreDNS → NodeLocal → upstream (chain) |
| **Envoy STRICT_DNS** | Envoy re-resolves periodically, caches results |

### NodeLocal DNS Cache (Best Solution)

```
Without NodeLocal DNS:
Pod → CoreDNS (cluster IP, via kube-proxy) → upstream
       │
       └── Single point of failure, iptables hop

With NodeLocal DNS:
Pod → node-local-dns (169.254.20.10, on same node) → CoreDNS → upstream
       │
       └── Local cache, survives CoreDNS outage for cached entries
```

### Istio ServiceEntry (Bypass CoreDNS for Known Externals)

```yaml
apiVersion: networking.istio.io/v1beta1
kind: ServiceEntry
metadata:
  name: googleapis
spec:
  hosts:
    - "*.googleapis.com"
  ports:
    - number: 443
      name: https
      protocol: TLS
  resolution: DNS         # Envoy resolves, caches, and manages
  location: MESH_EXTERNAL
```

Now Envoy handles the DNS resolution and caching — even if CoreDNS is down, Envoy still has the resolved IPs cached.

### Emergency Recovery Without Redeploy

```bash
# 1. Check CoreDNS status
kubectl -n kube-system get pods -l k8s-app=kube-dns

# 2. If OOMKilled — increase resources
kubectl -n kube-system patch deployment coredns -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"coredns","resources":{"limits":{"memory":"256Mi"}}}]}}}}'

# 3. If stuck — force restart
kubectl -n kube-system rollout restart deployment coredns

# 4. Nuclear option — pods can use node's resolv.conf
# (hostNetwork pods or dnsPolicy: Default bypass CoreDNS entirely)
```

---

## 8. Terraform remote_state Backend Timeout Recovery

### The Scenario

```
terraform plan
│
└── Error: Failed to load state: RequestError: send request failed
    caused by: dial tcp s3.amazonaws.com:443: i/o timeout

State is in S3 + DynamoDB lock. S3 is timing out.
You can't plan OR apply. Infrastructure is frozen.
```

### Immediate Response (Minute 0-5)

```
┌─────────────────────────────────────────────┐
│  1. Notify all teams: "Terraform freeze"     │
│  2. No one runs plan/apply                   │
│  3. Check AWS status page                    │
│  4. Check VPC/NAT/DNS (maybe YOUR network)   │
└─────────────────────────────────────────────┘
```

### Diagnosis

```bash
# Is it S3 or your network?
curl -v https://s3.us-east-1.amazonaws.com
nslookup s3.us-east-1.amazonaws.com
traceroute s3.us-east-1.amazonaws.com

# Is it DynamoDB lock stuck?
aws dynamodb scan --table-name terraform-locks
# If a lock is stuck from a crashed CI job:
aws dynamodb delete-item \
  --table-name terraform-locks \
  --key '{"LockID": {"S": "my-state-bucket/terraform.tfstate-md5"}}'

# Is it the state file itself? (corruption)
aws s3 cp s3://my-state-bucket/terraform.tfstate ./terraform.tfstate.backup
```

### Recovery by Root Cause

| Root Cause | Recovery |
|---|---|
| **AWS S3 outage** | Wait, or switch to backup region (cross-region replication) |
| **Network issue** (NAT, VPN) | Fix network; TF can retry |
| **DynamoDB lock stuck** | Delete lock entry (after confirming no one else is running) |
| **State corruption** | Restore from S3 versioning: `aws s3api list-object-versions` |
| **CI runner can't reach S3** | Fix runner networking, or run locally |

### Prevention Architecture

```
┌──────────────────────────────────────────────────┐
│                 Terraform Backend                  │
│                                                    │
│  S3 Bucket (state)           DynamoDB (lock)       │
│  ├── Versioning: ON          ├── TTL on locks      │
│  ├── Cross-region replica    ├── Point-in-time     │
│  ├── MFA delete: ON          │   recovery: ON      │
│  └── Lifecycle: keep 90      └────────────────     │
│       versions                                     │
│                                                    │
│  Backend config:                                   │
│  ├── retry with backoff (TF_BACKEND_HTTP_RETRY)    │
│  ├── skip_metadata_api_check = true (if IMDS slow) │
│  └── multiple state files (split large states)     │
└──────────────────────────────────────────────────┘
```

### Break Glass Procedure

```bash
# If you MUST make changes and S3 is down:

# 1. Pull last known good state from cache/backup
cp .terraform/terraform.tfstate ./emergency.tfstate

# 2. Use local backend temporarily
terraform init -backend=false
terraform plan -state=emergency.tfstate

# 3. Apply ONLY if critical (outage-level)
terraform apply -state=emergency.tfstate

# 4. When S3 is back, push state
terraform init -migrate-state  # migrates local → S3

# 5. Verify
terraform plan  # should show "No changes"
```

### State Splitting (Prevention for Large Orgs)

```
# BAD: One massive state file
terraform/
  └── main.tf  (manages EVERYTHING)

# GOOD: Split by blast radius
terraform/
  ├── networking/     (VPC, subnets — rarely changes)
  ├── eks-cluster/    (EKS — changes weekly)
  ├── databases/      (RDS — sensitive, separate access)
  └── apps/           (K8s resources — changes daily)

Each has its own S3 key → one timeout doesn't block everything
```

---

## Quick Reference Summary

| Question | Key Insight |
|---|---|
| **Service discovery at scale** | Scope xDS with Sidecar CRD, don't push full catalog |
| **eBPF + Cilium** | O(1) lookups vs O(n) iptables, L7 policies, no kube-proxy |
| **Multi-cloud routing** | Workload Identity Federation, External Secrets Operator, Istio multi-cluster |
| **systemd failures on EKS** | Node Problem Detector + immutable nodes (don't fix, replace) |
| **Custom AMI vetting** | Packer → CIS scan → CVE scan → kernel test → soak test → promote |
| **Business logic probes** | Readiness checks dependencies + error rate + latency, not just HTTP 200 |
| **DNS outage in mesh** | NodeLocal DNS cache + Istio ServiceEntry bypass CoreDNS |
| **Terraform state timeout** | S3 versioning, state splitting, break-glass local state procedure |
