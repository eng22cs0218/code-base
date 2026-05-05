import { Badge } from "@/components/ui/badge";
import { AlertTriangle, AlertCircle } from "lucide-react";

interface ServiceMetaBoxProps {
  title: string;
  children: React.ReactNode;
  className?: string;
}

const ServiceMetaBox = ({ title, children, className = "" }: ServiceMetaBoxProps) => {
  return (
    <div 
      className={`p-4 rounded-lg bg-secondary/30 neon-border min-w-[180px] ${className}`}
      role="group"
      aria-label={`${title} information`}
    >
      <div className="space-y-2">
        <p className="text-xs text-muted-foreground uppercase tracking-wide">
          {title}
        </p>
        <div className="text-sm">
          {children}
        </div>
      </div>
    </div>
  );
};

interface ServiceMetaRowProps {
  namespace: string;
  name: string;
  labels: Record<string, string>;
  status: "Low" | "High" | "Critical";
}

const ServiceMetaRow = ({ namespace, name, labels, status }: ServiceMetaRowProps) => {
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'Low':
        return 'bg-yellow-500/20 text-yellow-500 border-yellow-500';
      case 'High':
        return 'bg-orange-500/20 text-orange-500 border-orange-500';
      case 'Critical':
        return 'bg-destructive/20 text-destructive border-destructive';
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

  return (
    <div className="flex flex-col lg:flex-row gap-4 mb-6">
      {/* Namespace Box */}
      <ServiceMetaBox title="Namespace" aria-label={`Namespace: ${namespace}`}>
        <span className="font-medium text-foreground">{namespace}</span>
      </ServiceMetaBox>

      {/* Name Box */}
      <ServiceMetaBox title="Name" aria-label={`Service name: ${name}`}>
        <span className="font-semibold text-primary neon-text">{name}</span>
      </ServiceMetaBox>

      {/* Labels Box */}
      <ServiceMetaBox title="Labels" className="flex-1" aria-label="Service labels">
        <div className="flex flex-wrap gap-1">
          {Object.entries(labels).map(([key, value]) => (
            <Badge 
              key={key} 
              className="text-xs neon-badge"
            >
              {key}: {value}
            </Badge>
          ))}
        </div>
      </ServiceMetaBox>

      {/* Status Box */}
      <ServiceMetaBox title="Status" aria-label={`Status: ${status}`}>
        <Badge className={`${getStatusColor(status)} flex items-center gap-1 w-fit`}>
          {getStatusIcon(status)}
          {status}
        </Badge>
      </ServiceMetaBox>
    </div>
  );
};

export default ServiceMetaRow;
export { ServiceMetaBox };