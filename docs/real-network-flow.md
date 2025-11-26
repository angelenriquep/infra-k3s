# 🎯 Flujo Real de Request al Backend - Con IPs Actuales

## 🌐 Diagrama con IPs Reales de tu Cluster

```ascii
                                🖥️  CLIENTE
                        ┌─────────────────────────┐
                        │ IP: 192.168.88.250      │
                        │ curl/browser            │
                        │ URL: :30080/            │
                        └─────────────────────────┘
                                    │ 
                         HTTP GET / HTTP/1.1
                         Host: 192.168.88.243:30080
                                    │
                                    ▼
                        ┌─────────────────────────┐
                        │   🖧 K3S NODE           │
                        │ IP: 192.168.88.243      │
                        │ Port: 30080 (NodePort)  │
                        │ iptables/kube-proxy     │
                        └─────────────────────────┘
                                    │
                          DNAT: :30080 → 10.42.0.14:10080
                                    │
                                    ▼
                        ┌─────────────────────────┐
                        │ 🔌 NodePort Service     │
                        │ envoy-gateway-nodeport  │
                        │ ClusterIP: 10.43.90.71  │
                        │ NodePort: 30080         │
                        │ TargetPort: 10080       │
                        └─────────────────────────┘
                                    │
                            Service Discovery
                            Routes to Envoy Pod
                                    │
                                    ▼
                        ┌─────────────────────────┐
                        │ 🚪 ENVOY PROXY POD      │
                        │ Pod IP: 10.42.0.14      │
                        │ Container Port: 10080   │
                        │ Name: envoy-default-eg  │
                        │                         │
                        │ 📋 HTTPRoute Logic:     │
                        │ ✅ Match: PathPrefix "/" │
                        │ 🎯 Route to: backend    │
                        └─────────────────────────┘
                                    │ ┌─────────────────────────────────────┐
                          Forward to backend service    │ 🔄 gRPC Control Connection        │
                          Source: 10.42.0.14          │ Envoy ←→ Envoy Gateway            │
                          Dest: backend:3000           │ Source: 10.42.0.14               │
                                    │                  │ Dest: 10.42.0.11:18000 (EG Pod)  │
                                                      │ Protocol: xDS/gRPC                │
                                                      │ Updates: Routes, Clusters, etc    │
                                                      └─────────────────────────────────────┘
                                    ▼
                        ┌─────────────────────────┐
                        │ 🎯 BACKEND SERVICE      │
                        │ ClusterIP: 10.43.230.157│
                        │ Port: 3000              │
                        │ TargetPort: 3000        │
                        │ Selector: app=backend   │
                        └─────────────────────────┘
                                    │
                            kube-proxy routing
                            Load balance to pod
                                    │
                                    ▼
                        ┌─────────────────────────┐
                        │ 📦 BACKEND POD          │
                        │ Pod IP: 10.42.0.11      │
                        │ Container Port: 3000    │
                        │ Image: echo-basic       │
                        │                         │
                        │ 🔄 Processes request    │
                        │ 📤 Returns JSON         │
                        └─────────────────────────┘
```

## 🎛️ Envoy Gateway Control Plane Connection

### 📡 gRPC xDS Connection - What "Control Plane Manages gRPC" Means

The diagram above shows the data plane traffic flow, but there's also a critical **control plane connection**:

```text
┌─────────────────────────┐       gRPC xDS API        ┌─────────────────────────┐
│ 🚪 ENVOY PROXY POD      │◄────────────────────────►│ 🎛️ ENVOY GATEWAY POD    │
│ Pod IP: 10.42.0.6       │                          │ Pod IP: 10.42.0.5       │
│ Port: 10080 (data)      │   Discovery Requests:    │ Port: 18000 (xDS gRPC)  │
│                         │   • RouteConfiguration    │ Port: 18001 (rate limit)│
│ 📋 Configuration:       │   • ClusterLoadAssignment│ Port: 18002 (ext auth)  │
│ ✅ Routes from HTTPRoute│   • Listener updates      │                         │
│ ✅ Backend clusters     │   • Endpoint discovery    │ 🔧 Control Functions:   │
│ ✅ Load balancing       │                          │ ✅ HTTPRoute processing │
└─────────────────────────┘                          │ ✅ Gateway management   │
                                                     │ ✅ Certificate handling │
                                                     └─────────────────────────┘
```

### 🤔 What Does "Control Plane Manages gRPC" Mean?

**It means the Envoy Gateway control plane:**

1. **Runs a gRPC Server** (port 18000) that serves Envoy's xDS (Discovery Service) APIs
2. **Translates Kubernetes Resources** into Envoy configuration 
3. **Pushes Configuration** to Envoy Proxy pods via gRPC
4. **Manages the gRPC Connection State** between itself and all Envoy proxies

