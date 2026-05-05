# Aegios Backend Services

A high-performance Go backend built with the Gin framework, responsible for GitHub integration, Kubernetes resource parsing, and security posture analysis.

## 🏗️ Service Architecture

The backend follows a service-based architecture designed for scalability and clear separation of concerns.

### 🧩 Core Services
- **Authentication Service**: Handles user signup, login, and session management using bcrypt and SHA-256 tokens.
- **Fetching Service**: Manages GitHub repository discovery, file scanning, and extraction of K8s resources from YAML and Helm charts.
- **Security Service**: Performs security scoring, posture analysis, and agentic remediation suggestions.

### 📁 Directory Structure
```tree
github-pat-backend/
├── pkg/                 # Shared logic
│   ├── database/        # PostgreSQL connection and schema
│   ├── logger/          # Structured logging
│   └── response/        # Standardized API response helpers
├── services/            # Business logic by service
│   ├── authentication/
│   ├── fetching-service/
│   └── security-service/
└── main.go              # Entry point and route orchestration
```

---

## 🚀 Setup & Local Development

### 1. Prerequisites
- **Go**: 1.22+
- **Postgres**: 17+ (Running on port 5433)
- **GitHub PAT**: Personal Access Token with read permissions.

### 2. Configuration
Create a `.env` file in the backend directory:
```env
DB_HOST=localhost
DB_PORT=5433
DB_USER=aayushsugandhi
DB_PASSWORD=aayushsugandhi
DB_NAME=aegios
PORT=8080
FRONTEND_URL=http://localhost:8082
```

### 3. Run Manually
```bash
go mod tidy
go run main.go
```

---

## 📡 API Reference Summary

The following endpoints are exposed via the API Gateway on port **8080**.

### 🔐 Authentication
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/authentication/login` | Login and initialize session |
| `POST` | `/authentication/signup` | Register new organization/user |
| `POST` | `/authentication/signout` | Invalidate current session |

### 📁 Fetching & Data
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/fetching-service/dashboard` | Get summary stats for dashboard |
| `POST` | `/fetching-service/fetch-data`| Trigger repository scanning |
| `POST` | `/fetching-service/rendering` | Process K8s resource extraction |

### 🛡️ Security Analysis
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/security-service/k8s-score` | Get security score breakdown |
| `POST` | `/security-service/k8s-posture`| Get detailed posture analysis |
| `POST` | `/security-service/k8s-action` | Get remediation recommendations |
| `POST` | `/security-service/k8s-agentic`| GPT-powered remediation application |

---

## 🗄️ Database Schema

Aegios uses PostgreSQL with a schema optimized for multi-tenancy (via `org_id`) and flexible Kubernetes storage (via `JSONB`).

### Key Tables
1. **organization**: Core organization and billing data.
2. **github_credentials**: Encrypted PATs and hashed passwords.
3. **github_repository**: Discovered repositories for each organization.
4. **github_files**: Metadata of scanned files (uses SHA-256 deduplication).
5. **kubernetes_resource**: Extracted K8s manifests stored as `JSONB`.
6. **findings**: Security issues and compliance gaps identified during analysis.

---

**Last Updated**: April 4, 2026
