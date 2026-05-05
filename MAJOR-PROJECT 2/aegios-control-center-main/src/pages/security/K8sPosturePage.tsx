import { useEffect } from "react";
import { useSecurityContext } from "@/contexts/SecurityContext";
import ServiceCard from "@/components/security/ServiceCard";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent } from "@/components/ui/card";
import { AlertCircle } from "lucide-react";

const K8sPosturePage = () => {
  const { services, isLoading, refreshData } = useSecurityContext();

  useEffect(() => {
    // Fetch data when page loads
    refreshData();
  }, [refreshData]);

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-8 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
        
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Card key={i} className="border-cyber-border bg-card">
              <CardContent className="p-6">
                <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                  <div className="space-y-3">
                    <Skeleton className="h-4 w-20" />
                    <Skeleton className="h-6 w-32" />
                    <Skeleton className="h-4 w-24" />
                  </div>
                  <div className="space-y-3">
                    <Skeleton className="h-4 w-24" />
                    <Skeleton className="h-8 w-full" />
                  </div>
                  <div className="space-y-3">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-16 w-full" />
                  </div>
                </div>
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
        <h1 className="text-3xl font-bold text-primary">K8s Security Posture</h1>
        <p className="text-muted-foreground">
          Review security status and recommendations for all Kubernetes services in your cluster.
        </p>
      </div>

      {/* Services Count */}
      <div className="flex items-center gap-4 text-sm text-muted-foreground">
        <span>Total Services: <span className="text-primary font-semibold">{services.length}</span></span>
        <span>•</span>
        <span>Critical: <span className="text-red-500 font-semibold">
          {services.filter(s => s.status === 'Critical').length}
        </span></span>
        <span>•</span>
        <span>High: <span className="text-orange-500 font-semibold">
          {services.filter(s => s.status === 'High').length}
        </span></span>
        <span>•</span>
        <span>Low: <span className="text-yellow-500 font-semibold">
          {services.filter(s => s.status === 'Low').length}
        </span></span>
      </div>

      {/* Services List */}
      {services.length === 0 ? (
        <Card className="border-cyber-border bg-card">
          <CardContent className="p-12 text-center">
            <AlertCircle className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
            <h3 className="text-lg font-semibold text-foreground mb-2">No Services Found</h3>
            <p className="text-muted-foreground">
              No Kubernetes services are currently being monitored in your cluster.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {services.map((service) => (
            <ServiceCard key={service.id} service={service} />
          ))}
        </div>
      )}
    </div>
  );
};

export default K8sPosturePage;