**It does NOT mean:**
- ❌ It manages your application's gRPC traffic (that would be data plane)
- ❌ It's a gRPC proxy for your services
- ❌ It handles gRPC load balancing for your apps

### 🔄 Control Plane Flow - Step by Step

1. **Envoy Gateway watches Kubernetes APIs** 📡
   - Gateway objects (listeners, ports, TLS)
   - HTTPRoute objects (routing rules, matches)  
   - Service objects (backend endpoints, health)

2. **Translation** 🔄
   - HTTPRoute → Envoy RouteConfiguration  
   - Service → Envoy Cluster + Endpoints
   - Gateway → Envoy Listener

3. **gRPC xDS Server (Port 18000)** 📤
   - LDS (Listener Discovery Service)
   - RDS (Route Discovery Service)  
   - CDS (Cluster Discovery Service)
   - EDS (Endpoint Discovery Service)

4. **Config Push & ACK** ⚡
   - Envoy Proxy: "Give me routes for gateway 'eg'"
   - Gateway: "Here's RouteConfiguration with backend:3000"
   - Proxy: "ACK - configuration applied"

### 🚨 When Control Plane Fails
- ❌ No config updates → Routes don't work
- ❌ Pod `1/2` Ready → No service endpoints  
- ❌ External traffic fails → Connection timeout

### 🚨 Control Plane Issues (What We Just Fixed)

**Problem**: `gRPC config stream to xds_cluster closed: Connection refused`

**Impact**: Pod `1/2` Ready → No endpoints → External access broken

**Solution**: Restart Envoy Proxy pod to re-establish gRPC connection

## 🔍 Transformaciones de IP en Cada Paso

### 📍 **Paso 1: Cliente → Nodo K3s**
```
Source IP: 192.168.88.250    (tu máquina)
Dest IP:   192.168.88.243    (K3s node)
Dest Port: 30080             (NodePort)
Protocol:  TCP/HTTP
```

### 📍 **Paso 2: Nodo K3s → Envoy Proxy**
```
Source IP: 192.168.88.250    (preservada)
Dest IP:   10.42.0.14        (Envoy Pod IP)
Dest Port: 10080             (Envoy listener)
Transform: DNAT por iptables
```

### 📍 **Paso 3: Envoy → Backend Service**
```
Source IP: 10.42.0.14        (Envoy Pod)
Dest IP:   10.43.230.157     (backend ClusterIP)
Dest Port: 3000              (backend service port)
Headers:   + X-Forwarded-* + Envoy headers
```

### 📍 **Paso 4: Service → Backend Pod**
```
Source IP: 10.42.0.14        (Envoy Pod - preservada)
Dest IP:   10.42.0.11        (backend Pod IP)
Dest Port: 3000              (container port)
Final:     iptables DNAT por kube-proxy
```

## 🕵️ Headers que Ve el Backend Pod

```json
{
  "headers": {
    "host": "192.168.88.243:30080",
    "user-agent": "curl/7.68.0",
    "accept": "*/*",
    "x-forwarded-for": "192.168.88.250",
    "x-forwarded-proto": "http",
    "x-envoy-internal": "true",
    "x-request-id": "550e8400-e29b-41d4-a716-446655440000"
  },
  "pod": {
    "name": "backend-869c8646c5-685q6",
    "ip": "10.42.0.11",
    "node": "k3s"
  }
}
```

## 🔄 Response Path (Inverso)

```
📦 Backend Pod (10.42.0.11)
        ↓ HTTP 200 + JSON body
🎯 Backend Service (10.43.230.157)  
        ↓ Routes back to source
🚪 Envoy Proxy (10.42.0.14)
        ↓ Applies response processing
🔌 NodePort Service (10.43.90.71)
        ↓ iptables SNAT
🖧 K3s Node (192.168.88.243)
        ↓ Network routing
🖥️  Cliente (192.168.88.250)
```

## 🧪 Verificar el Flujo

Puedes tracear este flujo exacto:

```bash
# 1. Hacer request y ver response headers
curl -v http://192.168.88.243:30080/

# 2. Ver logs del Envoy proxy
kubectl logs -n envoy-gateway-system envoy-default-eg-e41e7b31-5c4d7f6665-h6g5g -c envoy

# 3. Ver logs del backend pod  
kubectl logs backend-869c8646c5-685q6

# 4. Ver iptables rules (en el nodo K3s)
# sudo iptables -t nat -L KUBE-NODEPORTS -n
```

## 🎯 Puntos Clave del Flujo

1. **NodePort (30080)** → Envoy listener (10080)
2. **Envoy Pod (10.42.0.14)** procesa HTTPRoute
3. **Service ClusterIP (10.43.230.157)** load-balances  
4. **Backend Pod (10.42.0.11)** recibe request final
5. **Response** sigue el path inverso

La **IP original del cliente (192.168.88.250)** se preserva en headers `X-Forwarded-For` para que el backend la pueda ver.
