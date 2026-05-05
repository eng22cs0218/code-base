export interface Service {
  id: string;
  namespace: string;
  name: string;
  labels: Record<string, string>;
  status: "Low" | "High" | "Critical";
  ports: number[];
  recommendations: string[];
  created_at: string;
  metadata: Record<string, any>;
}

export interface K8sScore {
  total: number;
  counts: {
    Low: number;
    High: number;
    Critical: number;
  };
  percentages: {
    Low: number;
    High: number;
    Critical: number;
  };
  score: number;
  criticality_level: "Low" | "High" | "Critical";
}

export interface K8sAction {
  type: string;
  priority: "low" | "medium" | "high" | "critical";
  description: string;
  remediation: string;
}

export interface K8sResourceActions {
  resource_id: string;
  kind: string;
  name: string;
  namespace: string;
  actions: K8sAction[];
}

export interface K8sActionsResponse {
  total_actions: number;
  actions: K8sResourceActions[];
}

export type PostureCategory =
  | 'rbac'
  | 'network-policy'
  | 'service-port'
  | 'resource-limit'
  | 'container-security'
  | 'container-port'
  | 'pod'
  | 'container-image'
  | 'secrets';

export interface K8sPostureFinding {
  finding_id: string;
  resource_id?: string;
  namespace: string;
  name: string;
  kind: string;
  repo_name?: string;
  missing_kind?: string;
  issue_type: PostureCategory | string;
  check_name?: string;
  severity: string;
  description: string;
  recommendation: string;
  detected_at: string;
}

export interface SecurityEvent {
  event: "service.created" | "service.updated" | "score.updated";
  data: Service | K8sScore;
}

export interface ActivityLog {
  id: string;
  type: 'success' | 'info' | 'warning';
  message: string;
  timestamp: Date;
}

export interface SecurityContextType {
  services: Service[];
  score: K8sScore | null;
  actions: K8sResourceActions[];
  findings: K8sPostureFinding[];
  activities: ActivityLog[];
  isConnected: boolean;
  isLoading: boolean;
  loadingValidation: boolean;
  validationStatus: 'idle' | 'running' | 'success' | 'error';
  updateService: (service: Service) => void;
  addService: (service: Service) => void;
  updateScore: (score: K8sScore) => void;
  applyAction: (serviceId: string, command: string, findingDetails?: { description?: string; recommendation?: string; kind?: string }) => Promise<{ success: boolean; message: string; output?: string }>;
  takeTerminalAction: (findingId: string) => void;
  runValidation: () => Promise<{ success: boolean; message: string }>;
  fetchActions: () => Promise<void>;
  fetchFindings: () => Promise<void>;
  getFindingsByCategory: (category: string) => K8sPostureFinding[];
  getActionFindingsByCategory: (category: string) => K8sPostureFinding[];
  getServicesByCategory: (category: string) => Service[];
  refreshData: () => Promise<void>;
  addActivity: (message: string, type: 'success' | 'info' | 'warning') => void;
}