# 🐘 PostgreSQL Credentials for API Gateway

## 🔗 Connection Information

```bash
# Connection data
HOST: api-gateway-postgres.default.svc.cluster.local
PORT: 5432
DATABASE: api_gateway_db

# Application user
USERNAME: api_gateway_app
PASSWORD: 7lOLFeB51bpR24cCvUYYsxyTBhjp1K2vrQ8F87yj9v49h6hVFvz2HElKE40RFjyz

# Owner user (admin)
USERNAME: api_gateway_owner  
PASSWORD: $(kubectl get secret api-gateway-owner.api-gateway-postgres.credentials.postgresql.acid.zalan.do -o jsonpath='{.data.password}' | base64 -d)
```

## 🧪 Test Connection

### From a temporary pod:
```bash
kubectl run postgres-client --rm -i --tty --image postgres:16 -- \
  psql "postgresql://api_gateway_app:7lOLFeB51bpR24cCvUYYsxyTBhjp1K2vrQ8F87yj9v49h6hVFvz2HElKE40RFjyz@api-gateway-postgres.default.svc.cluster.local:5432/api_gateway_db"
```

### Port-forward for local connection:
```bash
kubectl port-forward svc/api-gateway-postgres 5432:5432
```

Then connect from your local machine:
```bash
psql "postgresql://api_gateway_app:7lOLFeB51bpR24cCvUYYsxyTBhjp1K2vrQ8F87yj9v49h6hVFvz2HElKE40RFjyz@localhost:5432/api_gateway_db"
```

## 📊 Estado del Cluster

```bash
# View PostgreSQL cluster status
kubectl get postgresql

# View cluster pods
kubectl get pods -l application=spilo,cluster-name=api-gateway-postgres

# View services
kubectl get services -l application=spilo,cluster-name=api-gateway-postgres

# View PostgreSQL logs
kubectl logs api-gateway-postgres-0
```

## 🔧 Environment Variables for Backend

To use in your backend application:

```yaml
env:
  - name: DB_HOST
    value: "api-gateway-postgres.default.svc.cluster.local"
  - name: DB_PORT
    value: "5432"
  - name: DB_NAME
    value: "api_gateway_db"
  - name: DB_USER
    value: "api_gateway_app"
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: api-gateway-app.api-gateway-postgres.credentials.postgresql.acid.zalan.do
        key: password
```
