import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Service } from "@/types/security";
import { useNavigate } from "react-router-dom";
import { ExternalLink, AlertTriangle, AlertCircle } from "lucide-react";

interface ServiceCardProps {
  service: Service;
}

const ServiceCard = ({ service }: ServiceCardProps) => {
  const navigate = useNavigate();
  const issueType = typeof service.metadata?.issue_type === 'string' ? service.metadata.issue_type.toLowerCase() : '';
  const ownerText = typeof service.metadata?.owner === 'string' ? service.metadata.owner.toLowerCase() : '';
  const kindLabel = typeof service.labels?.kind === 'string' ? service.labels.kind.toLowerCase() : '';
  const shouldShowPorts = service.ports.length > 0 && (
    issueType === 'container-port' ||
    ownerText.includes('service exposure') ||
    kindLabel === 'service'
  );

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'Low':
        return 'bg-orange-500/20 text-orange-500 border-orange-500';
      case 'High':
        return 'bg-yellow-500/20 text-yellow-500 border-yellow-500';
      case 'Critical':
        return 'bg-[#FF0000]/20 text-[#FF0000] border-[#FF0000]';
      default:
        return 'bg-muted/20 text-muted-foreground border-muted';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'Low':
        return <AlertTriangle className="h-4 w-4" />;
      case 'High':
        return <AlertTriangle className="h-4 w-4" />;
      case 'Critical':
        return <AlertCircle className="h-4 w-4" />;
      default:
        return <AlertCircle className="h-4 w-4" />;
    }
  };

  const handleViewActions = () => {
    const issueType = typeof service.metadata?.issue_type === 'string'
      ? service.metadata.issue_type.toLowerCase()
      : '';

    const categoryMap: Record<string, string> = {
      'container-port': 'service-port',
      'pod': 'resource-limit',
      'container-image': 'container-security',
    };

    const category = categoryMap[issueType] || issueType;

    if (category) {
      navigate(`/security/k8s-action/${category}`);
      return;
    }

    navigate('/security/k8s-action');
  };

  return (
    <Card className="border-cyber-border bg-card glow-border transition-colors hover:border-primary">
      <CardContent className="p-6 space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Namespace</p>
            <p className="text-sm font-medium text-foreground">{service.namespace || "default"}</p>
          </div>
          <div>
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Name</p>
            <p className="text-sm font-semibold text-primary">{service.name || "Unknown"}</p>
          </div>
          <div>
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Labels</p>
            <div className="flex flex-wrap gap-2 mt-1">
              {Object.entries(service.labels).map(([key, value]) => (
                <Badge
                  key={key}
                  className="text-xs neon-badge"
                >
                  {key}: {value}
                </Badge>
              ))}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Status:</span>
          <Badge className={`${getStatusColor(service.status)} flex items-center gap-1 w-fit`}>
            {getStatusIcon(service.status)}
            {service.status}
          </Badge>
        </div>

        <div className="rounded-lg border border-primary/40 bg-secondary/10 p-4 space-y-2">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">Current Config</p>
          {shouldShowPorts && (
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">Exposed Ports:</p>
              <div className="flex flex-wrap gap-2">
                {service.ports.map((port) => (
                  <span
                    key={port}
                    className="text-lg font-bold text-destructive bg-destructive/10 border border-destructive/30 rounded px-2 py-1"
                  >
                    {port}
                  </span>
                ))}
              </div>
            </div>
          )}
          {service.metadata?.description ? (
            <p className="text-sm text-foreground whitespace-pre-wrap">{service.metadata.description}</p>
          ) : service.metadata?.owner ? (
            <p className="text-sm text-foreground whitespace-pre-wrap">{service.metadata.owner}</p>
          ) : !shouldShowPorts && (
            <p className="text-sm text-foreground">No issue details available.</p>
          )}
        </div>

        <div className="rounded-lg border border-primary/40 bg-secondary/10 p-4 space-y-2">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">Recommendation</p>
          {service.metadata?.recommendation_raw ? (
            <p className="text-sm text-foreground whitespace-pre-wrap">{service.metadata.recommendation_raw}</p>
          ) : service.recommendations.length > 0 ? (
            <ul className="space-y-2">
              {service.recommendations.map((recommendation, index) => (
                <li
                  key={index}
                  className="text-sm text-foreground flex items-start gap-2"
                >
                  <span className="text-primary mt-1">•</span>
                  <span>{recommendation}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-foreground">No recommendation provided.</p>
          )}
        </div>

        <div className="pt-4 border-t border-cyber-border mt-2">
          <Button
            onClick={handleViewActions}
            variant="outline"
            className="w-full lg:w-auto border-primary text-primary hover:bg-primary/10 hover:text-primary bg-transparent"
          >
            <ExternalLink className="h-4 w-4 mr-2" />
            View Actions
          </Button>
        </div>
      </CardContent>
    </Card>
  );
};

export default ServiceCard;