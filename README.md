# ArgoCD Setup for API Gateway

1) This set up uses VMs:

```bash
ssh angel@192.168.88.238
sudo virsh list --all

sudo virsh start k3s-node
sudo virsh autostart k3s-node
```

## 1. Installation

Client Version: v1.32.7
Kustomize Version: v5.5.0
Server Version: v1.33.5+k3s1

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

## 2. URLS - Load Balaced

```bash
ssh k3s
http://192.168.88.243:30080/ (BE Service)
http://192.168.88.243:32080/login (Argo CD)
```

## 3. Local DNS (`/etc/hosts`)

```bash
sudo vi /etc/hosts
```

```text
192.168.88.243  api.homelab.local
192.168.88.243  argocd.homelab.local
192.168.88.243  grafana.homelab.local
```

## Pending to add:

metalLB - l3, l4
Metric server - node metrics 
Vault - secrets 
Rookceph - storage 
Prometheus - monitoring 
Grafana - visualization 
Loki - logs
Gitlab - source code
Authentik - ldap


```
Internet
   │
   ▼
┌─────────────────────────────────────────┐
│ MetalLB (Load Balancer)                 │
│ Asigna IP: 192.168.88.250               │
│ (capa de red: L3/L4)                    │
└────────────┬────────────────────────────┘
             │ tráfico L4
             ▼
┌─────────────────────────────────────────┐
│ Envoy Gateway (api-gateway)             │
│ Enruta HTTP/HTTPS por hostname/path     │
│ (capa de aplicación: L7)                │
└────────────┬────────────────────────────┘
             │ tráfico HTTP
             ▼
        Backend Services
```
