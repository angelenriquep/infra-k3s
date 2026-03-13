# ArgoCD Setup for API Gateway

```bash
ssh angel@192.168.88.238
sudo virsh list --all

# Marcado como autostart en caso de que se reinicie la maquina
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

## 2. URLS - Port forwarded

Change the IP for your cluster IP

```bash
ssh k3s
http://192.168.88.243:30080/frontend/ (FE Service)
http://192.168.88.243:30080/api (BE Service)
http://192.168.88.243:30080/ (BE Service)
http://192.168.88.243:31104/login
http://192.168.88.243:32080/login (Argo CD)
http://192.168.88.243:30901/browser/postgresql-backups (Minio)
https://192.168.88.254/admin/login (Pi Hole)
```
