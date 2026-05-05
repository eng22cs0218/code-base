import React, { createContext, useContext, useEffect, useReducer, useCallback } from 'react';
import { Service, K8sScore, K8sResourceActions, SecurityContextType, K8sPostureFinding } from '@/types/security';
import { toast } from 'sonner';
import { API_CONFIG } from '@/config/api';
import { getSessionToken, transformFindingsToServices, transformPostureToServices, transformScoreToK8sScore } from '@/lib/data-transformers';

interface SecurityState {
  services: Service[];
  score: K8sScore | null;
  actions: K8sResourceActions[];
  findings: K8sPostureFinding[];
  isConnected: boolean;
  isLoading: boolean;
  loadingValidation: boolean;
  validationStatus: 'idle' | 'running' | 'success' | 'error';
  activities: import('@/types/security').ActivityLog[];
}

type SecurityAction =
  | { type: 'SET_LOADING'; payload: boolean }
  | { type: 'SET_SERVICES'; payload: Service[] }
  | { type: 'SET_SCORE'; payload: K8sScore }
  | { type: 'SET_ACTIONS'; payload: K8sResourceActions[] }
  | { type: 'SET_FINDINGS'; payload: K8sPostureFinding[] }
  | { type: 'SET_VALIDATION_LOADING'; payload: boolean }
  | { type: 'SET_VALIDATION_STATUS'; payload: 'idle' | 'running' | 'success' | 'error' }
  | { type: 'ADD_SERVICE'; payload: Service }
  | { type: 'UPDATE_SERVICE'; payload: Service }
  | { type: 'SET_CONNECTION_STATUS'; payload: boolean }
  | { type: 'ADD_ACTIVITY'; payload: import('@/types/security').ActivityLog };

const getInitialActivities = (): import('@/types/security').ActivityLog[] => {
  try {
    const stored = sessionStorage.getItem('aegios_recent_activities');
    if (stored) {
      const parsed = JSON.parse(stored);
      return parsed.map((item: any) => ({
        ...item,
        timestamp: new Date(item.timestamp)
      }));
    }
  } catch (e) {
    console.warn("Failed to parse recent activities from localStorage", e);
  }
  return [
    {
      id: 'init-1',
      type: 'info',
      message: 'System initialized and ready',
      timestamp: new Date()
    }
  ];
};

const initialState: SecurityState = {
  services: [],
  score: null,
  actions: [],
  findings: [],
  isConnected: false,
  isLoading: true,
  loadingValidation: false,
  validationStatus: 'idle',
  activities: getInitialActivities(),
};

const securityReducer = (state: SecurityState, action: SecurityAction): SecurityState => {
  switch (action.type) {
    case 'SET_LOADING':
      return { ...state, isLoading: action.payload };
    case 'SET_SERVICES':
      return { ...state, services: action.payload };
    case 'SET_SCORE':
      return { ...state, score: action.payload };
    case 'SET_ACTIONS':
      return { ...state, actions: action.payload };
    case 'SET_FINDINGS':
      return { ...state, findings: action.payload };
    case 'SET_VALIDATION_LOADING':
      return { ...state, loadingValidation: action.payload };
    case 'SET_VALIDATION_STATUS':
      return { ...state, validationStatus: action.payload };
    case 'ADD_SERVICE':
      return { ...state, services: [...state.services, action.payload] };
    case 'UPDATE_SERVICE':
      return {
        ...state,
        services: state.services.map(service =>
          service.id === action.payload.id ? action.payload : service
        ),
      };
    case 'SET_CONNECTION_STATUS':
      return { ...state, isConnected: action.payload };
    case 'ADD_ACTIVITY':
      return { 
        ...state, 
        activities: [action.payload, ...state.activities].slice(0, 50) 
      };
    default:
      return state;
  }
};

const SecurityContext = createContext<SecurityContextType | undefined>(undefined);

