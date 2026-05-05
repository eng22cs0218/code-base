import { useState, useEffect } from "react";
import { Service } from "@/types/security";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ChevronDown, ChevronRight, AlertTriangle, AlertCircle } from "lucide-react";

interface NamespaceGroupListProps {
  services: Service[];
  selectedServiceId?: string;
  onServiceSelect: (serviceId: string) => void;
}

const NamespaceGroupList = ({ services, selectedServiceId, onServiceSelect }: NamespaceGroupListProps) => {
  const [expandedNamespaces, setExpandedNamespaces] = useState<Set<string>>(new Set());

  // Auto-expand namespace containing selected service
  useEffect(() => {
    if (selectedServiceId) {
      const selectedService = services.find(s => s.id === selectedServiceId);
      if (selectedService) {
        setExpandedNamespaces(prev => new Set([...prev, selectedService.namespace]));
      }
    }
  }, [selectedServiceId, services]);

  // Group services by namespace
  const servicesByNamespace = services.reduce((acc, service) => {
    if (!acc[service.namespace]) {
      acc[service.namespace] = [];
    }
    acc[service.namespace].push(service);
    return acc;
  }, {} as Record<string, Service[]>);

  const toggleNamespace = (namespace: string) => {
    const newExpanded = new Set(expandedNamespaces);
    if (newExpanded.has(namespace)) {
      newExpanded.delete(namespace);
    } else {
      newExpanded.add(namespace);
    }
    setExpandedNamespaces(newExpanded);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'Low':
        return <AlertTriangle className="h-3 w-3 text-yellow-500" />;
      case 'High':
        return <AlertTriangle className="h-3 w-3 text-orange-500" />;
      case 'Critical':
        return <AlertCircle className="h-3 w-3 text-destructive" />;
      default:
        return <AlertCircle className="h-3 w-3 text-muted-foreground" />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'Low':
        return 'text-yellow-500';
      case 'High':
        return 'text-orange-500';
      case 'Critical':
        return 'text-destructive';
      default:
        return 'text-muted-foreground';
    }
  };

  return (
    <Card className="neon-border bg-card h-[calc(100vh-12rem)]">
      <CardContent className="p-4">
        <div className="space-y-2 overflow-y-auto h-full">
          <h3 className="text-lg font-semibold text-primary neon-text mb-4">Services by Namespace</h3>
          
          {Object.entries(servicesByNamespace).map(([namespace, namespaceServices]) => (
            <Collapsible
              key={namespace}
              open={expandedNamespaces.has(namespace)}
              onOpenChange={() => toggleNamespace(namespace)}
            >
              <CollapsibleTrigger className="w-full">
                <div className="flex items-center justify-between p-3 rounded-lg bg-secondary/50 hover:bg-secondary/70 transition-colors neon-border">
                  <div className="flex items-center gap-2">
                    {expandedNamespaces.has(namespace) ? (
                      <ChevronDown className="h-4 w-4 text-primary" />
                    ) : (
                      <ChevronRight className="h-4 w-4 text-primary" />
                    )}
                    <span className="font-medium text-foreground">{namespace}</span>
                  </div>
                  <Badge variant="secondary" className="text-xs">
                    {namespaceServices.length}
                  </Badge>
                </div>
              </CollapsibleTrigger>
              
              <CollapsibleContent>
                <div className="ml-6 mt-2 space-y-1">
                  {namespaceServices.map((service) => (
                    <div
                      key={service.id}
                      onClick={() => onServiceSelect(service.id)}
                      className={`p-3 rounded-lg cursor-pointer transition-all duration-200 ${
                        selectedServiceId === service.id
                          ? 'bg-primary/20 neon-border border-primary'
                          : 'bg-secondary/30 hover:bg-secondary/50 border border-transparent hover:border-cyber-border'
                      }`}
                      role="button"
                      tabIndex={0}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          onServiceSelect(service.id);
                        }
                      }}
                      aria-label={`Select service ${service.name} in ${service.namespace} namespace`}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {getStatusIcon(service.status)}
                          <span className={`font-medium ${
                            selectedServiceId === service.id ? 'text-primary neon-text' : 'text-foreground'
                          }`}>
                            {service.name}
                          </span>
                        </div>
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                          <span className={getStatusColor(service.status)}>
                            {service.status}
                          </span>
                          {service.ports.length > 0 && (
                            <span>• {service.ports.length} ports</span>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </CollapsibleContent>
            </Collapsible>
          ))}
        </div>
      </CardContent>
    </Card>
  );
};

export default NamespaceGroupList;