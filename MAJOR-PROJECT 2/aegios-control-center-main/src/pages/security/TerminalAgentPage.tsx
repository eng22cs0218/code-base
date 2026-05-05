import { useState, useRef, useEffect } from 'react';
import { AgentConfigUpload } from '@/components/security/AgentConfigUpload';
import { AgentTerminal, AgentTerminalRef } from '@/components/security/AgentTerminal';
import { Terminal, ShieldCheck, ArrowLeft, Play, AlertTriangle, Zap } from 'lucide-react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { API_CONFIG } from '@/config/api';
import { toast } from 'sonner';

const TerminalAgentPage = () => {
  const [token, setToken] = useState<string | null>(null);
  const [pendingCommand, setPendingCommand] = useState<string | null>(null);
  const [pendingFindingId, setPendingFindingId] = useState<string | null>(null);
  const [isQueuingAction, setIsQueuingAction] = useState(false);
  const [actionQueued, setActionQueued] = useState(false);
  const terminalRef = useRef<AgentTerminalRef>(null);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  // Check if we arrived with a remediation command or finding_id via URL params
  useEffect(() => {
    const cmd = searchParams.get('command');
    const t = searchParams.get('token');
    const findingId = searchParams.get('finding_id');
    const rUrl = searchParams.get('returnUrl');

    if (cmd) setPendingCommand(decodeURIComponent(cmd));
    if (findingId) setPendingFindingId(findingId);
    if (t) {
      setToken(t);
      localStorage.setItem('aegios_terminal_token', t);
    } else {
      // Check if we already have a token stored
      const existingToken = localStorage.getItem('aegios_terminal_token');
      if (existingToken) {
        setToken(existingToken);
      }
    }
  }, [searchParams]);

  // When token becomes available and we have a pending finding_id, trigger the take-action
  useEffect(() => {
    if (token && pendingFindingId && !actionQueued) {
      triggerTakeAction(token, pendingFindingId);
    }
  }, [token, pendingFindingId, actionQueued]);

  const handleConfigSuccess = async (newToken: string) => {
    setToken(newToken);
    localStorage.setItem('aegios_terminal_token', newToken);

    // If there's a pending remediation command, queue it via take-action
    if (pendingCommand) {
      await queueRemediationCommand(newToken, pendingCommand);
    }
  };

  // Trigger take-action with finding_id — this tells the backend to look up the
  // remediation command from agent_output and execute it via the terminal
  const triggerTakeAction = async (sessionToken: string, findingId: string) => {
    setIsQueuingAction(true);
    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SESSION.TAKE_ACTION, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          token: sessionToken,
          finding_id: findingId,
        }),
      });

      const result = await response.json();

      if (response.ok && result.success) {
        toast.success('Remediation command sent to terminal!');
        setActionQueued(true);
      } else {
        toast.error(result.error || 'Failed to execute remediation');
      }
    } catch (err) {
      console.error('Error triggering take-action:', err);
      toast.error('Failed to send remediation command');
    } finally {
      setIsQueuingAction(false);
    }
  };

  const queueRemediationCommand = async (sessionToken: string, command: string) => {
    setIsQueuingAction(true);
    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SESSION.TAKE_ACTION, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          token: sessionToken,
          command: command,
          correct_config: '',
        }),
      });

      if (!response.ok) {
        console.error('Failed to queue remediation command');
      }
    } catch (err) {
      console.error('Error queuing action:', err);
    } finally {
      setIsQueuingAction(false);
    }
  };

  const handleDisconnect = () => {
    // Optionally reset so user can reconnect
  };

  return (
    <div className="min-h-[calc(100vh-8rem)] space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary/10 border border-primary/30">
              <Terminal className="h-6 w-6 text-primary" />
            </div>
            <h1 className="text-3xl font-bold text-primary">Terminal Agent</h1>
          </div>
          <p className="text-muted-foreground ml-[52px]">
            Connect to your Kubernetes cluster and remediate vulnerabilities directly from Aegios.
          </p>
        </div>
        <button
          onClick={() => {
            const returnUrl = searchParams.get('returnUrl');
            if (returnUrl) {
              navigate(decodeURIComponent(returnUrl));
            } else {
              navigate('/security-service/k8s-action');
            }
          }}
          className="flex items-center gap-2 text-sm text-muted-foreground hover:text-primary transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Actions
        </button>
      </div>

      {/* Pending Remediation Banner — shows when we have a finding_id but no token yet */}
      {pendingFindingId && !token && (
        <div className="p-4 rounded-xl bg-yellow-500/10 border border-yellow-500/30 flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-yellow-500 mt-0.5 shrink-0" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-yellow-400">Remediation Action Pending</p>
            <p className="text-xs text-muted-foreground">
              Connect your cluster below — the remediation fix for <code className="text-yellow-300 bg-yellow-500/10 px-1 rounded">Finding #{pendingFindingId}</code> will auto-execute once connected.
            </p>
          </div>
        </div>
      )}

      {/* Pending Command Banner — shows when we have a raw command but no token yet */}
      {pendingCommand && !token && !pendingFindingId && (
        <div className="p-4 rounded-xl bg-yellow-500/10 border border-yellow-500/30 flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-yellow-500 mt-0.5 shrink-0" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-yellow-400">Remediation Command Queued</p>
            <p className="text-xs text-muted-foreground">
              Connect your cluster below — the following command will auto-execute once connected:
            </p>
            <code className="block mt-2 text-xs font-mono text-yellow-300 bg-yellow-500/10 border border-yellow-500/20 rounded-lg p-3 break-all">
              {pendingCommand}
            </code>
          </div>
        </div>
      )}

      {/* Action Queued Success Banner */}
      {actionQueued && token && (
        <div className="p-4 rounded-xl bg-green-500/10 border border-green-500/30 flex items-start gap-3">
          <Zap className="h-5 w-5 text-green-400 mt-0.5 shrink-0" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-green-400">Remediation Sent to Terminal</p>
            <p className="text-xs text-muted-foreground">
              The fix command has been sent to the terminal below. Check the output for results.
            </p>
          </div>
        </div>
      )}

      {/* Stepper Indicator */}
      <div className="flex items-center gap-4">
        <div className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${!token ? 'bg-primary/10 border border-primary/40 text-primary' : 'bg-green-500/10 border border-green-500/40 text-green-400'}`}>
          <ShieldCheck className="h-4 w-4" />
          <span className="text-sm font-medium">Step 1: Connect Cluster</span>
          {token && <span className="text-green-400 text-xs">✓</span>}
        </div>
        <div className="h-px w-8 bg-border" />
        <div className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${token ? 'bg-primary/10 border border-primary/40 text-primary' : 'bg-secondary/30 border border-border text-muted-foreground'}`}>
          <Terminal className="h-4 w-4" />
          <span className="text-sm font-medium">Step 2: Use Terminal</span>
        </div>
      </div>

      {/* Config Upload or Terminal */}
      {!token ? (
        <div className="p-6 rounded-xl bg-card border border-border neon-border">
          <h2 className="text-lg font-semibold text-foreground mb-4">Connect Your Cluster</h2>
          <p className="text-sm text-muted-foreground mb-6">
            Enter your Kubernetes context name, generate the connection command, run it in your terminal,
            then upload the generated config file.
          </p>
          <AgentConfigUpload onSuccess={handleConfigSuccess} />
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <p className="text-sm text-green-400 flex items-center gap-2">
              <ShieldCheck className="h-4 w-4" />
              {isQueuingAction
                ? 'Sending remediation command to terminal...'
                : actionQueued
                  ? 'Cluster connected — Remediation command sent. Check output below.'
                  : pendingFindingId
                    ? 'Cluster connected — Preparing remediation...'
                    : 'Cluster connected — You can now run kubectl commands below.'}
            </p>
            <button
              onClick={() => {
                setToken(null);
                localStorage.removeItem('aegios_terminal_token');
                setActionQueued(false);
                setPendingFindingId(null);
                setPendingCommand(null);
              }}
              className="text-xs text-muted-foreground hover:text-destructive transition-colors border border-border hover:border-destructive/50 px-3 py-1.5 rounded-lg"
            >
              Disconnect
            </button>
          </div>
          <AgentTerminal ref={terminalRef} token={token} onDisconnect={handleDisconnect} />
          
          {/* Manual command send */}
          <div className="flex items-center gap-2">
            <button
              onClick={() => {
                const cmd = prompt('Enter kubectl command to execute:');
                if (cmd && terminalRef.current) {
                  terminalRef.current.sendCommand(cmd);
                }
              }}
              className="flex items-center gap-2 text-xs text-muted-foreground hover:text-primary transition-colors border border-border hover:border-primary/50 px-3 py-1.5 rounded-lg"
            >
              <Play className="h-3 w-3" />
              Send Command
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

export default TerminalAgentPage;
