import { useMemo, useState, useEffect, useRef } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { CheckCircle2, Loader2, GitPullRequest, GitBranch, ExternalLink, AlertTriangle, Zap, XCircle, WifiOff } from "lucide-react";
import { K8sPostureFinding } from "@/types/security";
import { API_CONFIG } from "@/config/api";
import { getSessionToken } from "@/lib/data-transformers";
import { toast } from "sonner";
import { useNavigate } from "react-router-dom";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

interface ActionCardProps {
  finding: K8sPostureFinding;
  onApply: (finding: K8sPostureFinding, command: string) => Promise<{ success: boolean; message: string; output?: string }>;
}

type RemediationStatus = 'idle' | 'sending' | 'running' | 'success' | 'failed' | 'no-agent';

const severityToStatus = (severity: string): "Low" | "High" | "Critical" => {
  const normalized = (severity || "").toLowerCase();
  if (normalized === "critical") return "Critical";
  if (normalized === "high") return "High";
  return "Low";
};

const statusClassMap: Record<string, string> = {
  Critical: "bg-[#FF0000]/20 text-[#FF0000] border-[#FF0000]",
  High: "bg-yellow-500/20 text-yellow-500 border-yellow-500",
  Low: "bg-orange-500/20 text-orange-500 border-orange-500",
};

