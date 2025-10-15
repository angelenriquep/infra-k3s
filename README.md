# ArgoCD Setup para API Gateway

## Estructura Completa del Repositorio

```
infra-k3s/
├── api-gateway/              ← Tu aplicación actual
│   ├── base/
│   │   ├── kustomization.yaml
│   │   ├── backend-app.yaml
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
├── argocd/                   ← Configuración de ArgoCD
│   ├── applications/
│   │   ├── kustomization.yaml
│   │   ├── api-gateway-dev.yaml
│   │   └── api-gateway-prod.yaml
│   └── projects/
│       └── api-gateway-project.yaml
├── bootstrap/                ← App of Apps
│   └── root-app.yaml
└── README.md
```

## Pasos para Desplegar

### 1. Instalar ArgoCD en K3s

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

### 2. Acceso a ArgoCD UI

```bash
# Port forward
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Obtener password
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

### 3. Configurar Repositorio Git

- Crear repo: `https://github.com/tu-usuario/infra-k3s`
- Subir todo el directorio al repo
- Actualizar URLs en los archivos YAML

### 4. Desplegar ArgoCD Apps

```bash
# Desplegar proyecto
kubectl apply -f argocd/projects/api-gateway-project.yaml

# Desplegar bootstrap (App of Apps)
kubectl apply -f bootstrap/root-app.yaml
```

## Workflow GitOps

### Development
```bash
git checkout -b feature/new-feature
# Hacer cambios
git commit -m "feat: add new feature"
git push origin feature/new-feature
# ArgoCD despliega automáticamente a development
```

### Production
```bash
git checkout main
git merge feature/new-feature
git push origin main
# ArgoCD detecta pero requiere sync manual para production
```

## Comandos Útiles

```bash
# Ver aplicaciones
kubectl get applications -n argocd

# Sync desde CLI
argocd app sync api-gateway-dev
argocd app sync api-gateway-prod

# Ver logs
kubectl logs -n argocd deployment/argocd-application-controller
```

## Configuración

- **Development**: Auto-sync habilitado
- **Production**: Sync manual requerido
- **RBAC**: Roles admin/developer configurados
- **Notificaciones**: Slack configurado (opcional)
