/**
 * Data Transformers
 * Transform backend API responses to frontend types
 */

import { Service, K8sScore, K8sPostureFinding } from '@/types/security';

// Backend response types
interface BackendPostureResource {
  id: number;
  resource_id: string;
  kind: string;
  name: string;
  namespace: string;
  file_path: string;
  repo_name: string;
  issues: string[] | null; // Can be null
  severity: 'none' | 'low' | 'medium' | 'high' | 'critical';
}

interface BackendPostureResponse {
  total_resources: number;
  total_issues: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  resources: BackendPostureResource[];
}

interface BackendScoreResponse {
  total_findings: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  critical_percentage: number;
  high_percentage: number;
  medium_percentage: number;
  low_percentage: number;
  security_score: number;
  has_validation_data?: boolean;
  overall_grade: string;
  criticality_level?: string;
}

/**
 * Transform backend posture response to frontend Service[] type
 */
export function transformPostureToServices(response: BackendPostureResponse | any): Service[] {
  // Handle both direct response and wrapped response
  const postureData = response.resources ? response : response;
  
  if (!postureData.resources || !Array.isArray(postureData.resources)) {
    console.warn('⚠️ Invalid posture response format:', response);
    return [];
  }

  return postureData.resources.map((resource: BackendPostureResource) => {
    // Map severity to status
    let status: 'Low' | 'High' | 'Critical';
    if (resource.severity === 'critical') {
      status = 'Critical';
    } else if (resource.severity === 'high') {
      status = 'High';
    } else {
      status = 'Low';
    }

    return {
      id: resource.resource_id,
      namespace: resource.namespace || 'default',
      name: resource.name,
      labels: {
        kind: resource.kind,
        repo: resource.repo_name,
      },
      status,
      ports: [], // Backend doesn't provide ports in posture response
      recommendations: resource.issues || [], // Handle null issues
      created_at: new Date().toISOString(),
      metadata: {
        file_path: resource.file_path,
        repo_name: resource.repo_name,
        severity: resource.severity,
        issue_count: (resource.issues || []).length,
      },
    };
  });
}

/**
 * Transform backend score response to frontend K8sScore type
 */
export function transformScoreToK8sScore(response: BackendScoreResponse | any): K8sScore {
  const scoreData = response.total_findings !== undefined ? response : response;

  if (scoreData.total_findings === undefined) {
    console.warn('⚠️ Invalid score response format:', response);
    return {
      total: 0,
      counts: { Low: 0, High: 0, Critical: 0 },
      percentages: { Low: 0, High: 0, Critical: 0 },
      score: 0,
      criticality_level: 'Critical',
    };
  }

  const total = Number(scoreData.total_findings || 0);
  if (total === 0) {
    return {
      total: 0,
      counts: { Low: 0, High: 0, Critical: 0 },
      percentages: { Low: 0, High: 0, Critical: 0 },
      score: 0,
      criticality_level: 'Critical',
    };
  }

  // Keep only 3 buckets in UI:
  // Low -> low + medium severities
  // High -> high severities
  // Critical -> critical severities
  const lowCount = Number(scoreData.low || 0) + Number(scoreData.medium || 0);
  const highCount = Number(scoreData.high || 0);
  const criticalCount = Number(scoreData.critical || 0);

  const lowPct = Number(scoreData.low_percentage || 0) + Number(scoreData.medium_percentage || 0);
  const highPct = Number(scoreData.high_percentage || 0);
  const criticalPct = Number(scoreData.critical_percentage || 0);

  return {
    total,
    counts: {
      Low: lowCount,
      High: highCount,
      Critical: criticalCount,
    },
    percentages: {
      Low: lowPct,
      High: highPct,
      Critical: criticalPct,
    },
    score: Math.round(Number(scoreData.security_score || 0)),
    criticality_level: getCriticalityLevel(Number(scoreData.security_score || 0)),
  };
}

/**
 * Transform findings rows to Service[] so existing ServiceCard UI can be reused.
 */
export function transformFindingsToServices(findings: K8sPostureFinding[]): Service[] {
  return findings.map((finding) => {
    const severity = (finding.severity || '').toLowerCase();

    let status: 'Low' | 'High' | 'Critical';
    if (severity === 'critical') {
      status = 'Critical';
    } else if (severity === 'high') {
      status = 'High';
    } else {
      status = 'Low';
    }

    const recommendations = splitRecommendationText(finding.recommendation, finding.description);
    const ports = extractPorts(`${finding.description || ''} ${finding.recommendation || ''}`);

    return {
      id: finding.resource_id || finding.finding_id,
      namespace: finding.namespace || 'default',
      name: finding.name || finding.kind || 'Unknown Resource',
      labels: {
        kind: finding.kind || finding.missing_kind || 'Unknown',
        repo: finding.repo_name || 'unknown-repo',
      },
      status,
      ports,
      recommendations,
      created_at: finding.detected_at || new Date().toISOString(),
      metadata: {
        finding_id: finding.finding_id,
        resource_id: finding.resource_id,
        severity,
        missing_kind: finding.missing_kind,
        description: finding.description,
        recommendation_raw: finding.recommendation,
        issue_type: finding.issue_type,
        owner: `${(finding.check_name || finding.issue_type || 'Security Check').toUpperCase()} • ${severity.toUpperCase()}`,
      },
    };
  });
}

function splitRecommendationText(recommendation: string, fallbackDescription: string): string[] {
  const raw = (recommendation || '').trim();
  if (!raw) {
    return fallbackDescription ? [fallbackDescription] : [];
  }

  return raw
    .split(/\n+|\.\s+|;\s+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function extractPorts(text: string): number[] {
  const matches = text.match(/\b\d{2,5}\b/g) || [];
  const ports = matches
    .map((value) => Number(value))
    .filter((value) => Number.isInteger(value) && value > 0 && value <= 65535);

  return Array.from(new Set(ports));
}

/**
 * Get criticality level based on score percentage
 */
function getCriticalityLevel(percentage: number): 'Low' | 'High' | 'Critical' {
  if (percentage >= 80) return 'Low';
  if (percentage >= 60) return 'High';
  return 'Critical';
}

/**
 * Get session token from localStorage
 */
export function getSessionToken(): string | null {
  return localStorage.getItem('aegios_session_token');
}

/**
 * Store session token in localStorage
 */
export function setSessionToken(token: string): void {
  localStorage.setItem('aegios_session_token', token);
}

/**
 * Remove session token from localStorage
 */
export function clearSessionToken(): void {
  localStorage.removeItem('aegios_session_token');
  localStorage.removeItem('aegios_recent_activities');
}