const ActionCard = ({ finding, onApply }: ActionCardProps) => {
  const [isRemediating, setIsRemediating] = useState(false);
  const [resultMessage, setResultMessage] = useState<string | null>(null);
  const [resultOutput, setResultOutput] = useState<string | null>(null);
  const navigate = useNavigate();

  // --- Phase 3: Remediation State ---
  const [remediationStatus, setRemediationStatus] = useState<RemediationStatus>('idle');
  const [executionId, setExecutionId] = useState<number | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // --- PR State ---
  const [showPRDialog, setShowPRDialog] = useState(false);
  const [branches, setBranches] = useState<{name: string; sha: string}[]>([]);
  const [selectedBranch, setSelectedBranch] = useState("");
  const [prTitle, setPrTitle] = useState("");
  const [prDescription, setPrDescription] = useState("");
  const [isLoadingBranches, setIsLoadingBranches] = useState(false);
  const [isCreatingPR, setIsCreatingPR] = useState(false);
  const [prResult, setPrResult] = useState<{pr_url: string; pr_number: number; branch_name: string} | null>(null);

  const status = severityToStatus(finding.severity);

  const currentPorts = useMemo(() => {
    const matches = `${finding.description || ""} ${finding.recommendation || ""}`.match(/\b\d{2,5}\b/g) || [];
    const ports = matches
      .map((value) => Number(value))
      .filter((value) => Number.isInteger(value) && value > 0 && value <= 65535);
    return Array.from(new Set(ports));
  }, [finding.description, finding.recommendation]);

  const issueType = (finding.issue_type || "").toLowerCase();
  const checkName = (finding.check_name || "").toLowerCase();
  const kind = (finding.kind || finding.missing_kind || "").toLowerCase();
  const shouldShowPorts = currentPorts.length > 0 && (
    issueType === "container-port" ||
    checkName.includes("service exposure") ||
    kind === "service"
  );

  // --- Phase 3: Poll remediation status ---
  useEffect(() => {
    if (remediationStatus !== 'running' || !executionId) return;

    pollRef.current = setInterval(async () => {
      try {
        const resp = await fetch(
          `${API_CONFIG.ENDPOINTS.SESSION.REMEDIATION_STATUS}?execution_id=${executionId}`
        );
        const data = await resp.json();
        if (data.success) {
          if (data.status === 'success') {
            setRemediationStatus('success');
            toast.success('Remediation completed successfully!');
            if (pollRef.current) clearInterval(pollRef.current);
          } else if (data.status === 'failed') {
            setRemediationStatus('failed');
            toast.error('Remediation failed. Check terminal for details.');
            if (pollRef.current) clearInterval(pollRef.current);
          }
        }
      } catch {
        // silently retry
      }
    }, 3000);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [remediationStatus, executionId]);

  // --- Remediate via agentic endpoint (Bedrock AI) ---
  const handleRemediate = async () => {
    if (isRemediating) return;

    setIsRemediating(true);
    setResultMessage(null);
    setResultOutput(null);

    try {
      const result = await onApply(finding, "remediate");
      setResultMessage(result.message);
      if (result.output) {
        setResultOutput(result.output);
      }
    } finally {
      setIsRemediating(false);
    }
  };

  // Take Action handler — navigates to Terminal page with finding_id
  const handleTakeAction = () => {
    const findingId = finding.finding_id || finding.resource_id || '';
    const terminalToken = localStorage.getItem('aegios_terminal_token') || '';
    const currentPath = window.location.pathname;
    
    if (terminalToken) {
      // Cluster already connected — go directly to terminal with finding_id
      navigate(`/security-service/terminal?finding_id=${encodeURIComponent(findingId)}&token=${encodeURIComponent(terminalToken)}&returnUrl=${encodeURIComponent(currentPath)}`);
    } else {
      // No cluster connected — go to terminal page to connect first, then auto-queue
      toast.info('Please connect your cluster first, then the fix will auto-execute.');
      navigate(`/security-service/terminal?finding_id=${encodeURIComponent(findingId)}&returnUrl=${encodeURIComponent(currentPath)}`);
    }
  };

  // --- Remediation Button Render ---
  const renderRemediateButton = () => {
    return (
      <Button
        onClick={handleRemediate}
        disabled={isRemediating}
        className="bg-primary hover:bg-primary/90 text-primary-foreground font-semibold"
      >
        {isRemediating ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <Zap className="h-4 w-4 mr-2" />}
        Remediate
      </Button>
    );
  };

  // --- PR Functions ---
  const fetchBranches = async () => {
    const sessionToken = getSessionToken();
    if (!sessionToken) return;

    setIsLoadingBranches(true);
    try {
      const resp = await fetch(API_CONFIG.ENDPOINTS.SECURITY.LIST_BRANCHES, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: sessionToken }),
      });
      const result = await resp.json();
      if (result.success && result.data?.branches) {
        setBranches(result.data.branches);
        if (result.data.branches.length > 0 && !selectedBranch) {
          setSelectedBranch(result.data.branches[0].name);
        }
      } else {
        toast.error(result.message || 'Failed to fetch branches');
      }
    } catch (err) {
      console.error('Failed to fetch branches:', err);
      toast.error('Failed to fetch branches');
    } finally {
      setIsLoadingBranches(false);
    }
  };

  const openPRDialog = () => {
    const resourceName = finding.name || finding.kind || "resource";
    const ns = finding.namespace || "default";
    setPrTitle(`[Aegios] Fix ${finding.kind || "resource"} ${resourceName} in ${ns}`);
    setPrDescription(
      `Security remediation for ${resourceName} in namespace ${ns}.\n\n` +
      `**Issue:** ${finding.description || "Security finding"}\n` +
      `**Recommendation:** ${finding.recommendation || "Apply fix"}`
    );
    setPrResult(null);
    setShowPRDialog(true);
    fetchBranches();
  };

  const handleRaisePR = async () => {
    if (!selectedBranch) return;

    const sessionToken = getSessionToken();
    if (!sessionToken) {
      toast.error('Not authenticated');
      return;
    }

    const resourceId = finding.resource_id || finding.finding_id;
    if (!resourceId) {
      toast.error('No resource ID available');
      return;
    }

    // Build a YAML fix from the recommendation
    const fixedYaml = finding.recommendation || `# Fix for ${finding.kind || "resource"} ${finding.name}\n# ${finding.description}`;

    setIsCreatingPR(true);
    try {
      const resp = await fetch(API_CONFIG.ENDPOINTS.SECURITY.RAISE_PR, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_token: sessionToken,
          resource_id: resourceId,
          fixed_yaml: fixedYaml,
          target_branch: selectedBranch,
          title: prTitle,
          description: prDescription,
        }),
      });
      const result = await resp.json();
      if (result.success && result.data) {
        setPrResult(result.data);
        toast.success('Pull request created successfully!');
      } else {
        toast.error(result.message || 'Failed to create PR');
      }
    } catch (err) {
      console.error('Failed to create PR:', err);
      toast.error('Failed to create pull request');
    } finally {
      setIsCreatingPR(false);
    }
  };

  return (
    <>
      <Card className="border-cyber-border bg-card glow-border transition-colors hover:border-primary">
        <CardContent className="p-6 space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Namespace</p>
              <p className="text-sm font-medium text-foreground">{finding.namespace || "default"}</p>
            </div>
            <div>
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Name</p>
              <p className="text-sm font-semibold text-primary">{finding.name || finding.kind || "Unknown"}</p>
            </div>
            <div>
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Labels</p>
              <div className="flex flex-wrap gap-2 mt-1">
                <Badge className="text-xs neon-badge">kind: {finding.kind || finding.missing_kind || "unknown"}</Badge>
                {finding.repo_name && (
                  <Badge className="text-xs neon-badge">repo: {finding.repo_name}</Badge>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">Status:</span>
            <Badge className={`flex items-center gap-1 w-fit border ${statusClassMap[status]}`}>
              {status}
            </Badge>
          </div>

          <div className="rounded-lg border border-primary/40 bg-secondary/10 p-4 space-y-2">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Current Config</p>
            <p className="text-sm text-foreground whitespace-pre-wrap">{finding.description || "No issue details available."}</p>
            {shouldShowPorts && (
              <div className="space-y-2">
                <p className="text-sm text-muted-foreground">Exposed Ports:</p>
                <div className="flex flex-wrap gap-2">
                  {currentPorts.map((port) => (
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
          </div>

          <div className="rounded-lg border border-primary/40 bg-secondary/10 p-4 space-y-2">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Recommendation</p>
            <p className="text-sm text-foreground whitespace-pre-wrap">{finding.recommendation || "No recommendation provided."}</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2">
            <Button
              onClick={openPRDialog}
              className="bg-[#238636] hover:bg-[#2ea043] text-white font-semibold"
            >
              <GitPullRequest className="h-4 w-4 mr-2" />
              Raise PR
            </Button>
            <Button 
              onClick={handleTakeAction}
              className="bg-primary hover:bg-primary/90 text-primary-foreground font-semibold"
            >
              Take Action
            </Button>
            {renderRemediateButton()}
          </div>

          {resultMessage && (
            <div className="flex items-center gap-2 text-sm text-primary border border-primary/40 rounded-lg p-3 bg-secondary/10 mt-2">
              <CheckCircle2 className="h-4 w-4" />
              <span>{resultMessage}</span>
            </div>
          )}

          {/* AI Remediation Output */}
          {resultOutput && (
            <div className="mt-4 p-4 rounded-lg bg-black border border-green-500/40">
              <p className="text-xs uppercase tracking-wide text-green-500 mb-2">Agentic Remediation</p>
              <pre className="text-sm text-green-300 font-mono whitespace-pre-wrap overflow-x-auto">
                {resultOutput}
              </pre>
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── Raise PR Dialog ── */}
      <Dialog open={showPRDialog} onOpenChange={setShowPRDialog}>
        <DialogContent className="sm:max-w-[560px] neon-border bg-card">
          <DialogHeader>
            <DialogTitle className="text-primary neon-text flex items-center gap-2">
              <GitPullRequest className="h-5 w-5" />
              Raise Pull Request
            </DialogTitle>
            <DialogDescription>
              Push your remediation as a PR to your GitHub repository
            </DialogDescription>
          </DialogHeader>

          {prResult ? (
            <div className="py-6 text-center space-y-4">
              <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-[#238636]/20 mx-auto">
                <GitPullRequest className="h-8 w-8 text-[#238636]" />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-foreground">PR #{prResult.pr_number} Created!</h3>
                <p className="text-sm text-muted-foreground mt-1">
                  Branch: <code className="text-xs bg-secondary/50 px-1.5 py-0.5 rounded">{prResult.branch_name}</code>
                </p>
              </div>
              <Button
                onClick={() => window.open(prResult.pr_url, '_blank')}
                className="bg-[#238636] hover:bg-[#2ea043] text-white"
              >
                <ExternalLink className="h-4 w-4 mr-2" />
                View PR on GitHub
              </Button>
            </div>
          ) : (
            <div className="space-y-4 py-4">
              {/* Branch Picker */}
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide flex items-center gap-1.5">
                  <GitBranch className="h-3.5 w-3.5" />
                  Target Branch
                </label>
                {isLoadingBranches ? (
                  <div className="flex items-center gap-2 p-3 rounded-lg bg-secondary/30 neon-border">
                    <Loader2 className="h-4 w-4 animate-spin text-primary" />
                    <span className="text-sm text-muted-foreground">Loading branches...</span>
                  </div>
                ) : (
                  <select
                    value={selectedBranch}
                    onChange={(e) => setSelectedBranch(e.target.value)}
                    className="w-full p-2.5 rounded-lg bg-secondary/50 border border-cyber-border text-foreground text-sm focus:border-primary focus:outline-none transition-colors"
                  >
                    {branches.map((b) => (
                      <option key={b.name} value={b.name}>
                        {b.name}
                      </option>
                    ))}
                  </select>
                )}
              </div>

              {/* PR Title */}
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  PR Title
                </label>
                <input
                  type="text"
                  value={prTitle}
                  onChange={(e) => setPrTitle(e.target.value)}
                  className="w-full p-2.5 rounded-lg bg-secondary/50 border border-cyber-border text-foreground text-sm focus:border-primary focus:outline-none transition-colors"
                  placeholder="[Aegios] Fix security issue..."
                />
              </div>

              {/* PR Description */}
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  Description
                </label>
                <Textarea
                  value={prDescription}
                  onChange={(e) => setPrDescription(e.target.value)}
                  className="min-h-[80px] bg-secondary/50 neon-border text-sm resize-none"
                  placeholder="Describe the security fix..."
                />
              </div>

              {/* Info Box */}
              <div className="p-3 rounded-lg bg-primary/5 border border-primary/20">
                <p className="text-xs text-muted-foreground">
                  A new branch <code className="text-primary">aegios/fix-*</code> will be created from{" "}
                  <code className="text-primary">{selectedBranch || "..."}</code>, the fix will be committed, and a PR will be opened.
                </p>
              </div>
            </div>
          )}

          {!prResult && (
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => setShowPRDialog(false)}
                className="neon-border"
              >
                Cancel
              </Button>
              <Button
                onClick={handleRaisePR}
                disabled={isCreatingPR || !selectedBranch || isLoadingBranches}
                className="bg-[#238636] hover:bg-[#2ea043] text-white"
              >
                {isCreatingPR ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    Creating PR...
                  </>
                ) : (
                  <>
                    <GitPullRequest className="h-4 w-4 mr-2" />
                    Create Pull Request
                  </>
                )}
              </Button>
            </DialogFooter>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
};

export default ActionCard;
