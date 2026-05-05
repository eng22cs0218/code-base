import React, { useEffect, useRef, useState, useImperativeHandle, forwardRef } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { API_CONFIG } from '@/config/api';

interface AgentTerminalProps {
  token: string | null;
  onDisconnect?: () => void;
}

export interface AgentTerminalRef {
  sendCommand: (cmd: string) => void;
}

export const AgentTerminal = forwardRef<AgentTerminalRef, AgentTerminalProps>(({ token, onDisconnect }, ref) => {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<'Connecting' | 'Connected' | 'Disconnected'>('Connecting');

  useImperativeHandle(ref, () => ({
    sendCommand: (cmd: string) => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.send(cmd + '\r');
      } else {
        xtermRef.current?.write('\r\n[Error] Not connected to Agent.\r\n');
      }
    }
  }));

  useEffect(() => {
    let isMounted = true;
    if (!token || !terminalRef.current) return;

    // Initialize xterm
    const term = new Terminal({
      cursorBlink: true,
      theme: {
        background: '#1e1e1e',
        foreground: '#f8f8f2',
      },
      fontFamily: 'monospace',
      fontSize: 14,
      scrollback: 10000,
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(terminalRef.current);
    
    // Nudge fit after a short delay to ensure container is ready
    setTimeout(() => fitAddon.fit(), 0);
    
    xtermRef.current = term;

    // Connect WebSocket
    const wsUrl = `${API_CONFIG.ENDPOINTS.SESSION.WS}?token=${token}`;
    const ws = new WebSocket(wsUrl);
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;

    ws.onopen = () => {
      if (!isMounted) return;
      setStatus('Connected');
      term.writeln('Connected to server...');
    };

    ws.onmessage = (event) => {
      if (typeof event.data === 'string') {
        term.write(event.data);
        term.scrollToBottom();
      } else if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data));
        term.scrollToBottom();
      }
    };

    ws.onclose = () => {
      if (!isMounted) return;
      setStatus('Disconnected');
      term.writeln('\r\nDisconnected from server.');
      if (onDisconnect) onDisconnect();
    };

    ws.onerror = (e) => {
      if (!isMounted) return;
      console.error('WebSocket Error', e);
      term.writeln('\r\nWebSocket connection error.');
    };

    // Handle copy/paste natively
    term.attachCustomKeyEventHandler((e) => {
      if (e.type === 'keydown') {
        if (e.ctrlKey && e.code === 'KeyC' && term.hasSelection()) {
          navigator.clipboard.writeText(term.getSelection());
          return false;
        }
        if (e.ctrlKey && e.code === 'KeyV') {
          navigator.clipboard.readText().then(text => {
            if (ws.readyState === WebSocket.OPEN) {
              ws.send(text);
            }
          });
          return false;
        }
      }
      return true;
    });

    // Handle user input
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data);
      }
    });

    const handleResize = () => fitAddon.fit();
    window.addEventListener('resize', handleResize);

    return () => {
      isMounted = false;
      window.removeEventListener('resize', handleResize);
      ws.onclose = null; // Prevent triggering close events after unmount
      ws.close();
      term.dispose();
    };
  }, [token, onDisconnect]);

  if (!token) return null;

  return (
    <div className="flex flex-col w-full h-[400px] bg-[#1e1e1e] rounded-md overflow-hidden border border-slate-700 shadow-xl">
      <div className="flex justify-between items-center px-4 py-2 bg-slate-800 border-b border-slate-700">
        <span className="text-white text-sm font-semibold flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${status === 'Connected' ? 'bg-green-500' : status === 'Connecting' ? 'bg-yellow-500' : 'bg-red-500'}`} />
          Terminal
        </span>
        <span className="text-slate-400 text-xs">{status}</span>
      </div>
      <div ref={terminalRef} className="flex-1 p-2 h-full" />
    </div>
  );
});

AgentTerminal.displayName = 'AgentTerminal';
