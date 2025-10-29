# ArgoCD Setup for API Gateway

## Complete Repository Structure

```
infra-k3s/
├── base/                     ← Base Kubernetes manifests
│   ├── kustomization.yaml
│   ├── backend/
│   │   └── backend-api.yaml
│   ├── frontend/
│   │   └── frontend-deployment-service.yaml
│   └── cluster/
│       ├── envoy-gateway/
│       ├── monitoring/
│       └── pgoperator/
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
└── README.md
```

## Deployment Steps

### 1. Install ArgoCD on K3s

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

### 2. Access ArgoCD UI

```bash
# Port forward
kubectl port-forward svc/argocd-server -n argocd 8080:443

kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

### 3. Configure Git Repository

- Create repo: `https://github.com/your-username/infra-k3s`
- Subir todo el directorio al repo
- Actualizar URLs en los archivos YAML

### 4. Deploy ArgoCD Apps

```bash
# Deploy project
kubectl apply -f argocd/projects/api-gateway-project.yaml

# Deploy bootstrap (App of Apps)
kubectl apply -f argocd/bootstrap/root-app.yaml
```

## Workflow GitOps

### Development
```bash
git checkout -b feature/new-feature
git commit -m "feat: add new feature"
git push origin feature/new-feature
# ArgoCD automatically deploys to development
```

### Production
```bash
git checkout main
git merge feature/new-feature
git push origin main
# ArgoCD detecta pero requiere sync manual para production
```

## Useful Commands

```bash
# View applications
kubectl get applications -n argocd

# Sync desde CLI
argocd app sync api-gateway-dev
argocd app sync api-gateway-prod

# View logs
kubectl logs -n argocd deployment/argocd-application-controller
```

## 🐳 Docker Images

### Backend Go API

Para construir y subir la imagen del backend:

```bash
# Navegar al directorio del backend
cd backend

# Construir la imagen Docker
docker build -t backend:v1.0.1 .

# Etiquetar para Docker Hub
docker tag backend:v1.0.1 mockingbird2912/api-gateway-backend:v1.0.1

# Login to Docker Hub (if not logged in)
docker login

# Subir la imagen
docker push mockingbird2912/api-gateway-backend:v1.0.1
```

### Complete command (single line)

```bash
cd backend && docker build -t backend:v1.0.1 . && docker tag backend:v1.0.1 mockingbird2912/api-gateway-backend:v1.0.1 && docker push mockingbird2912/api-gateway-backend:v1.0.1
```

### Actualizar imagen en Kubernetes

After uploading the new image, update the version in the deployment:

```bash
# Edit backend-api.yaml to change image version
# Then apply changes
cd kustomize/base
kubectl apply -f backend/backend-api.yaml

# O usar kustomize
kubectl apply -k kustomize/overlays/development
```

## 📊 Observability Stack

### Monitoring Components

- **Prometheus**: Metrics collection and storage
- **Grafana**: Dashboards and visualization
- **Fluent Bit**: Log aggregation and forwarding

### Access URLs

```bash
# Prometheus
http://192.168.88.243:30090

# Grafana
http://192.168.88.243:31104
# User: admin
# Password: admin123
```

### Recommended Grafana Dashboards

Import these dashboards using their IDs in Grafana:

| Dashboard | ID | Description |
|-----------|----|-----------| 
| **Kubernetes Cluster Monitoring** | `7249` | Complete cluster overview with pods, deployments, resources |
| **Node Exporter Full** | `1860` | Server metrics (CPU, memory, disk, network) |
| **PostgreSQL Database** | `9628` | Database performance and connection metrics |
| **Kubernetes cluster monitoring** | `315` | Alternative cluster monitoring (simpler) |

#### How to import

1. Go to Grafana → **+** → **Import**
2. Enter dashboard ID (e.g., `7249`)
3. Click **Load**
4. Select **Prometheus** as data source
5. Click **Import**

### Monitoring Stack Deployment

```bash
# Deploy monitoring stack
kubectl apply -f monitoring/prometheus.yaml
kubectl apply -f monitoring/grafana.yaml
kubectl apply -f monitoring/logging.yaml
```

## Configuration

- **Development**: Auto-sync habilitado
- **Production**: Sync manual requerido
- **RBAC**: Roles admin/developer configurados
- **Notificaciones**: Slack configurado (opcional)
