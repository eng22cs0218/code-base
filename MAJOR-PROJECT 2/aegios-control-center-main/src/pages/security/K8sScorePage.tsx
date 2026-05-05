import { useEffect } from "react";
import { useSecurityContext } from "@/contexts/SecurityContext";
import ScoreVisuals from "@/components/security/ScoreVisuals";
import SecurityOverview from "@/components/security/SecurityOverview";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Loader2, CheckCircle2, AlertCircle } from "lucide-react";

const K8sScorePage = () => {
  const { refreshData, runValidation, loadingValidation, validationStatus } = useSecurityContext();

  useEffect(() => {
    // Fetch data when page loads
    refreshData();
  }, [refreshData]);

  const handleRunValidation = async () => {
    await runValidation();
  };

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-3xl font-bold text-primary">K8s Security Score</h1>
        <p className="text-muted-foreground">
          Monitor your Kubernetes cluster security posture and overall health metrics.
        </p>
      </div>

      <Card className="border-cyber-border bg-card glow-border">
        <CardHeader>
          <CardTitle className="text-primary">Namespace Validation</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 p-6 pt-0">
          <p className="text-muted-foreground">
            Run validation checks to detect missing Kubernetes resources, RBAC issues, network policies, and other security misconfigurations.
          </p>

          <div className="flex items-center gap-3">
            <Button
              onClick={handleRunValidation}
              disabled={loadingValidation}
              className="border-primary bg-primary text-black hover:bg-primary/90 disabled:opacity-70"
            >
              {loadingValidation ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Running Validation...
                </>
              ) : (
                "Run Validation"
              )}
            </Button>

            {validationStatus === 'success' && (
              <span className="inline-flex items-center text-sm text-primary">
                <CheckCircle2 className="mr-1 h-4 w-4" />
                Validation completed
              </span>
            )}

            {validationStatus === 'error' && (
              <span className="inline-flex items-center text-sm text-red-500">
                <AlertCircle className="mr-1 h-4 w-4" />
                Validation failed
              </span>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Charts Row - Only Pie and Bar */}
      <ScoreVisuals />

      {/* Divider */}
      <hr className="border-cyber-border neon-divider my-6" />

      {/* Security Overview Section */}
      <div className="space-y-2">
        <h2 className="text-xl font-semibold text-primary">Security Overview</h2>
        <SecurityOverview />
      </div>
    </div>
  );
};

export default K8sScorePage;