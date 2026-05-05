import { useState, useEffect } from "react";  
import { useParams, useNavigate } from "react-router-dom";
import { useSecurityContext } from "@/contexts/SecurityContext";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { AlertCircle, AlertTriangle, ChevronDown, ChevronRight, Play, Eye, GitPullRequest, GitBranch, ExternalLink, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { API_CONFIG } from "@/config/api";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

const K8sActionsPage = () => {
  const { serviceId } = useParams<{ serviceId: string }>();
  const navigate = useNavigate();
  const { services, applyAction, isLoading, refreshData } = useSecurityContext();
  
  const [selectedServiceId, setSelectedServiceId] = useState<string | undefined>(serviceId);
  const [expandedNamespaces, setExpandedNamespaces] = useState<Set<string>>(new Set());
  const [command, setCommand] = useState("");
  const [isApplying, setIsApplying] = useState(false);
  const [output, setOutput] = useState<string | null>(null);

  useEffect(() => {
    // Fetch data when page loads
    refreshData();
  }, [refreshData]);
  const [isOutputOpen, setIsOutputOpen] = useState(false);
  const [showPreview, setShowPreview] = useState(false);
  const [previewData, setPreviewData] = useState<{
    service: string;
    namespace: string;
    command: string;
    impact: string;
    warnings: string[];
  } | null>(null);

  // --- PR State ---
  const [showPRDialog, setShowPRDialog] = useState(false);
  const [branches, setBranches] = useState<{name: string; sha: string}[]>([]);
  const [selectedBranch, setSelectedBranch] = useState("");
  const [prTitle, setPrTitle] = useState("");
  const [prDescription, setPrDescription] = useState("");
  const [isLoadingBranches, setIsLoadingBranches] = useState(false);
  const [isCreatingPR, setIsCreatingPR] = useState(false);
  const [prResult, setPrResult] = useState<{pr_url: string; pr_number: number; branch_name: string} | null>(null);

  const selectedService = selectedServiceId ? services.find(s => s.id === selectedServiceId) : undefined;

  // Group services by namespace
  const servicesByNamespace = services.reduce((acc, service) => {
    if (!acc[service.namespace]) {
      acc[service.namespace] = [];
    }
    acc[service.namespace].push(service);
    return acc;
  }, {} as Record<string, typeof services>);

  useEffect(() => {
    if (serviceId && services.length > 0) {
      setSelectedServiceId(serviceId);
      const service = services.find(s => s.id === serviceId);
      if (service) {
        setExpandedNamespaces(prev => new Set([...prev, service.namespace]));
      }
    }
  }, [serviceId, services]);

  const handleServiceSelect = (newServiceId: string) => {
    setSelectedServiceId(newServiceId);
    navigate(`/security/k8s-actions/${newServiceId}`);
  };

  const toggleNamespace = (namespace: string) => {
    const newExpanded = new Set(expandedNamespaces);
    if (newExpanded.has(namespace)) {
      newExpanded.delete(namespace);
    } else {
      newExpanded.add(namespace);
    }
    setExpandedNamespaces(newExpanded);
  };

  const handlePreview = () => {
    if (!command.trim() || !selectedService) return;

    // Generate preview data
    const warnings: string[] = [];
    const commandLower = command.toLowerCase();

    if (commandLower.includes('delete') || commandLower.includes('remove')) {
      warnings.push('⚠️ This action will DELETE resources - cannot be undone');
    }
    if (commandLower.includes('privileged')) {
      warnings.push('⚠️ Modifying privileged settings affects security posture');
    }
    if (commandLower.includes('scale')) {
      warnings.push('ℹ️ Scaling will affect resource availability');
    }
    if (selectedService.status === 'Critical') {
      warnings.push('⚠️ Service has critical security issues - review carefully');
    }

    setPreviewData({
      service: selectedService.name,
      namespace: selectedService.namespace,
      command: command.trim(),
          impact: commandLower.includes('delete') ? 'CRITICAL' : 
            commandLower.includes('scale') || commandLower.includes('update') ? 'HIGH' : 'LOW',
      warnings: warnings.length > 0 ? warnings : ['✓ No major warnings detected'],
    });
    setShowPreview(true);
  };

  const handleConfirmApply = async () => {
    setShowPreview(false);
    await handleApply();
  };

  const handleApply = async () => {
    if (!command.trim() || !selectedServiceId) return;

    setIsApplying(true);
    try {
      const result = await applyAction(selectedServiceId, command.trim());
      if (result.success) {
        setOutput(result.output || result.message);
        setIsOutputOpen(true);
        setCommand("");
        toast.success("Command applied successfully");
      }
    } catch (error) {
      console.error("Failed to apply action:", error);
      toast.error("Failed to apply command");
    } finally {
      setIsApplying(false);
    }
  };

  // --- PR Functions ---
  const fetchBranches = async () => {
    const sessionToken = localStorage.getItem('aegios_session_token');
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
    if (!selectedService) return;
    setPrTitle(`[Aegios] Fix security issues for ${selectedService.name}`);
    setPrDescription(`Security remediation for ${selectedService.name} in namespace ${selectedService.namespace}`);
    setPrResult(null);
    setShowPRDialog(true);
    fetchBranches();
  };

  const handleRaisePR = async () => {
    if (!selectedServiceId || !selectedBranch || !output) return;

    const sessionToken = localStorage.getItem('aegios_session_token');
    if (!sessionToken) {
      toast.error('Not authenticated');
      return;
    }

    setIsCreatingPR(true);
    try {
      const resp = await fetch(API_CONFIG.ENDPOINTS.SECURITY.RAISE_PR, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_token: sessionToken,
          resource_id: selectedServiceId,
          fixed_yaml: output,
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
        return 'bg-yellow-500/20 text-yellow-500 border-yellow-500';
      case 'High':
        return 'bg-orange-500/20 text-orange-500 border-orange-500';
      case 'Critical':
        return 'bg-destructive/20 text-destructive border-destructive';
      default:
        return 'bg-muted/20 text-muted-foreground border-muted';
    }
  };

  if (isLoading) {
    return (
      <div className="min-h-[calc(100vh-8rem)] space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-8 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="space-y-4">
          <Skeleton className="h-64 w-full" />
          <Skeleton className="h-32 w-full" />
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-[calc(100vh-8rem)] space-y-6">
      {/* Header */}
      <div className="space-y-2">
        <h1 className="text-3xl font-bold text-primary">K8s Actions</h1>
        <p className="text-muted-foreground">
          Select a service from the list below to view details and apply security actions.
        </p>
      </div>

      {/* Vertical Layout */}
      <div className="space-y-6">
        {/* Top - Services List */}
        <Card className="neon-border bg-card">
          <CardHeader>
            <CardTitle className="text-primary neon-text">Services by Namespace</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-h-[40vh] overflow-y-auto space-y-2">
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
                          onClick={() => handleServiceSelect(service.id)}
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
                              handleServiceSelect(service.id);
                            }
                          }}
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
                              <Badge className={`${getStatusColor(service.status)} text-xs`}>
                                {service.status}
                              </Badge>
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

        {/* Bottom - Selected Service Details */}
        {selectedService ? (
          <div className="space-y-6">
            {/* Service Details - Three Sub-boxes */}
            <Card className="neon-border bg-card">
              <CardHeader>
                <CardTitle className="text-primary neon-text">Service Details</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                {/* Composite Meta Box */}
                <div className="p-4 rounded-lg bg-secondary/30 neon-border">
                  <h4 className="text-sm font-semibold text-primary neon-text uppercase tracking-wide mb-3">
                    Service Information
                  </h4>
                  <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
                    <div>
                      <p className="text-xs text-muted-foreground uppercase tracking-wide">Namespace</p>
                      <p className="text-sm font-medium text-foreground">{selectedService.namespace}</p>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground uppercase tracking-wide">Name</p>
                      <p className="text-sm font-semibold text-primary neon-text">{selectedService.name}</p>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Status</p>
                      <Badge className={`${getStatusColor(selectedService.status)} flex items-center gap-1 w-fit`}>
                        {getStatusIcon(selectedService.status)}
                        {selectedService.status}
                      </Badge>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Labels</p>
                      <div className="flex flex-wrap gap-1">
                        {Object.entries(selectedService.labels).slice(0, 2).map(([key, value]) => (
                          <Badge key={key} className="text-xs neon-badge">
                            {key}: {value}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Current Status Box */}
                <div className="p-4 rounded-lg bg-secondary/30 neon-border">
                  <h4 className="text-sm font-semibold text-primary neon-text uppercase tracking-wide mb-3">
                    Current Status
                  </h4>
                  <div className="space-y-3">
                    <div>
                      <p className="text-xs text-muted-foreground mb-2">Exposed Ports:</p>
                      <div className="flex flex-wrap gap-2">
                        {selectedService.ports.map((port) => (
                          <span 
                            key={port}
                            className="text-lg font-bold text-destructive bg-destructive/10 border border-destructive/30 rounded px-3 py-1"
                          >
                            {port}
                          </span>
                        ))}
                      </div>
                    </div>
                    {selectedService.metadata.owner && (
                      <div>
                        <p className="text-xs text-muted-foreground">Owner:</p>
                        <p className="text-sm text-foreground font-medium">{selectedService.metadata.owner}</p>
                      </div>
                    )}
                  </div>
                </div>

                {/* Recommendations Box */}
                <div className="p-4 rounded-lg bg-secondary/30 neon-border">
                  <h4 className="text-sm font-semibold text-primary neon-text uppercase tracking-wide mb-3">
                    Security Recommendations
                  </h4>
                  <div className="max-h-32 overflow-y-auto">
                    {selectedService.recommendations.length > 0 ? (
                      <ul className="space-y-2">
                        {selectedService.recommendations.map((recommendation, index) => (
                          <li 
                            key={index}
                            className="flex items-start gap-3 p-2 rounded bg-destructive/10 border border-destructive/30"
                          >
                            <span className="text-destructive mt-1 font-bold">•</span>
                            <span className="text-sm text-destructive font-medium">
                              {recommendation}
                            </span>
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <p className="text-sm text-primary neon-text font-medium text-center py-2">
                        ✓ No recommendations - service is secure
                      </p>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Command Input */}
            <Card className="neon-border bg-card">
              <CardHeader>
                <CardTitle className="text-primary">Apply Security Action</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="relative">
                  <Textarea
                    placeholder="Type your command or notes here..."
                    value={command}
                    onChange={(e) => setCommand(e.target.value)}
                    className="min-h-[120px] bg-secondary/50 neon-border focus:border-primary resize-none pr-36"
                    disabled={isApplying}
                    aria-label={`Command input for service ${selectedService.name}`}
                  />
                  <div className="absolute bottom-3 right-3 flex gap-2">
                    <Button
                      onClick={handlePreview}
                      disabled={!command.trim() || isApplying}
                      className="bg-secondary hover:bg-secondary/80 text-foreground neon-border"
                      size="sm"
                    >
                      <Eye className="h-4 w-4 mr-2" />
                      Preview
                    </Button>
                    <Button
                      onClick={handleApply}
                      disabled={!command.trim() || isApplying}
                      className="bg-primary hover:bg-primary/80 text-primary-foreground neon-border"
                      size="sm"
                    >
                      {isApplying ? (
                        <>
                          <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary-foreground mr-2" />
                          Applying...
                        </>
                      ) : (
                        <>
                          <Play className="h-4 w-4 mr-2" />
                          Apply
                        </>
                      )}
                    </Button>
                  </div>
                </div>
                
                <p className="text-xs text-muted-foreground">
                  Enter kubectl commands, security policies, or configuration changes to apply to this service.
                </p>
              </CardContent>
            </Card>

            {/* Output Panel */}
            {output && (
              <Collapsible open={isOutputOpen} onOpenChange={setIsOutputOpen}>
                <Card className="neon-border bg-card">
                  <CollapsibleTrigger asChild>
                    <CardHeader className="cursor-pointer hover:bg-secondary/50 transition-colors">
                      <div className="flex items-center justify-between">
                        <CardTitle className="text-primary neon-text">Command Output</CardTitle>
                        {isOutputOpen ? (
                          <ChevronDown className="h-5 w-5 text-primary" />
                        ) : (
                          <ChevronRight className="h-5 w-5 text-primary" />
                        )}
                      </div>
                    </CardHeader>
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <CardContent className="space-y-4">
                      <pre className="bg-secondary/50 neon-border rounded p-4 text-sm text-foreground overflow-x-auto whitespace-pre-wrap">
                        {output}
                      </pre>
                      {/* Raise PR Button */}
                      <div className="flex items-center gap-3">
                        <Button
                          onClick={openPRDialog}
                          className="bg-[#238636] hover:bg-[#2ea043] text-white"
                        >
                          <GitPullRequest className="h-4 w-4 mr-2" />
                          Raise PR to GitHub
                        </Button>
                        <p className="text-xs text-muted-foreground">
                          Push this fix as a Pull Request to your repository
                        </p>
                      </div>
                    </CardContent>
                  </CollapsibleContent>
                </Card>
              </Collapsible>
            )}
          </div>
        ) : (
          /* Empty State */
          <Card className="neon-border bg-card">
            <CardContent className="p-12 text-center">
              <AlertCircle className="h-16 w-16 text-muted-foreground mx-auto mb-6" />
              <h3 className="text-xl font-semibold text-foreground mb-3">
                Select a Service
              </h3>
              <p className="text-muted-foreground max-w-md mx-auto">
                Choose a service from the list above to view its details and apply security actions.
              </p>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Preview Dialog */}
      <Dialog open={showPreview} onOpenChange={setShowPreview}>
        <DialogContent className="sm:max-w-[600px] neon-border bg-card">
          <DialogHeader>
            <DialogTitle className="text-primary neon-text">Action Preview</DialogTitle>
            <DialogDescription>
              Review the action before applying it to the service
            </DialogDescription>
          </DialogHeader>
          
          {previewData && (
            <div className="space-y-4 py-4">
              {/* Target Service */}
              <div className="p-4 rounded-lg bg-secondary/30 neon-border">
                <h4 className="text-sm font-semibold text-primary mb-2">Target Service</h4>
                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div>
                    <span className="text-muted-foreground">Service:</span>
                    <span className="ml-2 font-medium text-foreground">{previewData.service}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Namespace:</span>
                    <span className="ml-2 font-medium text-foreground">{previewData.namespace}</span>
                  </div>
                </div>
              </div>

              {/* Command */}
              <div className="p-4 rounded-lg bg-secondary/30 neon-border">
                <h4 className="text-sm font-semibold text-primary mb-2">Command</h4>
                <pre className="text-sm text-foreground bg-secondary/50 p-3 rounded overflow-x-auto">
                  {previewData.command}
                </pre>
              </div>

              {/* Impact Level */}
              <div className="p-4 rounded-lg bg-secondary/30 neon-border">
                <h4 className="text-sm font-semibold text-primary mb-2">Impact Level</h4>
                <Badge className={`
                  ${previewData.impact === 'CRITICAL' ? 'bg-destructive/20 text-destructive border-destructive' : ''}
                  ${previewData.impact === 'HIGH' ? 'bg-destructive/20 text-destructive border-destructive' : ''}
                  ${previewData.impact === 'LOW' ? 'bg-primary/20 text-primary border-primary' : ''}
                `}>
                  {previewData.impact}
                </Badge>
              </div>

              {/* Warnings */}
              <div className="p-4 rounded-lg bg-secondary/30 neon-border">
                <h4 className="text-sm font-semibold text-primary mb-2">Warnings & Notes</h4>
                <ul className="space-y-2">
                  {previewData.warnings.map((warning, index) => (
                    <li key={index} className="text-sm text-foreground flex items-start gap-2">
                      <span className="mt-0.5">•</span>
                      <span>{warning}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setShowPreview(false)}
              className="neon-border"
            >
              Cancel
            </Button>
            <Button
              onClick={handleConfirmApply}
              className="bg-primary hover:bg-primary/80 text-primary-foreground neon-border"
            >
              <Play className="h-4 w-4 mr-2" />
              Confirm & Apply
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
            /* ── Success State ── */
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
            /* ── Form State ── */
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
                  A new branch <code className="text-primary">aegios/fix-*</code> will be created from <code className="text-primary">{selectedBranch || '...'}</code>, the fixed YAML will be committed, and a PR will be opened.
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
    </div>
  );
};

export default K8sActionsPage;