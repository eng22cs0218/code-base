import React, { useState, useEffect, useRef } from 'react';
import { Copy, Check, RefreshCw, Loader2, Zap, Terminal, CheckCircle2, Upload } from 'lucide-react';
import { API_CONFIG } from '@/config/api';
import { getSessionToken } from '@/lib/data-transformers';

interface AgentConfigUploadProps {
  onSuccess: (token: string) => void;
}

type FlowStatus = 'idle' | 'waiting' | 'active';

export const AgentConfigUpload: React.FC<AgentConfigUploadProps> = ({ onSuccess }) => {
  // ── State ──────────────────────────────────────────────────────────────────
  const [contextName, setContextName] = useState('');
  const [curlCommand, setCurlCommand] = useState('');
  const [orgId, setOrgId] = useState('');
  const [status, setStatus] = useState<FlowStatus>('idle');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // For generating a session token for the terminal after activation
  const [generatedToken, setGeneratedToken] = useState('');

  const pollIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // ── Handle File Upload Directly ────────────────────────────────────────────
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!contextName.trim()) {
      setError('Please enter a Kubernetes context name before uploading.');
      if (fileInputRef.current) fileInputRef.current.value = '';
      return;
    }

    setIsLoading(true);
    setError(null);
    setCurlCommand(''); // Clear if previously generated

    try {
      // 1. Get Phase 2 Token
      const initRes = await fetch(API_CONFIG.ENDPOINTS.SESSION.INIT, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ context_name: contextName.trim(), session_token: getSessionToken() || '' }),
      });
      const initData = await initRes.json();
      if (!initRes.ok) throw new Error(initData.error || 'Init failed');

      const p2Token = initData.token;
      if (!p2Token) throw new Error('Token not received from backend');

      // 2. Upload File with Phase 2 Token
      const formData = new FormData();
      formData.append('config', file);

      const uploadRes = await fetch(API_CONFIG.ENDPOINTS.SESSION.UPLOAD_CONFIG_V2, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${p2Token}`
        },
        body: formData,
      });

      const uploadData = await uploadRes.json();
      if (!uploadRes.ok) throw new Error(uploadData.error || 'Upload failed');

      // 3. Success! Set to active and trigger connection
      if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
      setStatus('active');
      localStorage.setItem('aegios_terminal_token', uploadData.session_token);
      setTimeout(() => onSuccess(uploadData.session_token), 1000);

    } catch (err: any) {
      setError(err.message || 'File upload failed');
    } finally {
      setIsLoading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  // ── Persist connection state on mount ──────────────────────────────────────
  useEffect(() => {
    const existingToken = localStorage.getItem('aegios_terminal_token');
    if (existingToken && status === 'idle') {
      fetch(`${API_CONFIG.ENDPOINTS.SESSION.CONFIG_STATUS}?token=${existingToken}`)
        .then(res => res.json())
        .then(data => {
          if (data.success && data.has_config) {
            setStatus('active');
            onSuccess(existingToken);
          } else {
            localStorage.removeItem('aegios_terminal_token');
          }
        })
        .catch(err => console.error("Failed to check agent status:", err));
    }
  }, []);

  // ── Polling: watch for status='active' ─────────────────────────────────────
  useEffect(() => {
    if (status !== 'waiting' || !orgId || !contextName) return;

    pollIntervalRef.current = setInterval(async () => {
      try {
        const response = await fetch(
          `${API_CONFIG.ENDPOINTS.SESSION.STATUS}?context_name=${encodeURIComponent(contextName)}&org_id=${encodeURIComponent(orgId)}`
        );
        const data = await response.json();

        if (data.status === 'active' && data.session_token) {
          // Config received! Transition to success state
          if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
          setStatus('active');
          
          // Store terminal token in localStorage for remediation actions (Phase 3)
          localStorage.setItem('aegios_terminal_token', data.session_token);
          
          // Connect to the terminal using the DB session token retrieved from the polling query!
          setTimeout(() => onSuccess(data.session_token), 1000);
        }
      } catch {
        // Silently retry on polling failure
      }
    }, 3000);

    return () => {
      if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
    };
  }, [status, orgId, contextName, onSuccess]);

  // ── Generate Link Handler ──────────────────────────────────────────────────
  const handleGenerate = async () => {
    if (!contextName.trim()) {
      setError('Please enter a Kubernetes context name.');
      return;
    }

    setIsLoading(true);
    setError(null);
    setCurlCommand('');

    try {
      const response = await fetch(API_CONFIG.ENDPOINTS.SESSION.GENERATE_COMMAND, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ context_name: contextName.trim(), org_id: "1" }),
      });

      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || 'Failed to initialize session');
      }

      setCurlCommand(data.command || data.curl_command);
      setOrgId(data.org_id || "1");
      setGeneratedToken(data.token);
      setStatus('waiting');
    } catch (err: any) {
      setError(err.message || 'An error occurred');
    } finally {
      setIsLoading(false);
    }
  };

  // ── Copy Command ───────────────────────────────────────────────────────────
  const handleCopy = () => {
    if (!curlCommand) return;
    navigator.clipboard.writeText(curlCommand);
    setCopied(true);
    setTimeout(() => {
      setCopied(false);
      // Immediately transition to Phase 1 Terminal!
      if (generatedToken) {
        localStorage.setItem('aegios_terminal_token', generatedToken);
        onSuccess(generatedToken);
      }
    }, 1500);
  };

  // ── Cancel/Reset ───────────────────────────────────────────────────────────
  const handleReset = () => {
    if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
    setContextName('');
    setCurlCommand('');
    setOrgId('');
    setStatus('idle');
    setError(null);
    setCopied(false);
    setGeneratedToken('');
  };

  // ── Render ─────────────────────────────────────────────────────────────────
  return (
    <div className="h-full flex flex-col justify-center gap-5">

      {/* ── Step 1: Context Input + Generate Link ──────────────────────────── */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="col-span-1 flex flex-col gap-2">
          <div className="h-[80px] bg-[#29A35C]/10 border border-[#29A35C]/50 rounded-xl flex items-center px-4 transition-colors focus-within:border-[#29A35C] focus-within:bg-[#29A35C]/20">
            <input
              type="text"
              value={contextName}
              onChange={(e) => {
                setContextName(e.target.value);
                if (status !== 'idle') handleReset();
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && contextName.trim() && status === 'idle') {
                  handleGenerate();
                }
              }}
              placeholder="enter cluster context"
              disabled={status === 'waiting' || status === 'active'}
              className="w-full bg-[#0a0a0a] text-center text-green-muted placeholder:text-green-muted/50 font-medium focus:outline-none disabled:opacity-50"
              id="context-name-input"
            />
          </div>
          <div className="flex gap-2 w-full">
            <button
              onClick={handleGenerate}
              disabled={isLoading || !contextName.trim() || status === 'waiting' || status === 'active'}
              className="flex-1 py-2 bg-green-600 hover:bg-green-500 disabled:opacity-50 text-black font-semibold rounded-lg flex justify-center items-center gap-2 transition-colors"
              id="generate-link-btn"
            >
              {isLoading ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : (
                <>
                  <Terminal className="w-4 h-4" />
                  Link
                </>
              )}
            </button>

            <input 
              type="file" 
              ref={fileInputRef} 
              hidden 
              onChange={handleFileUpload} 
              accept=".yaml,.yml,.txt,text/yaml"
            />
            <button
              onClick={() => fileInputRef.current?.click()}
              disabled={isLoading || !contextName.trim() || status === 'active'}
              className="flex-1 py-2 bg-green-600 hover:bg-green-500 disabled:opacity-50 text-black font-semibold rounded-lg flex justify-center items-center gap-2 transition-colors"
            >
              {isLoading ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : (
                <>
                  <Upload className="w-4 h-4" />
                  Upload
                </>
              )}
            </button>
          </div>
        </div>

        {/* ── Curl Command Display ──────────────────────────────────────── */}
        <div className="col-span-2">
          <div className="min-h-[80px] bg-red-500/10 border border-red-500/50 rounded-xl p-4 flex items-center justify-center relative hover:bg-red-500/20 transition-colors group">
            <code className="text-red-400 font-mono text-sm text-center break-all px-6 select-all" id="curl-command-display">
              {curlCommand
                ? curlCommand
                : 'Fill in the context and generate link first'}
            </code>
            {curlCommand && (
              <button
                onClick={handleCopy}
                className="absolute right-4 top-1/2 -translate-y-1/2 text-red-500/50 hover:text-red-400 transition-colors opacity-0 group-hover:opacity-100"
                title="Copy command"
                id="copy-curl-btn"
              >
                {copied ? (
                  <Check className="w-5 h-5 text-green-500" />
                ) : (
                  <Copy className="w-5 h-5" />
                )}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* ── Step 2: Waiting State — polling for agent connection ────────── */}
      {status === 'waiting' && (
        <div className="h-[80px] bg-[#29A35C]/10 border-2 border-dashed border-[#29A35C]/50 rounded-xl flex items-center justify-center gap-3">
          <Upload className="w-5 h-5 text-green-400 animate-bounce" />
          <span className="font-semibold text-green-muted text-sm">
            Run the command above, then click the Upload button to provide the saved config file.
          </span>
        </div>
      )}

      {/* ── Step 3: Success State — agent connected ────────────────────── */}
      {status === 'active' && (
        <div className="h-[80px] bg-green-500/10 border-2 border-green-500/50 rounded-xl flex items-center justify-center gap-3">
          <CheckCircle2 className="w-6 h-6 text-green-400" />
          <span className="font-semibold text-green-400 text-sm">
            ✅ Agent Connected Successfully
          </span>
        </div>
      )}

      {/* ── Error Display ──────────────────────────────────────────────── */}
      {error && (
        <div className="p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-red-500 text-sm text-center">
          {error}
        </div>
      )}
    </div>
  );
};
