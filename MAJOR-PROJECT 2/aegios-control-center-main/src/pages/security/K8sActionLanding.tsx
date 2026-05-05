import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { ShieldCheck, Network, ScanSearch, Boxes, Container, KeyRound } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useSecurityContext } from "@/contexts/SecurityContext";

const ACTION_CATEGORY_CARDS = [
  {
    id: "rbac",
    title: "RBAC",
    description: "Role and binding vulnerabilities in namespace and cluster access control.",
    icon: ShieldCheck,
  },
  {
    id: "network-policy",
    title: "Network Policy",
    description: "Open ingress and egress rules that weaken traffic isolation.",
    icon: Network,
  },
  {
    id: "service-port",
    title: "Service Port",
    description: "Service port exposure and routing issues that expand attack surface.",
    icon: ScanSearch,
  },
  {
    id: "resource-limit",
    title: "Resource Limit",
    description: "Resource request and limit misconfigurations in workloads.",
    icon: Boxes,
  },
  {
    id: "container-security",
    title: "Container Security",
    description: "Container runtime and image hardening security issues.",
    icon: Container,
  },
  {
    id: "secrets",
    title: "Secrets",
    description: "Secret leaks and sensitive data misconfiguration findings.",
    icon: KeyRound,
  },
] as const;

const K8sActionLanding = () => {
  const navigate = useNavigate();
  const { isLoading, refreshData, getActionFindingsByCategory } = useSecurityContext();

  useEffect(() => {
    refreshData();
  }, [refreshData]);

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-8 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {[1, 2, 3, 4, 5, 6].map((item) => (
            <Card key={item} className="bg-black border border-green-500/60 rounded-xl">
              <CardContent className="p-6 space-y-3">
                <Skeleton className="h-5 w-32" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-24" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-3xl font-bold text-primary">K8s Actions</h1>
        <p className="text-muted-foreground">
          Choose a security category to inspect findings and apply targeted remediations.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {ACTION_CATEGORY_CARDS.map((category) => {
          const Icon = category.icon;
          const count = getActionFindingsByCategory(category.id).length;

          return (
            <Card
              key={category.id}
              onClick={() => navigate(`/security/k8s-action/${category.id}`)}
              className="border-cyber-border bg-card glow-border cursor-pointer transition-all duration-300 hover:-translate-y-1 hover:border-primary hover:shadow-[0_0_24px_rgba(0,255,65,0.28)]"
              role="button"
              tabIndex={0}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  navigate(`/security/k8s-action/${category.id}`);
                }
              }}
            >
              <CardContent className="p-6 space-y-4">
                <div className="flex items-center justify-between">
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-primary bg-primary/10 text-primary">
                    <Icon className="h-5 w-5" />
                  </span>
                  <span className="text-xs px-2 py-1 rounded border border-primary/50 text-primary bg-primary/10">
                    {count} Findings
                  </span>
                </div>
                <div className="space-y-1">
                  <h2 className="text-lg font-semibold text-foreground">{category.title}</h2>
                  <p className="text-sm text-muted-foreground leading-relaxed">{category.description}</p>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>
    </div>
  );
};

export default K8sActionLanding;
