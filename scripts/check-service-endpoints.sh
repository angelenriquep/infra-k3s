#!/bin/bash

# Service Endpoint Health Monitor
# This script checks if NodePort services have healthy endpoints
# and validates that service selectors match available pod labels

set -e

NAMESPACE="${1:-envoy-gateway-system}"
SERVICE="${2:-envoy-gateway-nodeport}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔍 Checking service endpoints for $SERVICE in namespace $NAMESPACE..."

# Check if service exists
if ! kubectl get service "$SERVICE" -n "$NAMESPACE" &>/dev/null; then
    echo -e "${RED}❌ ERROR: Service $SERVICE not found in namespace $NAMESPACE${NC}"
    exit 1
fi

# Get service endpoints
ENDPOINTS=$(kubectl get endpoints "$SERVICE" -n "$NAMESPACE" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null)

if [ -z "$ENDPOINTS" ]; then
    echo -e "${RED}❌ ERROR: Service $SERVICE has no endpoints!${NC}"
    echo ""
    echo "🔍 Diagnosing the issue..."
    
    # Get service selector
    echo -e "${YELLOW}Service selector:${NC}"
    SELECTOR=$(kubectl get service "$SERVICE" -n "$NAMESPACE" -o jsonpath='{.spec.selector}')
    echo "$SELECTOR" | jq . 2>/dev/null || echo "$SELECTOR"
    
    echo ""
    echo -e "${YELLOW}Available pods in namespace:${NC}"
    kubectl get pods -n "$NAMESPACE" --show-labels
    
    echo ""
    echo -e "${YELLOW}Pods matching app.kubernetes.io/name=envoy:${NC}"
    kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=envoy" --show-labels 2>/dev/null || echo "No pods found"
    
    echo ""
    echo -e "${YELLOW}Suggested fixes:${NC}"
    echo "1. Check if the service selector matches the pod labels exactly"
    echo "2. Verify that kustomize commonLabels aren't affecting external services"
    echo "3. Consider moving external services to a separate kustomization without commonLabels"
    echo "4. Restart the pods if they exist but endpoints aren't updating"
    
    exit 1
else
    echo -e "${GREEN}✅ Service $SERVICE has endpoints: $ENDPOINTS${NC}"
    
    # Additional health checks
    echo ""
    echo "📊 Additional health information:"
    
    # Check endpoint count
    ENDPOINT_COUNT=$(echo "$ENDPOINTS" | wc -w)
    echo "   Endpoint count: $ENDPOINT_COUNT"
    
    # Check service type
    SERVICE_TYPE=$(kubectl get service "$SERVICE" -n "$NAMESPACE" -o jsonpath='{.spec.type}')
    echo "   Service type: $SERVICE_TYPE"
    
    if [ "$SERVICE_TYPE" = "NodePort" ]; then
        NODE_PORT=$(kubectl get service "$SERVICE" -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}')
        echo "   NodePort: $NODE_PORT"
        
        # Test NodePort connectivity (optional)
        if command -v curl &> /dev/null; then
            NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
            echo "   Testing connectivity to $NODE_IP:$NODE_PORT..."
            if curl -s --max-time 5 "http://$NODE_IP:$NODE_PORT" > /dev/null; then
                echo -e "   ${GREEN}✅ NodePort is accessible${NC}"
            else
                echo -e "   ${YELLOW}⚠️  NodePort test failed (this might be expected depending on the service)${NC}"
            fi
        fi
    fi
    
    # Check pod health
    echo ""
    echo "🏥 Pod health status:"
    kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=envoy" -o custom-columns="NAME:.metadata.name,STATUS:.status.phase,READY:.status.conditions[?(@.type=='Ready')].status" 2>/dev/null || echo "   No matching pods found"
fi

echo ""
echo -e "${GREEN}✅ Health check completed${NC}"
