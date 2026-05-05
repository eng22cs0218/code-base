# dummy-tenant

[![Build & Push Commerce Services](https://github.com/Aegios-k8s/dummy-tenant/actions/workflows/docker-build-push.yaml/badge.svg)](https://github.com/Aegios-k8s/dummy-tenant/actions/workflows/docker-build-push.yaml)

## 🚀 CI/CD Pipeline

This repository uses GitHub Actions to automatically build and push Docker images to Docker Hub whenever code is pushed to the main branch.

### Services:
- 🛒 **Ecommerce Service** - Go backend service
- 🖼️ **Wallpaper Service** - Go backend service  
- 💻 **Frontend** - React application

### Docker Images:
All images are automatically built and available on Docker Hub:
- `your-dockerhub-username/ecommerce-service:latest`
- `your-dockerhub-username/wallpaper-service:latest`
- `your-dockerhub-username/frontend:latest`