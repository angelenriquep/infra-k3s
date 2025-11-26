# Troubleshooting: Envoy Gateway NodePort Service Issue

## Problem Summary

External access to the API through `http://192.168.88.243:30080/api` was broken after applying kustomize configuration during the PostgreSQL migration from Zalando to CNPG.

## Timeline

- **Date:** October 30, 2025
- **Duration:** ~45 minutes (10:50:28Z - 11:35:11Z)
- **Affected:** External API access through NodePort service
- **Impact:** Connection timeouts on external API calls

## Root Cause

The issue was caused by **Kustomize's `commonLabels` feature** automatically injecting additional labels into the `envoy-gateway-nodeport` service selector, which prevented it from matching the Envoy Gateway pod labels.

### Configuration Mismatch

**Original NodePort Service Selector (in YAML):**

```yaml
selector:
  app.kubernetes.io/name: envoy
  gateway.envoyproxy.io/owning-gateway-name: eg
  gateway.envoyproxy.io/owning-gateway-namespace: default
```

**Applied Selector (after kustomize processing):**

```yaml
selector:
  app.kubernetes.io/name: envoy
  gateway.envoyproxy.io/owning-gateway-name: eg
  gateway.envoyproxy.io/owning-gateway-namespace: default
  component: web          # ← Added by commonLabels
  project: api-gateway    # ← Added by commonLabels
```

**Actual Envoy Pod Labels (managed by Envoy Gateway):**

```yaml
labels:
  app.kubernetes.io/name: envoy
  gateway.envoyproxy.io/owning-gateway-name: eg
  gateway.envoyproxy.io/owning-gateway-namespace: default
  # Missing: component: web, project: api-gateway
```

## Symptoms

1. **Connection Timeouts:** `curl: (7) Failed to connect to 192.168.88.243 port 30080: Connection timed out`
2. **No Service Endpoints:** `kubectl get endpoints envoy-gateway-nodeport` showed `<none>`
3. **Envoy Pod Not Ready:** Initially showed `1/2` ready status due to control plane connection issues
4. **Internal Access Working:** Direct port-forward to backend service worked fine

## Diagnostic Steps

1. **Check Service Endpoints:**

   ```bash
   kubectl get endpoints envoy-gateway-nodeport -n envoy-gateway-system
   ```

2. **Verify Pod Labels vs Service Selector:**

   ```bash
   kubectl get pod -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=eg --show-labels
   kubectl get service envoy-gateway-nodeport -n envoy-gateway-system -o yaml
   ```

3. **Check Envoy Pod Logs:**

   ```bash
   kubectl logs <envoy-pod-name> -n envoy-gateway-system -c envoy --tail=20
   ```

## Solution

### Immediate Fix

Apply the NodePort service directly, bypassing kustomize's commonLabels:

```bash
kubectl apply -f kustomize/base/cluster/envoy-gateway/nodeport-gateway.yaml
```

This restores the correct selector without the extra labels.

### Long-term Solutions

#### Option 1: Exclude Service from Common Labels

Add the service to a separate kustomization file that doesn't use commonLabels.

#### Option 2: Remove Common Labels

Update `kustomize/base/kustomization.yaml`:

```yaml
# Remove or comment out:
# commonLabels:
#   project: api-gateway
#   component: web

# Use labels instead (as recommended by the deprecation warning):
labels:
  - pairs:
      project: api-gateway
      component: web
    includeSelectors: false  # This prevents labels from being added to selectors
```

#### Option 3: Use Labels with Selective Application

```yaml
labels:
  - pairs:
      project: api-gateway
      component: web
    includeSelectors: false
    includeTemplates: true
```

## Prevention

1. **Test External Access:** Always verify external endpoints after applying kustomize configurations
2. **Review commonLabels Impact:** Be cautious when using commonLabels with externally managed resources
3. **Use Staging Environment:** Test infrastructure changes in a staging environment first
4. **Monitor Service Endpoints:** Set up monitoring for service endpoint availability

## Verification Commands

After applying the fix, verify the solution:

```bash
# Check service has endpoints
kubectl get endpoints envoy-gateway-nodeport -n envoy-gateway-system

# Test external access
curl -s http://192.168.88.243:30080/api

# Verify envoy pod is fully ready
kubectl get pods -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=eg
```

## Related Issues

- **Kustomize commonLabels deprecation warning:** Switch to `labels` field
- **Envoy Gateway pod readiness:** May require pod restart if control plane connection is lost
- **Service selector immutability:** Kubernetes service selectors cannot be changed once created

## Files Affected

- `kustomize/base/kustomization.yaml` - Contains problematic commonLabels
- `kustomize/base/cluster/envoy-gateway/nodeport-gateway.yaml` - NodePort service configuration
- External API access endpoint: `http://192.168.88.243:30080/api`

## Lessons Learned

1. **Kustomize Common Labels** can interfere with externally managed resources
2. **Service selectors** must exactly match pod labels for endpoints to be populated
3. **Infrastructure as Code** requires careful testing of label and selector modifications
4. **External dependencies** (like Envoy Gateway) may not follow the same labeling conventions as application components

---

**Author:** GitHub Copilot  
**Date:** October 30, 2025  
**Context:** PostgreSQL migration from Zalando to CNPG
