import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useSecurityContext } from "@/contexts/SecurityContext";
import { Skeleton } from "@/components/ui/skeleton";
import { Shield, CheckCircle, AlertCircle } from "lucide-react";

const SecurityOverview = () => {
  const { score, isLoading } = useSecurityContext();

  if (isLoading || !score) {
    return (
      <Card className="neon-border bg-card">
        <CardContent className="p-6">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="text-center space-y-2">
              <Skeleton className="h-16 w-24 mx-auto" />
              <Skeleton className="h-4 w-32 mx-auto" />
            </div>
            <div className="space-y-3">
              <Skeleton className="h-6 w-40" />
              <Skeleton className="h-4 w-full" />
            </div>
            <div className="grid grid-cols-3 gap-4">
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
            </div>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (score.total === 0) {
    return (
      <Card className="neon-border bg-card">
        <CardContent className="p-10 text-center">
          <AlertCircle className="h-10 w-10 text-muted-foreground mx-auto mb-3" />
          <h3 className="text-lg font-semibold text-foreground mb-1">No Validation Data</h3>
          <p className="text-muted-foreground">Run validation to calculate security score from findings.</p>
        </CardContent>
      </Card>
    );
  }

  const getCriticalityColor = (level: string) => {
    switch (level) {
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

  const getCriticalityIcon = (level: string) => {
    switch (level) {
      case 'Low':
        return <CheckCircle className="h-4 w-4" />;
      case 'High':
        return <AlertCircle className="h-4 w-4" />;
      case 'Critical':
        return <AlertCircle className="h-4 w-4" />;
      default:
        return <Shield className="h-4 w-4" />;
    }
  };

  const narrativeText = `${score.counts.Critical} findings are Critical, ${score.counts.High} are High, ${score.counts.Low} are Low.`;

  return (
    <Card className="border-cyber-border bg-card glow-border">
      <CardContent className="p-6">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-center">
          {/* Left Column - Big Score Number */}
          <div className="text-center space-y-2">
            <h2 
              className="text-6xl font-bold " 
              aria-live="polite"
              aria-label={`Security score: ${score.score} percent`}
            >
              {score.score}%
            </h2>
            <p className="text-sm text-muted-foreground uppercase tracking-wide">
              Security Score
            </p>
          </div>

          {/* Middle Column - Criticality & Narrative */}
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Criticality Level:</span>
              <Badge 
                className={`${getCriticalityColor(score.criticality_level)} flex items-center gap-1`}
              >
                {getCriticalityIcon(score.criticality_level)}
                {score.criticality_level}
              </Badge>
            </div>
            <p className="text-sm text-green-muted">
              {narrativeText}
            </p>
          </div>

          {/* Right Column - Metric Tiles */}
          <div className="grid grid-cols-3 gap-4">
            <div className="text-center p-3 rounded-lg bg-secondary/50 ">
              <div className="text-2xl font-bold text-orange-500">
                {score.counts.Low}
              </div>
              <div className="text-xs text-muted-foreground uppercase tracking-wide">
                Low
              </div>
            </div>
            <div className="text-center p-3 rounded-lg bg-secondary/50 ">
              <div className="text-2xl font-bold text-yellow-500">
                {score.counts.High}
              </div>
              <div className="text-xs text-muted-foreground uppercase tracking-wide">
                High
              </div>
            </div>
            <div className="text-center p-3 rounded-lg bg-secondary/50 ">
              <div className="text-2xl font-bold text-[#FF0000]">
                {score.counts.Critical}
              </div>
              <div className="text-xs text-muted-foreground uppercase tracking-wide">
                Critical
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
};

export default SecurityOverview;