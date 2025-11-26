# ArgoCD Setup for API Gateway

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
http://192.168.88.243:30080/frontend/
http://192.168.88.243:30080/api
http://192.168.88.243:30080/
http://192.168.88.243:31104/login
http://192.168.88.243:32080/login
http://192.168.88.243:30901/browser/postgresql-backups
```
