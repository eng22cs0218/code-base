import { useEffect, useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { AlertCircle, ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import ActionCard from "@/components/security/ActionCard";
import { useSecurityContext } from "@/contexts/SecurityContext";
import { K8sPostureFinding, K8sResourceActions } from "@/types/security";

const CATEGORY_LABELS: Record<string, string> = {
  rbac: "RBAC",
  "network-policy": "Network Policy",
  "service-port": "Service Port",
  "resource-limit": "Resource Limit",
  "container-security": "Container Security",
  "container-port": "Service Port",
  pod: "Resource Limit",
  "container-image": "Container Security",
  secrets: "Secrets",
};

const severityFromPriority = (priority: string): string => {
  const normalized = (priority || "").toLowerCase();
  if (["critical", "high", "low"].includes(normalized)) {
    return normalized;
  }
  if (normalized === "medium") {
    return "low";
  }
  return "high";
};

const mapActionsToFindingCards = (category: string, resources: K8sResourceActions[]): K8sPostureFinding[] => {
  const actionCards: K8sPostureFinding[] = [];
  const normalizedCategory = category.toLowerCase();
  const isServicePort = normalizedCategory === "service-port" || normalizedCategory === "container-port";
  const isResourceLimit = normalizedCategory === "resource-limit" || normalizedCategory === "pod";
  const isContainerSecurity = normalizedCategory === "container-security" || normalizedCategory === "container-image";

  resources.forEach((resource) => {
    const kind = (resource.kind || "").toLowerCase();

    (resource.actions || []).forEach((action, index) => {
      const actionTextBlob = `${action.type} ${action.description} ${action.remediation}`.toLowerCase();

      const belongsToCategory = (() => {
        if (category === "rbac") {
          return ["role", "clusterrole", "rolebinding", "clusterrolebinding", "serviceaccount"].includes(kind) || actionTextBlob.includes("rbac");
        }
        if (category === "network-policy") {
          return kind === "networkpolicy" || actionTextBlob.includes("network policy") || actionTextBlob.includes("ingress") || actionTextBlob.includes("egress");
        }
        if (isServicePort) {
          return kind === "service" || actionTextBlob.includes("port") || actionTextBlob.includes("nodeport") || actionTextBlob.includes("targetport");
        }
        if (isResourceLimit) {
          return actionTextBlob.includes("resource configuration") ||
            actionTextBlob.includes("resource requests") ||
            actionTextBlob.includes("resource limits") ||
            actionTextBlob.includes("requests.cpu") ||
            actionTextBlob.includes("limits.cpu") ||
            actionTextBlob.includes("requests.memory") ||
            actionTextBlob.includes("limits.memory");
        }
        if (isContainerSecurity) {
          return actionTextBlob.includes("privileged") ||
            actionTextBlob.includes("runasuser") ||
            actionTextBlob.includes("running as root") ||
            actionTextBlob.includes("allow privilege escalation") ||
            actionTextBlob.includes("allowprivilegeescalation") ||
            actionTextBlob.includes("image") ||
            actionTextBlob.includes("tag") ||
            actionTextBlob.includes("registry") ||
            actionTextBlob.includes("non-root") ||
            actionTextBlob.includes("non_root");
        }
        if (category === "secrets") {
          return ["secret", "configmap"].includes(kind) || actionTextBlob.includes("secret") || actionTextBlob.includes("password") || actionTextBlob.includes("token");
        }
        return false;
      })();

      if (belongsToCategory) {
        actionCards.push({
          finding_id: `${resource.resource_id}-${index}`,
          resource_id: resource.resource_id,
          namespace: resource.namespace || "default",
          name: resource.name,
          kind: resource.kind,
          issue_type: category,
          severity: severityFromPriority(action.priority),
          description: action.description || action.type,
          recommendation: action.remediation,
          detected_at: new Date().toISOString(),
        });
      }
    });
  });

  return actionCards;
};

const K8sActionCategoryPage = () => {
  const navigate = useNavigate();
  const { category = "" } = useParams<{ category: string }>();
  const {
    isLoading,
    refreshData,
    getActionFindingsByCategory,
    applyAction,
    actions,
  } = useSecurityContext();

  useEffect(() => {
    refreshData();
  }, [refreshData]);

  const normalizedCategory = category.toLowerCase();
  const title = CATEGORY_LABELS[normalizedCategory] || "Category";

  const findings = useMemo(() => {
    const realFindings = getActionFindingsByCategory(normalizedCategory);
    if (realFindings.length > 0) {
      return realFindings;
    }
    return mapActionsToFindingCards(normalizedCategory, actions);
  }, [actions, getActionFindingsByCategory, normalizedCategory]);

  const criticalCount = findings.filter((finding) => {
    const severity = (finding.severity || "").toLowerCase();
    return severity === "critical";
  }).length;

  const highCount = findings.filter((finding) => {
    const severity = (finding.severity || "").toLowerCase();
    return severity === "high";
  }).length;

  const lowCount = findings.filter((finding) => {
    const severity = (finding.severity || "").toLowerCase();
    return severity === "medium" || severity === "low";
  }).length;

  if (!CATEGORY_LABELS[normalizedCategory]) {
    return (
      <Card className="bg-black border border-green-500/60 rounded-xl">
        <CardContent className="p-8 space-y-4 text-center">
          <AlertCircle className="h-10 w-10 mx-auto text-yellow-400" />
          <h2 className="text-xl font-semibold text-green-200">Unknown Category</h2>
          <p className="text-green-200/80">The selected action category was not recognized.</p>
          <Button onClick={() => navigate("/security/k8s-action")} className="bg-green-600 text-black hover:bg-green-500">
            Back To Categories
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-8 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="space-y-4">
          {[1, 2, 3].map((item) => (
            <Card key={item} className="bg-black border border-green-500/60 rounded-xl">
              <CardContent className="p-6">
                <Skeleton className="h-32 w-full" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div className="space-y-2">
          <h1 className="text-3xl font-bold text-primary">{title} Actions</h1>
          <p className="text-green-200/80">Filtered Kubernetes action findings for {title}.</p>
        </div>
        <Button
          variant="outline"
          onClick={() => navigate("/security/k8s-action")}
          className="border-green-500 text-green-300 hover:bg-green-500/10"
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back To Categories
        </Button>
      </div>

      <div className="flex items-center gap-4 text-sm text-green-200/80">
        <span>Total Findings: <span className="text-green-300 font-semibold">{findings.length}</span></span>
        <span>•</span>
        <span>Critical: <span className="text-[#FF0000] font-semibold">{criticalCount}</span></span>
        <span>•</span>
        <span>High: <span className="text-yellow-300 font-semibold">{highCount}</span></span>
        <span>•</span>
        <span>Low: <span className="text-orange-300 font-semibold">{lowCount}</span></span>
      </div>

      {findings.length === 0 ? (
        <Card className="bg-black border border-green-500/60 rounded-xl">
          <CardContent className="p-12 text-center">
            <AlertCircle className="h-12 w-12 text-green-200/60 mx-auto mb-4" />
            <h3 className="text-lg font-semibold text-green-200 mb-2">No Findings In This Category</h3>
            <p className="text-green-200/80">No action findings currently match this category.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {findings.map((finding) => (
            <ActionCard
              key={`${finding.finding_id}-${finding.resource_id || finding.name}`}
              finding={finding}
              onApply={async (targetFinding, command) => {
                const targetResourceId = targetFinding.resource_id || targetFinding.finding_id;
                const result = await applyAction(targetResourceId, command, {
                  description: targetFinding.description,
                  recommendation: targetFinding.recommendation,
                  kind: targetFinding.kind || targetFinding.missing_kind,
                });
                return { success: result.success, message: result.message, output: result.output };
              }}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export default K8sActionCategoryPage;
