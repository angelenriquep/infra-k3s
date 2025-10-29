# ArgoCD Installation and Configuration

## Deployment Instructions

### 1. Install ArgoCD on your K3s cluster

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

### 2. Access ArgoCD UI

```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

- User: `admin`
- Password: `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d`

### 3. Configure your Git repository

- Create a repo: <https://github.com/your-username/infra-k3s>
- Upload this entire directory to the repo
- Update URLs in YAML files (search for "your-username")

### 4. Deploy the ArgoCD project

```bash
kubectl apply -f argocd/projects/api-gateway-project.yaml
```

### 5. Deploy the bootstrap application (App of Apps)

```bash
kubectl apply -f argocd/bootstrap/root-app.yaml
```

### 6. Verify in ArgoCD UI

- Go to Applications
- You should see: bootstrap, api-gateway-dev, api-gateway-prod

## Repository Structure

```text
infra-k3s/
├── manifests/                ← Kubernetes manifests
│   ├── base/
│   │   ├── kustomization.yaml
│   │   ├── backend-api.yaml
│   │   ├── frontend-deployment-service.yaml
│   │   ├── gateway-infrastructure.yaml
│   │   ├── httproutes.yaml
│   │   ├── nodeport-gateway.yaml
│   │   └── frontend/
│   │       ├── index.html
│   │       └── nginx.conf
│   └── overlays/
│       ├── development/
│       │   ├── kustomization.yaml
│       │   ├── replica-patch.yaml
│       │   └── resource-patch.yaml
│       └── production/
│           ├── kustomization.yaml
│           ├── replica-patch.yaml
│           ├── resource-patch.yaml
│           └── security-patch.yaml
├── argocd/                   ← ArgoCD Configuration
│   ├── applications/
│   │   ├── kustomization.yaml
│   │   ├── api-gateway-dev.yaml
│   │   └── api-gateway-prod.yaml
│   └── projects/
│       └── api-gateway-project.yaml
│   ├── bootstrap/            ← App of Apps
│   └── root-app.yaml
├── backend/                  ← Backend Go application
│   ├── Dockerfile
│   ├── build-and-deploy.sh
│   └── src/
│       ├── go.mod
│       └── main.go
└── README.md                 ← Documentation
```

## GitOps Workflow

### 1. Development

```bash
git checkout -b feature/new-feature
# Make changes in manifests/
git commit -m "feat: add new feature"
git push origin feature/new-feature
# ArgoCD detects and automatically deploys to development
```

### 2. Production

```bash
git checkout main
git merge feature/new-feature
git push origin main
# ArgoCD detects but REQUIRES manual sync for production
```

## Useful Commands

### View application status

```bash
kubectl get applications -n argocd
```

### Force sync from CLI

```bash
argocd app sync api-gateway-dev
argocd app sync api-gateway-prod
```

### View ArgoCD logs

```bash
kubectl logs -n argocd deployment/argocd-application-controller
```

## Important Notes

- **Development**: Auto-sync enabled (prune + selfHeal)
- **Production**: Manual sync required (security)
- **App of Apps pattern**: Bootstrap manages all apps
- **RBAC**: Configured with admin/developer roles
- **Notifications**: Configured for Slack (optional)
