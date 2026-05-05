# Aegios Control Center - Frontend

A modern, high-performance security dashboard built with **React**, **Vite**, **TypeScript**, and **Tailwind CSS**. Aegios provides a premium, interactive interface for Kubernetes security monitoring and remediation.

## 🎨 Design System

Aegios uses a custom, premium design system inspired by modern security operations centers:
- **Style**: Sleek dark mode with glassmorphism and vibrant emerald-green accents.
- **Typography**: Modern fonts (Inter/Roboto) for maximum legibility.
- **UI Components**: Built on **Shadcn UI** for consistency and accessibility.
- **Animations**: Subtle micro-animations for interactive feedback.

---

## 🏗️ Project Architecture

The frontend is organized for scalability and maintainability:

### 🧩 Core Directories
- **`/src/components/`**: Reusable UI and feature components (e.g., Security, UI, Layout).
- **`/src/contexts/`**: Global state management for Authentication and Security data.
- **`/src/pages/`**: Page-level components for Dashboard, Auth, and Security modules.
- **`/src/config/`**: Centralized API and environment settings.
- **`/src/lib/`**: API clients, data transformers, and utility functions.

### 📁 Technical Stack
- **Framework**: React 18 with Vite.
- **Styling**: Tailwind CSS & Vanilla CSS.
- **Icons**: Lucide React.
- **Charts**: Recharts for security score visualization.
- **Notifications**: Sonner for real-time toast feedback.

---

## 🚀 Setup & Local Development

### 1. Prerequisites
- **Node.js**: 18+ (LTS recommended)
- **NPM** or **Bun**: Package manager.

### 2. Configuration
Create a `.env` file in the frontend directory:
```env
VITE_API_BASE_URL=http://localhost:8080
```

### 3. Run Manually
```bash
npm install
npm run dev
```
The application will run on [http://localhost:8081](http://localhost:8081).

---

## 🧭 Pages & Navigation
- **Dashboard (`/`)**: High-level overview of organization stats and security posture.
- **K8s Posture (`/security/k8s-posture`)**: Detailed view of namespace groupings and service cards.
- **K8s Score (`/security/k8s-score`)**: Interactive security scoring with breakdown by category.
- **K8s Actions (`/security/k8s-actions`)**: Remediation dashboard with AI-powered "Apply" commands.
- **Auth (`/login`, `/signup`)**: Secure authentication and organization onboarding.

---

**Last Updated**: April 4, 2026
