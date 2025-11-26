# Complete Network Flows & Kustomize Label Management

## Overview

This document provides a comprehensive explanation of network flows in our K3s cluster and addresses the critical issue of **Kustomize commonLabels** affecting service selectors, particularly with externally-managed resources like Envoy Gateway.

## Table of Contents

1. [Network Flow Architecture](#network-flow-architecture)
2. [The Label Mismatch Problem](#the-label-mismatch-problem)
3. [Understanding Kustomize commonLabels](#understanding-kustomize-commonlabels)
4. [Solutions & Prevention Strategies](#solutions--prevention-strategies)
5. [Monitoring & Alerting](#monitoring--alerting)
6. [Best Practices](#best-practices)

---

## Network Flow Architecture

### External Request Flow
```
[Client] 
    ↓ HTTP Request (port 30080)
[K3s Node:30080] (NodePort)
    ↓
[envoy-gateway-nodeport Service]
    ↓ (selector matches pod labels)
[Envoy Proxy Pod] (port 10080)
    ↓ (routes based on HTTPRoute rules)
[backend-api Service] (ClusterIP)
    ↓
[Backend Pod(s)] (port 8080)
    ↓ (database queries)
[PostgreSQL CNPG Cluster]
```

### Internal Communication Flow
```
[Backend Pod] ←→ [PostgreSQL CNPG Service] ←→ [PostgreSQL Primary Pod]
                                         ↓
                                    [PostgreSQL Replica Pod(s)]
```

---

## The Label Mismatch Problem

### What Happened?

The **NodePort service selector** couldn't find the **Envoy Proxy pods** because Kustomize's `commonLabels` added extra labels to the service selector that the Envoy pods don't have.

### Problem Visualization

```
Service Selector (with commonLabels):        Envoy Pod Labels:
├─ app.kubernetes.io/name: envoy             ✓ app.kubernetes.io/name: envoy
├─ gateway.envoyproxy.io/owning-gateway-name: eg  ✓ gateway.envoyproxy.io/owning-gateway-name: eg
├─ gateway.envoyproxy.io/owning-gateway-namespace: default  ✓ gateway.envoyproxy.io/owning-gateway-namespace: default
├─ project: api-gateway                      ✗ (MISSING - added by kustomize)
└─ component: web                           ✗ (MISSING - added by kustomize)

Result: NO MATCH → Service can't route traffic to pods
```

### Current State (Fixed)

```
Service Selector (fixed):                   Envoy Pod Labels:
├─ app.kubernetes.io/name: envoy             ✓ app.kubernetes.io/name: envoy
├─ gateway.envoyproxy.io/owning-gateway-name: eg  ✓ gateway.envoyproxy.io/owning-gateway-name: eg
└─ gateway.envoyproxy.io/owning-gateway-namespace: default  ✓ gateway.envoyproxy.io/owning-gateway-namespace: default

Result: MATCH → Service successfully routes traffic
```

---

## Understanding Kustomize commonLabels

### How commonLabels Works

Kustomize's `commonLabels` feature automatically adds labels to **ALL** resources in the kustomization, including:
- **Pods** (via Deployment/ReplicaSet templates)
- **Services** (both metadata labels AND selectors)
- **ConfigMaps**
- **Secrets**
- **Other Kubernetes resources**

### Current Configuration

**File: `/kustomize/base/kustomization.yaml`**
```yaml
commonLabels:
  project: api-gateway
  component: web
```

### The Problem with External Resources

**Envoy Gateway pods** are managed by the **Envoy Gateway Controller**, not by our kustomization. They have their own label structure:

```bash
# Envoy pod labels (managed by Envoy Gateway)
app.kubernetes.io/component=proxy
app.kubernetes.io/managed-by=envoy-gateway
app.kubernetes.io/name=envoy
gateway.envoyproxy.io/owning-gateway-name=eg
gateway.envoyproxy.io/owning-gateway-namespace=default
```

When we apply a NodePort service through kustomize with `commonLabels`, the service selector gets ALL labels, including the kustomize-added ones that the external pods don't have.

---

## Solutions & Prevention Strategies

### Solution 1: Exclude Services from commonLabels (Recommended)

Create a separate kustomization for services that need to match external pods:

**Create: `/kustomize/base/external-services/kustomization.yaml`**
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../cluster/envoy-gateway/nodeport-gateway.yaml

# NO commonLabels here - prevents selector modification
commonAnnotations:
  managed-by: kustomize
  category: external-service
```

**Update: `/kustomize/base/kustomization.yaml`**
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  # Remove nodeport-gateway from here
  - cluster/cnpg/
  - cluster/monitoring/
  - cluster/envoy-gateway/gateway-infrastructure.yaml
  - cluster/envoy-gateway/httproutes.yaml
  # - cluster/envoy-gateway/nodeport-gateway.yaml  # REMOVED
  - backend/
  - frontend/
  - external-services/  # ADD THIS

commonLabels:
  project: api-gateway
  component: web
```

### Solution 2: Use Kustomize Patches

Override the selector specifically for the NodePort service:

**Create: `/kustomize/overlays/development/nodeport-selector-patch.yaml`**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: envoy-gateway-nodeport
  namespace: envoy-gateway-system
spec:
  selector:
    # Override to match only Envoy pod labels
    app.kubernetes.io/name: envoy
    gateway.envoyproxy.io/owning-gateway-name: eg
    gateway.envoyproxy.io/owning-gateway-namespace: default
```

**Update: `/kustomize/overlays/development/kustomization.yaml`**
```yaml
patchesStrategicMerge:
  - replica-patch.yaml
  - resource-patch.yaml
  - nodeport-patch.yaml
  - nodeport-selector-patch.yaml  # ADD THIS
```

### Solution 3: Apply External Services Directly

For critical external services, apply them directly to avoid kustomize processing:

```bash
kubectl apply -f kustomize/base/cluster/envoy-gateway/nodeport-gateway.yaml
```

### Solution 4: Modern Kustomize Approach

Use the newer `labels` field instead of deprecated `commonLabels`:

**Update: `/kustomize/base/kustomization.yaml`**
```yaml
# Replace commonLabels with labels
labels:
  - pairs:
      project: api-gateway
      component: web
    includeSelectors: false  # Don't modify service selectors
    includeTemplates: true   # Apply to pod templates
```

---

## Monitoring & Alerting

### 1. Service Endpoint Health Check

Create a monitoring script to verify service selectors:

**Create: `/scripts/check-service-endpoints.sh`**
```bash
#!/bin/bash

NAMESPACE="envoy-gateway-system"
SERVICE="envoy-gateway-nodeport"

echo "Checking service endpoints..."
ENDPOINTS=$(kubectl get endpoints $SERVICE -n $NAMESPACE -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null)

if [ -z "$ENDPOINTS" ]; then
    echo "❌ ERROR: Service $SERVICE has no endpoints!"
    echo "🔍 Checking service selector vs pod labels..."
    
    echo "Service selector:"
    kubectl get service $SERVICE -n $NAMESPACE -o jsonpath='{.spec.selector}' | jq .
    
    echo "Available pods with matching name label:"
    kubectl get pods -n $NAMESPACE -l "app.kubernetes.io/name=envoy" --show-labels
    
    exit 1
else
    echo "✅ Service $SERVICE has endpoints: $ENDPOINTS"
fi
```

### 2. Prometheus Alert Rule

```yaml
# Alert when service has no endpoints
groups:
- name: service.endpoints
  rules:
  - alert: ServiceNoEndpoints
    expr: kube_service_spec_type{type="NodePort"} and on(service,namespace) kube_endpoint_address_available == 0
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Service {{ $labels.service }} has no available endpoints"
      description: "NodePort service {{ $labels.service }} in namespace {{ $labels.namespace }} has no endpoints available"
```

### 3. ArgoCD Health Check

Add a health check to the ArgoCD application:

```yaml
# In argocd application spec
spec:
  syncPolicy:
    syncOptions:
    - CreateNamespace=true
    - PrunePropagationPolicy=foreground
  ignoreDifferences:
  - group: ""
    kind: Service
    name: envoy-gateway-nodeport
    namespace: envoy-gateway-system
    jsonPointers:
    - /spec/selector
```

---

## Best Practices

### 1. Label Strategy

**For Internal Resources (managed by kustomize):**
```yaml
commonLabels:
  project: api-gateway
  component: web
  version: v1.0.0
```

**For External Resources (managed by operators):**
- Use separate kustomizations without commonLabels
- Apply directly when necessary
- Use patches to override selectors

### 2. Documentation Requirements

For each external service integration:
1. Document which labels the external pods have
2. Verify service selectors match exactly
3. Add monitoring for endpoint availability
4. Create troubleshooting runbooks

### 3. Testing Strategy

```bash
# Test service selector matching
kubectl get pods -n envoy-gateway-system -l "app.kubernetes.io/name=envoy" --show-labels
kubectl get service envoy-gateway-nodeport -n envoy-gateway-system -o jsonpath='{.spec.selector}'

# Test connectivity
kubectl get endpoints envoy-gateway-nodeport -n envoy-gateway-system
```

### 4. Kustomize File Organization

```
kustomize/
├── base/
│   ├── kustomization.yaml           # Internal resources with commonLabels
│   ├── internal-services/           # Services that match our pods
│   └── external-services/           # Services that match external pods
│       └── kustomization.yaml       # NO commonLabels
├── overlays/
│   └── development/
│       ├── kustomization.yaml
│       └── patches/                 # Selector override patches
```

---

## Summary

The **Kustomize commonLabels** feature is powerful but can break service selectors when working with externally-managed resources. The key is to:

1. **Separate concerns**: Keep external service configurations separate from internal ones
2. **Monitor endpoints**: Ensure services always have available endpoints
3. **Test thoroughly**: Verify label matching after any kustomize changes
4. **Document dependencies**: Clearly document which services depend on external pod labels

By following these practices, we can prevent similar issues and maintain reliable network flows in our K3s cluster.
