# ArgoCD Installation and Configuration
# 
# INSTRUCCIONES PARA DESPLEGAR:
#
# 1. Instalar ArgoCD en tu cluster K3s:
#    kubectl create namespace argocd
#    kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
#
# 2. Acceso a ArgoCD UI:
#    kubectl port-forward svc/argocd-server -n argocd 8080:443
#    # Usuario: admin
#    # Password: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
#
# 3. Configurar tu repositorio Git:
#    - Crea un repo: https://github.com/tu-usuario/infra-k3s
#    - Sube todo este directorio al repo
#    - Actualiza las URLs en los archivos YAML (busca "tu-usuario")
#
# 4. Desplegar el proyecto ArgoCD:
#    kubectl apply -f argocd/projects/api-gateway-project.yaml
#
# 5. Desplegar la aplicación bootstrap (App of Apps):
#    kubectl apply -f bootstrap/root-app.yaml
#
# 6. Verificar en la UI de ArgoCD:
#    - Ve a Applications
#    - Deberías ver: bootstrap, api-gateway-dev, api-gateway-prod
#
# ESTRUCTURA DEL REPOSITORIO:
#
# infra-k3s/
# ├── api-gateway/              ← Tu aplicación actual
# │   ├── base/
# │   │   ├── kustomization.yaml
# │   │   ├── backend-app.yaml
# │   │   ├── frontend-deployment-service.yaml
# │   │   ├── gateway-infrastructure.yaml
# │   │   ├── httproutes.yaml
# │   │   ├── nodeport-gateway.yaml
# │   │   └── frontend/
# │   │       ├── index.html
# │   │       └── nginx.conf
# │   └── overlays/
# │       ├── development/
# │       │   ├── kustomization.yaml
# │       │   ├── replica-patch.yaml
# │       │   └── resource-patch.yaml
# │       └── production/
# │           ├── kustomization.yaml
# │           ├── replica-patch.yaml
# │           ├── resource-patch.yaml
# │           └── security-patch.yaml
# ├── argocd/                   ← Configuración de ArgoCD
# │   ├── applications/
# │   │   ├── kustomization.yaml
# │   │   ├── api-gateway-dev.yaml
# │   │   └── api-gateway-prod.yaml
# │   └── projects/
# │       └── api-gateway-project.yaml
# ├── bootstrap/                ← App of Apps
# │   └── root-app.yaml
# └── README.md                 ← Documentación
#
# WORKFLOW GITOPS:
#
# 1. Desarrollo:
#    git checkout -b feature/new-feature
#    # Haz cambios en api-gateway/
#    git commit -m "feat: add new feature"
#    git push origin feature/new-feature
#    # ArgoCD detecta y despliega automáticamente a development
#
# 2. Producción:
#    git checkout main
#    git merge feature/new-feature
#    git push origin main
#    # ArgoCD detecta pero REQUIERE sync manual para production
#
# COMANDOS ÚTILES:
#
# Ver estado de aplicaciones:
# kubectl get applications -n argocd
#
# Forzar sync desde CLI:
# argocd app sync api-gateway-dev
# argocd app sync api-gateway-prod
#
# Ver logs de ArgoCD:
# kubectl logs -n argocd deployment/argocd-application-controller
#
# NOTAS IMPORTANTES:
#
# - Development: Auto-sync habilitado (prune + selfHeal)
# - Production: Sync manual requerido (seguridad)
# - App of Apps pattern: Bootstrap gestiona todas las apps
# - RBAC configurado con roles admin/developer
# - Notificaciones configuradas para Slack (opcional)

apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-setup-guide
  namespace: argocd
data:
  setup.md: |
    # ArgoCD Setup Guide
    
    Este ConfigMap contiene las instrucciones completas para configurar ArgoCD
    con tu proyecto API Gateway.
    
    Ver comentarios en este archivo para instrucciones detalladas.