export const useSecurityContext = () => {
  const context = useContext(SecurityContext);
  if (!context) {
    throw new Error('useSecurityContext must be used within a SecurityProvider');
  }
  return context;
};

export const SecurityProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(securityReducer, initialState);
  const pollingRef = React.useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    sessionStorage.setItem('aegios_recent_activities', JSON.stringify(state.activities));
  }, [state.activities]);

  const fetchServices = useCallback(async () => {
    const sessionToken = getSessionToken();
    if (!sessionToken) {
      console.log('⚠️ No session token found, skipping fetch');
      return;
    }

    console.log('🔄 Fetching security posture...');
    console.log('📍 API URL:', API_CONFIG.ENDPOINTS.SECURITY.POSTURE);
    console.log('🔑 Session Token:', sessionToken.substring(0, 20) + '...');

    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SECURITY.POSTURE, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: sessionToken }),
      });

      console.log('📥 Response status:', response.status);

      const result = await response.json();
      console.log('📦 Posture response:', result);

      if (!result.success) {
        console.error('❌ Posture fetch failed:', result.message || result.error);
        throw new Error(result.message || result.error || 'Failed to fetch posture');
      }

      console.log('✅ Posture data received:', result.data);

      const services = transformPostureToServices(result.data);
      console.log('✅ Transformed services:', services.length);

      dispatch({ type: 'SET_SERVICES', payload: services });
      dispatch({ type: 'SET_CONNECTION_STATUS', payload: true });
    } catch (error) {
      console.error('❌ Failed to fetch services:', error);
      dispatch({ type: 'SET_CONNECTION_STATUS', payload: false });

      // Only show error toast if user is authenticated (has session token)
      // Don't show errors on login page
      if (getSessionToken() && !API_CONFIG.ENABLE_MOCK_DATA) {
        toast.error('Failed to fetch security posture');
      }
    }
  }, []);

  const fetchScore = useCallback(async () => {
    const sessionToken = getSessionToken();
    if (!sessionToken) {
      console.log('⚠️ No session token, skipping fetch');
      return;
    }

    console.log('🔄 Fetching security score...');

    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SECURITY.SCORE, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: sessionToken }),
      });

      console.log('📥 Score response status:', response.status);

      const result = await response.json();
      console.log('📦 Score response:', result);

      if (!result.success) {
        console.error('❌ Score fetch failed:', result.message || result.error);
        throw new Error(result.message || result.error || 'Failed to fetch score');
      }

      console.log('✅ Score data received:', result.data);

      const score = transformScoreToK8sScore(result.data);
      console.log('✅ Transformed score:', score);

      dispatch({ type: 'SET_SCORE', payload: score });
    } catch (error) {
      console.error('❌ Failed to fetch score:', error);

      // Only show error toast if user is authenticated
      if (getSessionToken() && !API_CONFIG.ENABLE_MOCK_DATA) {
        toast.error('Failed to fetch security score');
      }
    }
  }, []);

  const fetchActions = useCallback(async () => {
    const sessionToken = getSessionToken();
    if (!sessionToken) {
      console.log('⚠️ No session token, skipping actions fetch');
      return;
    }

    console.log('🔄 Fetching K8s actions...');

    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SECURITY.ACTIONS, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: sessionToken }),
      });

      console.log('📥 Actions response status:', response.status);

      const result = await response.json();
      console.log('📦 Actions response:', result);

      if (!result.success) {
        console.error('❌ Actions fetch failed:', result.message || result.error);
        throw new Error(result.message || result.error || 'Failed to fetch actions');
      }

      console.log('✅ Actions data received:', result.data);

      dispatch({ type: 'SET_ACTIONS', payload: result.data.actions || [] });
    } catch (error) {
      console.error('❌ Failed to fetch actions:', error);

      // Only show error toast if user is authenticated
      if (getSessionToken() && !API_CONFIG.ENABLE_MOCK_DATA) {
        toast.error('Failed to fetch K8s actions');
      }
    }
  }, []);

  const fetchFindings = useCallback(async () => {
    const sessionToken = getSessionToken();
    if (!sessionToken) {
      console.log('⚠️ No session token, skipping findings fetch');
      return;
    }

    console.log('🔄 Fetching posture findings...');

    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SECURITY.POSTURE_FINDINGS, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: sessionToken }),
      });

      const result = await response.json();
      console.log('📦 Findings response:', result);

      if (!result.success) {
        throw new Error(result.message || result.error || 'Failed to fetch findings');
      }

      dispatch({ type: 'SET_FINDINGS', payload: result.data?.findings || [] });
    } catch (error) {
      console.error('❌ Failed to fetch findings:', error);

      if (getSessionToken() && !API_CONFIG.ENABLE_MOCK_DATA) {
        toast.error('Failed to fetch posture findings');
      }
    }
  }, []);

  const getFindingsByCategory = useCallback((category: string) => {
    const normalizedCategory = category.toLowerCase();
    const issueTypeMap: Record<string, string[]> = {
      'rbac': ['rbac'],
      'network-policy': ['network-policy'],
      'service-port': ['service-port', 'container-port'],
      'resource-limit': ['resource-limit', 'pod'],
      'container-security': ['container-security', 'container-image'],
      'container-port': ['service-port', 'container-port'],
      'pod': ['resource-limit', 'pod'],
      'container-image': ['container-security', 'container-image'],
      'secrets': ['secrets'],
    };
    const expectedIssueTypes = issueTypeMap[normalizedCategory];

    if (!expectedIssueTypes || expectedIssueTypes.length === 0) {
      return [];
    }

    return state.findings.filter((finding) => {
      const issueType = (finding.issue_type || '').toLowerCase().trim();

      if (normalizedCategory === 'container-security' || normalizedCategory === 'container-image') {
        const details = `${finding.description || ''} ${finding.recommendation || ''} ${finding.check_name || ''}`.toLowerCase();
        if (expectedIssueTypes.includes(issueType)) return true;
        return details.includes('privileged') ||
          details.includes('running as root') ||
          details.includes('runasuser') ||
          details.includes('allow privilege escalation') ||
          details.includes('allowprivilegeescalation') ||
          details.includes('image') ||
          details.includes('registry') ||
          details.includes('latest tag') ||
          details.includes('digest');
      }

      return expectedIssueTypes.includes(issueType);
    });
  }, [state.findings]);

  const getActionFindingsByCategory = useCallback((category: string) => {
    const normalizedCategory = category.toLowerCase().trim();
    const isServicePort = normalizedCategory === 'service-port' || normalizedCategory === 'container-port';
    const isResourceLimit = normalizedCategory === 'resource-limit' || normalizedCategory === 'pod';
    const isContainerSecurity = normalizedCategory === 'container-security' || normalizedCategory === 'container-image';

    return state.findings.filter((finding) => {
      const issueType = (finding.issue_type || '').toLowerCase().trim();
      const kind = (finding.kind || finding.missing_kind || '').toLowerCase().trim();
      const details = `${finding.description || ''} ${finding.recommendation || ''} ${finding.check_name || ''}`.toLowerCase();

      if (normalizedCategory === 'rbac') {
        if (issueType === 'rbac') return true;
        return ['role', 'clusterrole', 'rolebinding', 'clusterrolebinding', 'serviceaccount'].includes(kind) ||
          details.includes('rbac') || details.includes('role binding');
      }

      if (normalizedCategory === 'network-policy') {
        if (issueType === 'network-policy') return true;
        return kind === 'networkpolicy' || details.includes('networkpolicy') || details.includes('network policy') || details.includes('ingress') || details.includes('egress');
      }

      if (isServicePort) {
        if (issueType === 'service-port' || issueType === 'container-port') return true;
        return kind === 'service' || details.includes('nodeport') || details.includes('targetport') || details.includes('containerport') || details.includes('port');
      }

      if (isResourceLimit) {
        if (issueType === 'resource-limit' || issueType === 'pod') return true;
        return details.includes('resource configuration') ||
          details.includes('resource requests') ||
          details.includes('resource limits') ||
          details.includes('requests.cpu') ||
          details.includes('limits.cpu') ||
          details.includes('requests.memory') ||
          details.includes('limits.memory');
      }

      if (isContainerSecurity) {
        if (issueType === 'container-security' || issueType === 'container-image') return true;
        return details.includes('privileged') ||
          details.includes('running as root') ||
          details.includes('runasuser') ||
          details.includes('allow privilege escalation') ||
          details.includes('allowprivilegeescalation') ||
          details.includes('image') ||
          details.includes('registry') ||
          details.includes('latest tag') ||
          details.includes('digest');
      }

      if (normalizedCategory === 'secrets') {
        if (issueType === 'secrets') return true;
        return kind === 'secret' || kind === 'configmap' ||
          details.includes('secret') || details.includes('password') || details.includes('token') || details.includes('apikey');
      }

      return false;
    });
  }, [state.findings]);

  const getServicesByCategory = useCallback((category: string) => {
    const filteredFindings = getFindingsByCategory(category);
    return transformFindingsToServices(filteredFindings);
  }, [getFindingsByCategory]);

  const startPolling = useCallback(() => {
    if (pollingRef.current) return;

    // Poll every 10 seconds
    pollingRef.current = setInterval(() => {
      fetchServices();
      fetchScore();
      fetchActions();
    }, 10000);
  }, [fetchServices, fetchScore, fetchActions]);

  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  const updateService = useCallback((service: Service) => {
    dispatch({ type: 'UPDATE_SERVICE', payload: service });
  }, []);

  const addService = useCallback((service: Service) => {
    dispatch({ type: 'ADD_SERVICE', payload: service });
  }, []);

  const updateScore = useCallback((score: K8sScore) => {
    dispatch({ type: 'SET_SCORE', payload: score });
  }, []);

  const addActivity = useCallback((message: string, type: 'success' | 'info' | 'warning') => {
    dispatch({
      type: 'ADD_ACTIVITY',
      payload: {
        id: Date.now().toString() + Math.random().toString(36).substring(2, 9),
        type,
        message,
        timestamp: new Date()
      }
    });
  }, []);

  const refreshData = useCallback(async () => {
    const sessionToken = getSessionToken();
    if (!sessionToken) {
      console.log('⚠️ No session token, cannot refresh data');
      return;
    }

    console.log('🔄 Refreshing all security data...');
    dispatch({ type: 'SET_LOADING', payload: true });
    await Promise.all([fetchServices(), fetchScore(), fetchActions(), fetchFindings()]);
    dispatch({ type: 'SET_LOADING', payload: false });
  }, [fetchServices, fetchScore, fetchActions, fetchFindings]);

  const applyAction = useCallback(async (resourceId: string, actionType: string, findingDetails?: { description?: string; recommendation?: string; kind?: string }) => {
    const sessionToken = getSessionToken();
    if (!sessionToken) {
      toast.error('Not authenticated');
      return { success: false, message: 'Not authenticated' };
    }

    console.log('🔄 Applying action:', actionType, 'to resource:', resourceId);

    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SECURITY.AGENTIC, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_token: sessionToken,
          resource_id: resourceId,
          action_type: actionType,
          finding_description: findingDetails?.description || '',
          finding_recommendation: findingDetails?.recommendation || '',
          finding_kind: findingDetails?.kind || '',
        }),
      });

      const result = await response.json();
      console.log('📦 Apply action response:', result);

      if (!result.success) {
        console.error('❌ Apply action failed:', result.message || result.error);
        throw new Error(result.message || result.error || 'Failed to apply action');
      }

      console.log('✅ Action applied:', result.data);

      toast.success(result.message || 'Action applied successfully');
      
      addActivity(`Applied fix to resource: ${actionType}`, 'success');

      // Refresh data after action
      await fetchServices();
      await fetchScore();
      await fetchActions();
      await fetchFindings();

      return {
        success: true,
        message: result.message,
        output: result.data?.ai_output || result.data?.note || result.data?.output || 'Action completed'
      };
    } catch (error) {
      console.error('❌ Failed to apply action:', error);
      const errorMessage = error instanceof Error ? error.message : 'Failed to apply action';
      toast.error(errorMessage);

      return {
        success: false,
        message: errorMessage,
        output: `Error: ${errorMessage}`
      };
    }
  }, [fetchServices, fetchScore, fetchActions, fetchFindings]);

  const runValidation = useCallback(async () => {
    const sessionToken = getSessionToken();
    if (!sessionToken) {
      toast.error('Not authenticated');
      return { success: false, message: 'Not authenticated' };
    }

    dispatch({ type: 'SET_VALIDATION_LOADING', payload: true });
    dispatch({ type: 'SET_VALIDATION_STATUS', payload: 'running' });

    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SECURITY.VALIDATE_NAMESPACES, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: sessionToken }),
      });

      const result = await response.json();
      if (!result.success) {
        throw new Error(result.message || result.error || 'Validation failed');
      }

      await Promise.all([fetchServices(), fetchScore(), fetchActions(), fetchFindings()]);

      dispatch({ type: 'SET_VALIDATION_STATUS', payload: 'success' });
      toast.success(result.message || 'Validation completed');
      
      addActivity('Validation checks completed', 'success');

      return { success: true, message: result.message || 'Validation completed' };
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Validation failed';
      dispatch({ type: 'SET_VALIDATION_STATUS', payload: 'error' });
      toast.error(errorMessage);

      return { success: false, message: errorMessage };
    } finally {
      dispatch({ type: 'SET_VALIDATION_LOADING', payload: false });
    }
  }, [fetchServices, fetchScore, fetchActions, fetchFindings]);

  useEffect(() => {
    const sessionToken = getSessionToken();

    // Don't automatically fetch data on mount
    // Data will only be fetched when user clicks "Fetch Data" button
    // or when explicitly navigating to security pages
    if (!sessionToken) {
      console.log('⚠️ No session token, skipping data initialization');
      dispatch({ type: 'SET_LOADING', payload: false });
      return;
    }

    // Set loading to false - data will be fetched on demand
    dispatch({ type: 'SET_LOADING', payload: false });

    // Listen for login events and fetch-data completion
    const handleLogin = async () => {
      console.log('🔔 Data fetch completed, refreshing security data...');
      dispatch({ type: 'SET_LOADING', payload: true });
      await Promise.all([fetchServices(), fetchScore(), fetchActions(), fetchFindings()]);
      dispatch({ type: 'SET_LOADING', payload: false });
    };

    window.addEventListener('aegios:login', handleLogin);

    return () => {
      stopPolling();
      window.removeEventListener('aegios:login', handleLogin);
    };
  }, [fetchServices, fetchScore, fetchActions, fetchFindings, stopPolling]);

  const contextValue: SecurityContextType = {
    services: state.services,
    score: state.score,
    actions: state.actions,
    findings: state.findings,
    isConnected: state.isConnected,
    isLoading: state.isLoading,
    loadingValidation: state.loadingValidation,
    validationStatus: state.validationStatus,
    updateService,
    addService,
    updateScore,
    applyAction,
    runValidation,
    fetchActions,
    fetchFindings,
    getFindingsByCategory,
    getActionFindingsByCategory,
    getServicesByCategory,
    refreshData,
    addActivity,
    activities: state.activities,
  };

  return (
    <SecurityContext.Provider value={contextValue}>
      {children}
    </SecurityContext.Provider>
  );
};