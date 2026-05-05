import { useEffect, useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import ServiceCard from "@/components/security/ServiceCard";
import { useSecurityContext } from "@/contexts/SecurityContext";

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

const K8sCategoryPage = () => {
  const navigate = useNavigate();
  const { category = "" } = useParams<{ category: string }>();
  const { isLoading, refreshData, getServicesByCategory } = useSecurityContext();

  useEffect(() => {
    refreshData();
  }, [refreshData]);

  const normalizedCategory = category.toLowerCase();
  const title = CATEGORY_LABELS[normalizedCategory] || "Category";

  const services = useMemo(() => getServicesByCategory(normalizedCategory), [getServicesByCategory, normalizedCategory]);

  const criticalCount = services.filter((service) => service.status === "Critical").length;
  const highCount = services.filter((service) => service.status === "High").length;
  const lowCount = services.filter((service) => service.status === "Low").length;

  if (!CATEGORY_LABELS[normalizedCategory]) {
    return (
      <Card className="border-cyber-border bg-card">
        <CardContent className="p-8 space-y-4 text-center">
          <AlertCircle className="h-10 w-10 mx-auto text-yellow-500" />
          <h2 className="text-xl font-semibold text-foreground">Unknown Category</h2>
          <p className="text-muted-foreground">The selected posture category was not recognized.</p>
          <Button onClick={() => navigate("/security/k8s-posture")} className="bg-primary text-primary-foreground hover:bg-primary/90">
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
            <Card key={item} className="border-cyber-border bg-card">
              <CardContent className="p-6">
                <Skeleton className="h-28 w-full" />
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
          <h1 className="text-3xl font-bold text-primary">{title} Findings</h1>
          <p className="text-muted-foreground">
            Filtered Kubernetes posture findings for the selected category.
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => navigate("/security/k8s-posture")}
          className="border-primary text-primary hover:bg-primary/10"
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back To Categories
        </Button>
      </div>

      <div className="flex items-center gap-4 text-sm text-muted-foreground">
        <span>Total Findings: <span className="text-primary font-semibold">{services.length}</span></span>
        <span>•</span>
        <span>Critical: <span className="text-[#FF0000] font-semibold">{criticalCount}</span></span>
        <span>•</span>
        <span>High: <span className="text-yellow-500 font-semibold">{highCount}</span></span>
        <span>•</span>
        <span>Low: <span className="text-orange-500 font-semibold">{lowCount}</span></span>
      </div>

      {services.length === 0 ? (
        <Card className="border-cyber-border bg-card">
          <CardContent className="p-12 text-center">
            <AlertCircle className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
            <h3 className="text-lg font-semibold text-foreground mb-2">No Findings In This Category</h3>
            <p className="text-muted-foreground">
              No findings currently match this posture category.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {services.map((service) => (
            <ServiceCard key={`${service.id}-${service.metadata.finding_id || service.name}`} service={service} />
          ))}
        </div>
      )}
    </div>
  );
};

export default K8sCategoryPage;
