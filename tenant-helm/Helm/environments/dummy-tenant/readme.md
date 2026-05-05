# Dummy Tenant Helm Commands

Run from: `Helm/environments/dummy-tenant/`

## Template Backend

```bash
helm template dummy-ecommerce ../../charts/backend-common -f values-ecommerce.yaml
```

## Template Frontend

```bash
helm template dummy-frontend ../../charts/frontend -f values-frontend.yaml
```

## Deploy Backend

```bash
helm upgrade --install dummy-ecommerce ../../charts/backend-common -f values-ecommerce.yaml -n dummy-tenant
```

## Deploy Frontend

```bash
helm upgrade --install dummy-frontend ../../charts/frontend -f values-frontend.yaml -n dummy-tenant
```

## Uninstall Backend

```bash
helm uninstall dummy-ecommerce -n dummy-tenant
```

## Uninstall Frontend

```bash
helm uninstall dummy-frontend -n dummy-tenant
```
