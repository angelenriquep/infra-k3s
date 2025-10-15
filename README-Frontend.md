# Frontend Architecture - Separated Files

## 📁 Structure

```
📁 api-gateway/
├── 📋 backend-app.yaml           # Backend deployment
├── 📋 gateway-infrastructure.yaml # Gateway + GatewayClass
├── 📋 httproutes.yaml            # Routing rules
├── 📋 nodeport-gateway.yaml      # External access
├── 📂 frontend/                  # Frontend source files
│   ├── 📄 index.html            # HTML source
│   └── 📄 nginx.conf            # Nginx config source
├── 🔧 build-frontend.sh         # Build script
└── 📋 frontend-app.yaml         # Generated K8s YAML
```

## 🔧 Development Workflow

### 1. Edit Source Files
- **HTML**: Edit `frontend/index.html`
- **Nginx Config**: Edit `frontend/nginx.conf`

### 2. Generate Kubernetes YAML
```bash
./build-frontend.sh
```

### 3. Apply to Kubernetes
```bash
kubectl apply -f frontend-app.yaml
```

## ✅ Benefits of This Structure

### **Before** (Monolithic YAML):
❌ HTML embedded in YAML  
❌ Hard to read and edit  
❌ No syntax highlighting for HTML  
❌ Difficult version control  

### **After** (Separated Files):
✅ Clean separation of concerns  
✅ Proper syntax highlighting  
✅ Easy to edit HTML/CSS/JS  
✅ Better version control  
✅ Reusable components  

## 🚀 Features of the New Frontend

### Enhanced UI:
- Better styling and responsive design
- Loading states and status indicators
- Success/error visual feedback
- Improved button interactions

### Enhanced Functionality:
- Better error handling
- Status indicators for each test
- More detailed POST request data
- Console logging for debugging

## 🛠️ Customization

### To add new features:
1. Edit `frontend/index.html`
2. Run `./build-frontend.sh`
3. Apply with `kubectl apply -f frontend-app.yaml`

### To modify nginx behavior:
1. Edit `frontend/nginx.conf`
2. Run `./build-frontend.sh`
3. Apply with `kubectl apply -f frontend-app.yaml`

## 📋 Access URLs

Both URLs work identically:
- `http://192.168.88.243:30080/frontend`
- `http://192.168.88.243:30080/frontend/`

## 🧪 Testing Features

The frontend provides three test endpoints:
1. **Simple Route**: `GET /` - Basic backend test
2. **Hostname Route**: `GET /api/` - With Host header
3. **POST Test**: `POST /` - JSON data submission

Each test shows:
- Loading state during request
- Success/error status with HTTP codes
- Formatted JSON response
- Visual feedback with color coding
