import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useSecurityContext } from "@/contexts/SecurityContext";
import { Skeleton } from "@/components/ui/skeleton";
import { Shield, AlertCircle, CheckCircle } from "lucide-react";

const Scoreboard = () => {
  const { score, isLoading } = useSecurityContext();

  if (isLoading || !score) {
    return (
      <Card className="border-cyber-border bg-card glow-border">
        <CardHeader>
          <Skeleton className="h-6 w-48" />
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
        </CardContent>
      </Card>
    );
  }

  const getCriticalityColor = (level: string) => {
    switch (level) {
      case 'Low':
        return 'text-primary';
      case 'High':
        return 'text-orange-500';
      case 'Critical':
        return 'text-destructive';
      default:
        return 'text-muted-foreground';
    }
  };

  const getCriticalityIcon = (level: string) => {
    switch (level) {
      case 'Low':
        return <CheckCircle className="h-5 w-5 text-primary" />;
      case 'High':
        return <AlertCircle className="h-5 w-5 text-orange-500" />;
      case 'Critical':
        return <Shield className="h-5 w-5 text-destructive" />;
      default:
        return <Shield className="h-5 w-5 text-muted-foreground" />;
    }
  };

  return (
    <Card className="border-cyber-border bg-card glow-border">
      <CardHeader>
        <CardTitle className="text-primary glow-text flex items-center gap-2">
          <Shield className="h-6 w-6" />
          Security Overview
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Security Score */}
        <div className="text-center space-y-2">
          <div className="text-4xl font-bold text-primary glow-text">
            {score.score}%
          </div>
          <p className="text-sm text-muted-foreground">Security Score</p>
        </div>

        {/* Criticality Level */}
        <div className="flex items-center justify-center gap-2">
          {getCriticalityIcon(score.criticality_level)}
          <span className="text-sm text-muted-foreground">Criticality Level:</span>
          <Badge 
            variant={score.criticality_level === 'High' ? 'destructive' : 'secondary'}
            className={`${getCriticalityColor(score.criticality_level)} font-semibold`}
          >
            {score.criticality_level}
          </Badge>
        </div>

        {/* Status Summary */}
        <div className="space-y-3">
          <p className="text-center text-foreground">
            <span className="text-destructive font-semibold">{score.counts.Critical}</span> services are Critical,{' '}
            <span className="text-orange-500 font-semibold">{score.counts.High}</span> High,{' '}
            <span className="text-yellow-500 font-semibold">{score.counts.Low}</span> Low.
          </p>
          
          <div className="grid grid-cols-3 gap-4 text-center">
            <div className="space-y-1">
              <div className="text-2xl font-bold text-yellow-500">
                {score.counts.Low}
              </div>
              <div className="text-xs text-muted-foreground">Low</div>
            </div>
            <div className="space-y-1">
              <div className="text-2xl font-bold text-orange-500">
                {score.counts.High}
              </div>
              <div className="text-xs text-muted-foreground">High</div>
            </div>
            <div className="space-y-1">
              <div className="text-2xl font-bold text-destructive">
                {score.counts.Critical}
              </div>
              <div className="text-xs text-muted-foreground">Critical</div>
            </div>
          </div>
        </div>

        {/* Total Services */}
        <div className="text-center pt-4 border-t border-cyber-border">
          <p className="text-sm text-muted-foreground">
            Total Services: <span className="text-primary font-semibold">{score.total}</span>
          </p>
        </div>
      </CardContent>
    </Card>
  );
};

export default Scoreboard;