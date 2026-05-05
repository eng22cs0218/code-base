/**
 * API Configuration
 * Centralized configuration for API endpoints (Real Backend: github-pat-backend)
 */

// Get environment variables with fallback defaults
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';
const ENABLE_MOCK_DATA = import.meta.env.VITE_ENABLE_MOCK_DATA === 'true';

// API Endpoints (Monolith Backend)
export const API_CONFIG = {
  BASE_URL: API_BASE_URL,
  ENABLE_MOCK_DATA: ENABLE_MOCK_DATA,
  
  ENDPOINTS: {
    // Authentication Service
    AUTH: {
      SIGNUP: `${API_BASE_URL}/authentication/signup`,
      LOGIN: `${API_BASE_URL}/authentication/login`,
      SIGNOUT: `${API_BASE_URL}/authentication/signout`,
    },
    
    // Fetching Service
    FETCHING: {
      DASHBOARD: `${API_BASE_URL}/fetching-service/dashboard`,
      FETCH_DATA: `${API_BASE_URL}/fetching-service/fetch-data`,
	  VALIDATION: `${API_BASE_URL}/fetching-service/rendering`,
    },
    
    // Security Service
    SECURITY: {
      POSTURE: `${API_BASE_URL}/security-service/k8s-posture`,
      POSTURE_FINDINGS: `${API_BASE_URL}/security-service/k8s-posture-findings`,
      SCORE: `${API_BASE_URL}/security-service/k8s-score`,
      ACTIONS: `${API_BASE_URL}/security-service/k8s-action`,
      AGENTIC: `${API_BASE_URL}/security-service/k8s-agentic`,
      VALIDATE_NAMESPACES: `${API_BASE_URL}/security-service/validate-namespaces`,
      RAISE_PR: `${API_BASE_URL}/security-service/raise-pr`,
      LIST_BRANCHES: `${API_BASE_URL}/security-service/list-branches`,
    },

    // Session / Kube Agent Service
    SESSION: {
      GENERATE_COMMAND: `${API_BASE_URL}/session/generate-command`,
      UPLOAD_CONFIG: `${API_BASE_URL}/session/upload`,
      WS: `${API_BASE_URL.replace(/^https?/, "wss")}/session/ws`,
      TAKE_ACTION: `${API_BASE_URL}/session/take-action`,
      ACTION_STATUS: `${API_BASE_URL}/session/action-status`,
      CONFIG_STATUS: `${API_BASE_URL}/session/config-status`,
      CLUSTER_MODE: `${API_BASE_URL}/session/cluster-mode`,
      // Phase 2
      INIT: `${API_BASE_URL}/session/init`,
      STATUS: `${API_BASE_URL}/session/status`,
      UPLOAD_CONFIG_V2: `${API_BASE_URL}/api/upload-config`,
      REMEDIATION_STATUS: `${API_BASE_URL}/session/remediation-status`,
    },
  },
};

// Standard API Response Format
export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

// HTTP Client Configuration
export const HTTP_CONFIG = {
  TIMEOUT: 10000, // 10 seconds
  RETRY_ATTEMPTS: 3,
  RETRY_DELAY: 1000, // 1 second
};

export default API_CONFIG;
