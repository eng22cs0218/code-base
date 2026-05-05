import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Service } from "@/types/security";
import ServiceMetaRow from "./ServiceMetaBox";

interface ActionsDetailBoxProps {
  service: Service;
}

const ActionsDetailBox = ({ service }: ActionsDetailBoxProps) => {
  return (
    <Card className="neon-border bg-card">
      <CardHeader>
        <CardTitle className="text-primary neon-text">Service Details</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Service Meta Row */}
        <ServiceMetaRow 
          namespace={service.namespace}
          name={service.name}
          labels={service.labels}
          status={service.status}
        />

        {/* Current Status and Recommendations */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Current Status */}
          <div className="space-y-4">
            <h4 className="text-sm font-semibold text-primary neon-text uppercase tracking-wide">
              Current Status
            </h4>
            <div className="p-4 rounded-lg bg-secondary/30 neon-border">
              <div className="space-y-3">
                <div>
                  <p className="text-xs text-muted-foreground mb-2">Exposed Ports:</p>
                  <div className="flex flex-wrap gap-2">
                    {service.ports.map((port) => (
                      <span 
                        key={port}
                        className="text-lg font-bold text-destructive bg-destructive/10 border border-destructive/30 rounded px-3 py-1"
                      >
                        {port}
                      </span>
                    ))}
                  </div>
                </div>
                
                {service.metadata.owner && (
                  <div>
                    <p className="text-xs text-muted-foreground">Owner:</p>
                    <p className="text-sm text-foreground font-medium">{service.metadata.owner}</p>
                  </div>
                )}

                <div>
                  <p className="text-xs text-muted-foreground">Created:</p>
                  <p className="text-sm text-foreground">
                    {new Date(service.created_at).toLocaleString()}
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Recommendations */}
          <div className="space-y-4">
            <h4 className="text-sm font-semibold text-primary neon-text uppercase tracking-wide">
              Security Recommendations
            </h4>
            <div className="p-4 rounded-lg bg-secondary/30 neon-border">
              {service.recommendations.length > 0 ? (
                <ul className="space-y-3">
                  {service.recommendations.map((recommendation, index) => (
                    <li 
                      key={index}
                      className="flex items-start gap-3 p-3 rounded bg-destructive/10 border border-destructive/30"
                    >
                      <span className="text-destructive mt-1 font-bold">•</span>
                      <span className="text-sm text-destructive font-medium">
                        {recommendation}
                      </span>
                    </li>
                  ))}
                </ul>
              ) : (
                <div className="text-center py-4">
                  <p className="text-sm text-primary neon-text font-medium">
                    ✓ No recommendations - service is secure
                  </p>
                </div>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
};

export default ActionsDetailBox;