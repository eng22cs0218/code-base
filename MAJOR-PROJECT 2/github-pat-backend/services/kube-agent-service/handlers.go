package kubeagentservice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github-pat-backend/pkg/database"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const ConfigDir = "/tmp/aegios-kubeconfigs"

var sessionMu sync.Mutex

func init() {
	os.MkdirAll(ConfigDir, 0700)
}

// agentScript is the bash script served at GET /session/agent-script
const agentScript = `#!/bin/bash
# Aegios Kube Agent Connector
# Canonical source: https://github.com/Aegios-k8s/kube-connect-script
set -e

CONTEXT_NAME="${1}"
TOKEN="${2}"
BACKEND_URL="${3:-@@BACKEND_URL@@}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

echo ""
echo -e "${CYAN}${BOLD}"
echo "  ╔═══════════════════════════════════════════════╗"
echo "  ║         Aegios Kube Agent Connector           ║"
echo "  ║       Secure Kubernetes Remediation           ║"
echo "  ╚═══════════════════════════════════════════════╝"
echo -e "${NC}"

if [ -z "$CONTEXT_NAME" ] || [ -z "$TOKEN" ]; then
    echo -e "${RED}✗ Error: Missing required arguments.${NC}"
    echo "  Usage: bash <script> <context_name> <token>"
    exit 1
fi

echo -e "${YELLOW}[1/6]${NC} Checking prerequisites..."
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}  ✗ kubectl is not installed.${NC}"
    exit 1
fi
echo -e "${GREEN}  ✓ kubectl found${NC}"

if ! command -v python3 &> /dev/null; then
    echo -e "${RED}  ✗ python3 is not installed (required for WebSocket agent).${NC}"
    exit 1
fi
echo -e "${GREEN}  ✓ python3 found${NC}"

echo -e "${YELLOW}[2/6]${NC} Validating context '${BOLD}${CONTEXT_NAME}${NC}'..."
if ! kubectl config get-contexts "$CONTEXT_NAME" &> /dev/null 2>&1; then
    echo -e "${RED}  ✗ Context '${CONTEXT_NAME}' not found.${NC}"
    echo "  Available contexts:"
    kubectl config get-contexts -o name 2>/dev/null | while read -r ctx; do echo "    - $ctx"; done
    exit 1
fi
echo -e "${GREEN}  ✓ Context '${CONTEXT_NAME}' exists${NC}"

if kubectl --context="$CONTEXT_NAME" cluster-info &> /dev/null 2>&1; then
    echo -e "${GREEN}  ✓ Cluster is reachable${NC}"
else
    echo -e "${YELLOW}  ⚠ Cluster may not be reachable (continuing anyway)${NC}"
fi

echo -e "${YELLOW}[3/6]${NC} Extracting kubeconfig..."
TEMP_CONFIG=$(mktemp /tmp/aegios-kubeconfig.XXXXXX)
trap "rm -f $TEMP_CONFIG $AGENT_SCRIPT" EXIT
kubectl config view --minify --context="$CONTEXT_NAME" --flatten > "$TEMP_CONFIG" 2>/dev/null
if [ ! -s "$TEMP_CONFIG" ]; then
    echo -e "${RED}  ✗ Failed to extract kubeconfig.${NC}"
    exit 1
fi
echo -e "${GREEN}  ✓ Kubeconfig extracted${NC}"
export KUBECONFIG="$TEMP_CONFIG"

echo -e "${YELLOW}[4/6]${NC} Uploading config to Aegios..."
UPLOAD_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${BACKEND_URL}/session/upload" -F "token=${TOKEN}" -F "config=@${TEMP_CONFIG}" --connect-timeout 10 --max-time 30)
HTTP_CODE=$(echo "$UPLOAD_RESPONSE" | tail -1)
if [ "$HTTP_CODE" -ne 200 ]; then
    echo -e "${RED}  ✗ Upload failed (HTTP ${HTTP_CODE}).${NC}"
    exit 1
fi
echo -e "${GREEN}  ✓ Config uploaded successfully${NC}"

echo -e "${YELLOW}[5/6]${NC} Downloading WebSocket agent..."
AGENT_SCRIPT=$(mktemp /tmp/aegios-agent.XXXXXX)
AGENT_URL="${BACKEND_URL}/session/agent-script-py"
if ! curl -fsSL "$AGENT_URL" -o "$AGENT_SCRIPT" --connect-timeout 10 --max-time 30; then
    echo -e "${YELLOW}  ⚠ Backend agent download failed, trying GitHub...${NC}"
    AGENT_URL="https://raw.githubusercontent.com/Aegios-k8s/kube-connect-script/main/agent.py"
    if ! curl -fsSL "$AGENT_URL" -o "$AGENT_SCRIPT" --connect-timeout 10 --max-time 30; then
        echo -e "${RED}  ✗ Failed to download agent.${NC}"
        exit 1
    fi
fi
echo -e "${GREEN}  ✓ Agent downloaded${NC}"

echo -e "${YELLOW}[6/6]${NC} Starting WebSocket agent..."
echo ""
echo -e "${GREEN}${BOLD}  ✓ Agent Starting!${NC}"
echo -e "  ${CYAN}Cluster:${NC} ${BOLD}${CONTEXT_NAME}${NC}"
echo -e "  ${CYAN}Backend:${NC} ${BOLD}${BACKEND_URL}${NC}"
echo -e "  ${YELLOW}   Commands from Aegios dashboard will execute here.${NC}"
echo -e "  ${YELLOW}   Press Ctrl+C to stop.${NC}"
echo ""

WS_URL=$(echo "$BACKEND_URL" | sed 's|^http://|ws://|;s|^https://|wss://|')
python3 "$AGENT_SCRIPT" "${WS_URL}/session/agent-ws" "$TOKEN"
`

// agentScriptV2 is the bash script specifically for Phase 2 flow (Bearer auth + memory tokens)
const agentScriptV2 = `#!/bin/bash
# Aegios Kube Agent Connector (Phase 2 V2)
set -e

CONTEXT_NAME="${1}"
TOKEN="${2}"
BACKEND_URL="${3:-@@BACKEND_URL@@}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

echo ""
echo -e "${CYAN}${BOLD}"
echo "  ╔═══════════════════════════════════════════════╗"
echo "  ║         Aegios Kube Agent Connector           ║"
echo "  ║       Secure Kubernetes Remediation           ║"
echo "  ╚═══════════════════════════════════════════════╝"
echo -e "${NC}"

if [ -z "$CONTEXT_NAME" ] || [ -z "$TOKEN" ]; then
    echo -e "${RED}✗ Error: Missing required arguments.${NC}"
    echo "  Usage: bash <script> <context_name> <token>"
    exit 1
fi

echo -e "${YELLOW}[1/6]${NC} Checking prerequisites..."
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}  ✗ kubectl is not installed.${NC}"
    exit 1
fi
echo -e "${GREEN}  ✓ kubectl found${NC}"

if ! command -v python3 &> /dev/null; then
    echo -e "${RED}  ✗ python3 is not installed (required for WebSocket agent).${NC}"
    exit 1
fi
echo -e "${GREEN}  ✓ python3 found${NC}"

echo -e "${YELLOW}[2/6]${NC} Validating context '${BOLD}${CONTEXT_NAME}${NC}'..."
if ! kubectl config get-contexts "$CONTEXT_NAME" &> /dev/null 2>&1; then
    echo -e "${RED}  ✗ Context '${CONTEXT_NAME}' not found.${NC}"
    echo "  Available contexts:"
    kubectl config get-contexts -o name 2>/dev/null | while read -r ctx; do echo "    - $ctx"; done
    exit 1
fi
echo -e "${GREEN}  ✓ Context '${CONTEXT_NAME}' exists${NC}"

if kubectl --context="$CONTEXT_NAME" cluster-info &> /dev/null 2>&1; then
    echo -e "${GREEN}  ✓ Cluster is reachable${NC}"
else
    echo -e "${YELLOW}  ⚠ Cluster may not be reachable (continuing anyway)${NC}"
fi

echo -e "${YELLOW}[3/3]${NC} Extracting kubeconfig..."
CONFIG_DIR="$HOME/.kube"
CONFIG_FILE="$CONFIG_DIR/aegios-${CONTEXT_NAME}-config.yaml"

mkdir -p "$CONFIG_DIR"
kubectl config view --minify --context="$CONTEXT_NAME" --flatten > "$CONFIG_FILE" 2>/dev/null
if [ ! -s "$CONFIG_FILE" ]; then
    echo -e "${RED}  ✗ Failed to extract kubeconfig.${NC}"
    exit 1
fi
echo -e "${GREEN}  ✓ Kubeconfig extracted securely${NC}"

echo ""
echo -e "${GREEN}${BOLD}  ✓ Success! Action Required:${NC}"
echo -e "  Your configuration has been saved locally at:"
echo -e "  ${CYAN}${BOLD}${CONFIG_FILE}${NC}"
echo ""
echo -e "  ${YELLOW}Please go back to the Aegios dashboard and manually${NC}"
echo -e "  ${YELLOW}upload this file to connect your terminal.${NC}"
echo ""
exit 0
`

// agentPythonScript is embedded so the backend can serve it even without GitHub access
//
//go:generate echo "agent.py is embedded as a string"
const agentPythonScript = `#!/usr/bin/env python3
"""Aegios Kube Agent - WebSocket agent (zero external dependencies)."""
import socket, struct, hashlib, base64, os, json, subprocess, sys, ssl, time

class WebSocketClient:
    def __init__(self, url):
        self.url = url
        self.sock = None
        self._parse_url(url)

    def _parse_url(self, url):
        if url.startswith("wss://"):
            self.use_ssl = True
            url = url[6:]
        elif url.startswith("ws://"):
            self.use_ssl = False
            url = url[5:]
        else:
            raise ValueError(f"Invalid URL: {url}")
        if "/" in url:
            host_port, self.path = url.split("/", 1)
            self.path = "/" + self.path
        else:
            host_port, self.path = url, "/"
        if ":" in host_port:
            self.host, port_str = host_port.split(":")
            self.port = int(port_str)
        else:
            self.host = host_port
            self.port = 443 if self.use_ssl else 80

    def connect(self):
        raw = socket.create_connection((self.host, self.port), timeout=30)
        if self.use_ssl:
            ctx = ssl.create_default_context()
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE
            self.sock = ctx.wrap_socket(raw, server_hostname=self.host)
        else:
            self.sock = raw
        key = base64.b64encode(os.urandom(16)).decode()
        hs = (f"GET {self.path} HTTP/1.1\r\nHost: {self.host}:{self.port}\r\n"
              f"Upgrade: websocket\r\nConnection: Upgrade\r\n"
              f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n")
        self.sock.send(hs.encode())
        resp = b""
        while b"\r\n\r\n" not in resp:
            c = self.sock.recv(1)
            if not c: raise ConnectionError("Closed during handshake")
            resp += c
        if b"101" not in resp:
            raise ConnectionError(f"Handshake failed: {resp.decode()}")
        self.sock.settimeout(None)  # Switch to blocking mode after handshake

    def send(self, data):
        payload = data.encode("utf-8") if isinstance(data, str) else data
        hdr = bytearray([0x81])
        mask = os.urandom(4)
        ln = len(payload)
        if ln < 126: hdr.append(0x80 | ln)
        elif ln < 65536: hdr.append(0x80 | 126); hdr.extend(struct.pack(">H", ln))
        else: hdr.append(0x80 | 127); hdr.extend(struct.pack(">Q", ln))
        hdr.extend(mask)
        masked = bytearray(b ^ mask[i % 4] for i, b in enumerate(payload))
        self.sock.sendall(hdr + masked)

    def recv(self):
        data = self._rx(2)
        if not data or len(data) < 2: return None
        opcode = data[0] & 0x0F
        m = data[1] & 0x80
        ln = data[1] & 0x7F
        if ln == 126: ln = struct.unpack(">H", self._rx(2))[0]
        elif ln == 127: ln = struct.unpack(">Q", self._rx(8))[0]
        mk = self._rx(4) if m else None
        p = self._rx(ln) if ln > 0 else b""
        if mk: p = bytearray(b ^ mk[i % 4] for i, b in enumerate(p))
        if opcode == 0x8: return None
        if opcode == 0x9:
            pong = bytearray([0x8A, 0x80]); pong.extend(os.urandom(4))
            self.sock.sendall(pong); return self.recv()
        if opcode == 0xA: return self.recv()
        return p.decode("utf-8", errors="replace") if opcode == 0x1 else p

    def _rx(self, n):
        buf = b""
        while len(buf) < n:
            c = self.sock.recv(n - len(buf))
            if not c: return None
            buf += c
        return buf

    def close(self):
        try:
            self.sock.sendall(bytearray([0x88, 0x80]) + os.urandom(4))
            self.sock.shutdown(socket.SHUT_RDWR)
        except: pass
        try: self.sock.close()
        except: pass

G="\033[0;32m"; R="\033[0;31m"; Y="\033[1;33m"; C="\033[0;36m"; B="\033[1m"; N="\033[0m"

def execute(cmd):
    try:
        r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=120)
        out = r.stdout + r.stderr
        return out.strip(), r.returncode
    except subprocess.TimeoutExpired: return "Error: timed out (120s)", 1
    except Exception as e: return f"Error: {e}", 1

def execute_streaming(ws, command_id, cmd):
    """Execute a command and stream output line-by-line back via WebSocket."""
    try:
        import subprocess as sp
        process = sp.Popen(cmd, shell=True, stdout=sp.PIPE, stderr=sp.STDOUT, text=True)
        for line in process.stdout:
            stripped = line.rstrip()
            if stripped:
                print(stripped)
                ws.send(json.dumps({"type":"output","command_id":command_id,"output":stripped}))
        process.wait()
        ec = process.returncode
        ws.send(json.dumps({"type":"done","command_id":command_id,"output":"","exit_code":ec}))
        if ec == 0:
            print(f"{G}✓ Remediation completed (exit {ec}){N}\n")
        else:
            print(f"{R}✗ Remediation failed (exit {ec}){N}\n")
    except Exception as e:
        print(f"{R}✗ Remediation error: {e}{N}\n")
        ws.send(json.dumps({"type":"error","command_id":command_id,"output":str(e),"exit_code":1}))

def run_agent(ws_url, token):
    url = f"{ws_url}?token={token}"
    print(f"{C}  Connecting to Aegios backend...{N}")
    ws = WebSocketClient(url)
    try: ws.connect()
    except Exception as e:
        print(f"{R}  ✗ Failed: {e}{N}"); return False
    print(f"{G}  ✓ WebSocket connected!{N}")
    print(f"{Y}  Listening for commands... (Ctrl+C to stop){N}\n")
    ws.send(json.dumps({"type":"agent_ready","message":"Agent online"}))
    try:
        while True:
            msg = ws.recv()
            if msg is None: print(f"{Y}  Connection closed.{N}"); break
            try: data = json.loads(msg)
            except: data = {"type":"command","command":msg}
            if data.get("type") == "ping":
                ws.send(json.dumps({"type":"pong"})); continue
            if data.get("type") == "execute":
                # Phase 3: streaming remediation execution
                command_id = data.get("command_id", 0)
                cmd = data.get("command", "").strip()
                manifest = data.get("manifest", "")
                
                temp_file = None
                if manifest and "apply -f" in cmd:
                    import re
                    m = re.search(r'-f\s+([^\s]+)', cmd)
                    if m:
                        filename = m.group(1)
                        # We don't want to overwrite important system files if names collide, but in CWD it's fine
                        with open(filename, 'w') as f:
                            f.write(manifest)
                        temp_file = filename

                if cmd:
                    print(f"{C}▶ Remediation [{command_id}]:{N} {cmd}")
                    execute_streaming(ws, command_id, cmd)
                
                if temp_file:
                    import os
                    try: os.unlink(temp_file)
                    except: pass
                continue
            cmd = data.get("command","").strip()
            if not cmd: continue
            print(f"{C}▶ Executing:{N} {cmd}")
            out, ec = execute(cmd)
            print(out)
            print(f"{G}✓ Done (exit {ec}){N}\n" if ec == 0 else f"{R}✗ Failed (exit {ec}){N}\n")
            ws.send(json.dumps({"type":"result","output":out,"exit_code":ec,"command":cmd}))
    except KeyboardInterrupt: print(f"\n{Y}  Stopped.{N}")
    except Exception as e: print(f"{R}  ✗ Error: {e}{N}"); return False
    finally: ws.close()
    return True

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(f"Usage: python3 {sys.argv[0]} <ws_url> <token>"); sys.exit(1)
    ws_url, token = sys.argv[1], sys.argv[2]
    print(f"{C}{B}\n  Aegios Kube Agent (WebSocket)\n{N}")
    retries = 0
    while retries < 5:
        if run_agent(ws_url, token): break
        retries += 1
        w = min(2**retries, 30)
        print(f"{Y}  Reconnecting in {w}s... ({retries}/5){N}")
        time.sleep(w)
`

type GenerateCommandRequest struct {
	OrgID       string `json:"org_id"`
	ContextName string `json:"context_name"`
}

func GenerateCommandHandler(c *gin.Context) {
	var req GenerateCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.ContextName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_name is required"})
		return
	}

	if req.OrgID == "" || req.OrgID == "1" {
		var existingOrgID string
		err := database.DB.QueryRow(`SELECT org_id FROM organization LIMIT 1`).Scan(&existingOrgID)
		if err == nil && existingOrgID != "" {
			req.OrgID = existingOrgID
		} else {
			fallbackOrgID := "1"
			database.CreateOrganization(fallbackOrgID, "DUMMY", "Fallback Org")
			req.OrgID = fallbackOrgID
		}
	}

	if err := database.CreateConfigCredential(req.OrgID, req.ContextName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config session"})
		return
	}

	token, _ := database.GenerateTerminalToken()
	StoreSessionToken(token, TokenSessionInfo{
		OrgID:       req.OrgID,
		ContextName: req.ContextName,
	})

	scheme := "http"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	backendURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	command := fmt.Sprintf("curl -fsSL %s/api/session/agent-script | bash -s %s %s \"%s/api\"", backendURL, req.ContextName, token, backendURL)
	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"command": command,
	})
}

func UploadConfigHandler(c *gin.Context) {
	token := c.PostForm("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	file, err := c.FormFile("config")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Config file is required"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config file"})
		return
	}
	defer f.Close()

	buf := make([]byte, file.Size)
	f.Read(buf)
	configContent := string(buf)

	re := regexp.MustCompile(`(https?://)(127\.0\.0\.1|localhost)(:\d+)`)
	configContent = re.ReplaceAllString(configContent, "${1}host.docker.internal${3}")

	info, err := LookupSessionToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token session"})
		return
	}

	if err := database.UpdateConfigCredential(info.OrgID, info.ContextName, configContent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	configPath := filepath.Join(ConfigDir, token+".yaml")
	os.WriteFile(configPath, []byte(configContent), 0600)

	go func(path string) {
		time.Sleep(30 * time.Minute)
		sessionMu.Lock()
		defer sessionMu.Unlock()
		os.Remove(path)
	}(configPath)

	c.JSON(http.StatusOK, gin.H{"message": "Config uploaded successfully, valid for 10 minutes"})
}

// ─── AgentWsHandler: WebSocket for the AGENT on the user's machine ───

func AgentWsHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Agent WS upgrade failed:", err)
		return
	}
	defer conn.Close()

	session := GetOrCreateSession(token)
	session.SetAgentConnected(true)
	session.SetAgentConn(conn)
	defer func() {
		session.SetAgentConnected(false)
		session.SetAgentConn(nil)
		if fc := session.GetFrontendConn(); fc != nil {
			fc.WriteMessage(websocket.BinaryMessage, []byte("\r\n⚠ Agent disconnected.\r\naegios:~$ "))
		}
	}()

	log.Printf("Agent connected for token: %s", token[:8])

	done := make(chan struct{})

	// Goroutine: Ping keepalive every 20 seconds to prevent agent timeout
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingMsg, _ := json.Marshal(map[string]string{"type": "ping"})
				if err := conn.WriteMessage(websocket.TextMessage, pingMsg); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Goroutine: Read commands from session channel → send to agent
	go func() {
		for {
			select {
			case cmd, ok := <-session.Commands:
				if !ok {
					return
				}
				if cmd != "" {
					msg, _ := json.Marshal(map[string]string{
						"type":    "command",
						"command": cmd,
					})
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						log.Println("Failed to send command to agent:", err)
						return
					}
				}
			case <-done:
				return
			}
		}
	}()

	// Main loop: Read messages from agent → put results in session
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Agent disconnected (token: %s): %v", token[:8], err)
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(p, &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)

		switch msgType {
		case "agent_ready":
			log.Printf("Agent ready for token: %s", token[:8])

		case "result":
			output, _ := msg["output"].(string)
			select {
			case session.Results <- output:
			default:
				log.Println("Result channel full, dropping result")
			}

		case "output":
			// Phase 3: streaming remediation output from agent
			outputLine, _ := msg["output"].(string)
			commandIDFloat, _ := msg["command_id"].(float64)
			commandID := int(commandIDFloat)

			// Append output to DB
			if commandID > 0 {
				go database.AppendRemediationOutput(commandID, outputLine)
			}

			// Forward to frontend terminal
			if fc := session.GetFrontendConn(); fc != nil {
				termLine := strings.ReplaceAll(outputLine, "\n", "\r\n")
				fc.WriteMessage(websocket.BinaryMessage, []byte(termLine+"\r\n"))
			}

			// Also push to Results channel for non-terminal consumers
			select {
			case session.Results <- outputLine:
			default:
			}

		case "done":
			// Phase 3: remediation execution completed
			commandIDFloat, _ := msg["command_id"].(float64)
			commandID := int(commandIDFloat)
			exitCodeFloat, _ := msg["exit_code"].(float64)
			exitCode := int(exitCodeFloat)

			status := "success"
			statusEmoji := "✅"
			if exitCode != 0 {
				status = "failed"
				statusEmoji = "❌"
			}

			if commandID > 0 {
				go database.CompleteRemediation(commandID, status, exitCode)
			}

			// Forward completion to frontend terminal
			if fc := session.GetFrontendConn(); fc != nil {
				doneLine := fmt.Sprintf("\r\n%s Remediation %s (exit %d)\r\naegios:~$ ", statusEmoji, status, exitCode)
				fc.WriteMessage(websocket.BinaryMessage, []byte(doneLine))
			}

			log.Printf("Remediation %s for command_id=%d (exit %d)", status, commandID, exitCode)

		case "error":
			// Phase 3: remediation execution error
			commandIDFloat, _ := msg["command_id"].(float64)
			commandID := int(commandIDFloat)
			errorMsg, _ := msg["output"].(string)

			if commandID > 0 {
				go func() {
					database.AppendRemediationOutput(commandID, "ERROR: "+errorMsg)
					database.CompleteRemediation(commandID, "failed", 1)
				}()
			}

			if fc := session.GetFrontendConn(); fc != nil {
				fc.WriteMessage(websocket.BinaryMessage, []byte("\r\n❌ Remediation error: "+errorMsg+"\r\naegios:~$ "))
			}

		case "pong":
			// ignore
		}
	}

	close(done)
}

// ─── Cluster Reachability Check ───

// checkClusterReachable tests if kubectl can reach the cluster from inside the container
func checkClusterReachable(token string) bool {
	configPath := filepath.Join(ConfigDir, token+".yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Write config file if not on disk yet
		info, err := LookupSessionToken(token)
		if err != nil {
			return false
		}
		configData, err := database.GetConfigCredential(info.OrgID, info.ContextName)
		if err != nil || configData == "" {
			return false
		}
		os.WriteFile(configPath, []byte(configData), 0600)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "cluster-info", "--request-timeout=3s", "--kubeconfig", configPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", configPath))
	err := cmd.Run()
	return err == nil
}

// ClusterModeHandler checks if the cluster is reachable from the backend directly
func ClusterModeHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token required"})
		return
	}

	reachable := checkClusterReachable(token)
	mode := "agent"
	if reachable {
		mode = "direct"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    mode,
		"message": fmt.Sprintf("Cluster mode: %s", mode),
	})
}

// ─── WsHandler: Dual-mode WebSocket for FRONTEND browser terminal ───

func WsHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	info, err := LookupSessionToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	configData, err := database.GetConfigCredential(info.OrgID, info.ContextName)
	if err != nil {
		log.Printf("Warning: Config not found or DB error for session %s: %v", token[:8], err)
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Frontend WS upgrade failed:", err)
		return
	}
	defer conn.Close()

	// Auto-detect mode: can backend reach the cluster directly?
	configPath := filepath.Join(ConfigDir, token+".yaml")
	if configData != "" {
		os.WriteFile(configPath, []byte(configData), 0600)
	}

	if configData != "" && checkClusterReachable(token) {
		// ═══ DIRECT MODE: PTY shell inside container ═══
		log.Printf("[%s] Direct mode — cluster reachable from backend", token[:8])
		wsHandleDirect(conn, token, configPath)
	} else {
		// ═══ AGENT MODE: Bridge to remote agent ═══
		log.Printf("[%s] Agent mode — bridging to remote agent", token[:8])
		wsHandleAgent(conn, token)
	}
}

// wsHandleDirect handles the terminal via PTY (cluster reachable from backend)
func wsHandleDirect(conn *websocket.Conn, token, configPath string) {
	cmd := exec.Command("bash", "--noprofile", "--norc")
	cmd.Dir = "/tmp"
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", configPath))
	cmd.Env = append(cmd.Env, "PS1=aegios:~$ ")
	cmd.Env = append(cmd.Env, "TERM=xterm")
	cmd.Env = append(cmd.Env, "HOME=/tmp")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Println("Failed to start pty:", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to start terminal\n"))
		return
	}
	defer ptmx.Close()
	defer cmd.Process.Kill()

	conn.WriteMessage(websocket.BinaryMessage, []byte("\r\n🚀 Direct mode — cluster reachable from backend.\r\n"))

	// Auto-execute remediation command if available
	remediationCmd, _, err := database.GetLatestAgentOutput(token)
	if err == nil && remediationCmd != "" {
		go func() {
			time.Sleep(500 * time.Millisecond)
			ptmx.Write([]byte(remediationCmd + "\n"))
		}()
	}

	conn.SetCloseHandler(func(code int, text string) error {
		ptmx.Close()
		cmd.Process.Kill()
		return nil
	})

	// 10 min timeout
	go func() {
		time.Sleep(30 * time.Minute)
		conn.WriteMessage(websocket.TextMessage, []byte("\r\n[Aegios] Session timed out.\r\n"))
		ptmx.Close()
		conn.Close()
	}()

	// PTY → WS
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Println("PTY read error:", err)
				}
				break
			}
			if err = conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				break
			}
		}
	}()

	// WS → PTY
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			break
		}
		ptmx.Write(p)
	}
}

// wsHandleAgent handles the terminal via remote agent bridge
func wsHandleAgent(conn *websocket.Conn, token string) {
	session := GetOrCreateSession(token)
	session.SetFrontendConn(conn)
	defer session.SetFrontendConn(nil)

	conn.WriteMessage(websocket.BinaryMessage, []byte("\r\n🚀 Agent mode — commands will execute on your machine.\r\n"))

	if session.IsAgentConnected() {
		conn.WriteMessage(websocket.BinaryMessage, []byte("✅ Agent is online — type kubectl commands below.\r\naegios:~$ "))
	} else {
		conn.WriteMessage(websocket.BinaryMessage, []byte("⏳ Waiting for agent to come online...\r\naegios:~$ "))
	}

	// Auto-execute remediation command
	remediationCmd, _, err := database.GetLatestAgentOutput(token)
	if err == nil && remediationCmd != "" {
		go func() {
			time.Sleep(1 * time.Second)
			conn.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("\r\n🔧 Auto-executing: %s\r\n", remediationCmd)))
			select {
			case session.Commands <- remediationCmd:
			case <-time.After(5 * time.Second):
				conn.WriteMessage(websocket.BinaryMessage, []byte("\r\n⚠ Agent not responding.\r\naegios:~$ "))
			}
		}()
	}

	done := make(chan struct{})

	go func() {
		time.Sleep(30 * time.Minute)
		conn.WriteMessage(websocket.BinaryMessage, []byte("\r\n[Aegios] Session timed out.\r\n"))
		conn.Close()
		close(done)
	}()

	// Results from agent → frontend
	go func() {
		for {
			select {
			case result, ok := <-session.Results:
				if !ok {
					return
				}
				termOutput := strings.ReplaceAll(result, "\n", "\r\n")
				conn.WriteMessage(websocket.BinaryMessage, []byte(termOutput))
				conn.WriteMessage(websocket.BinaryMessage, []byte("\r\naegios:~$ "))
			case <-done:
				return
			}
		}
	}()

	// Frontend input → agent commands
	var cmdBuffer string
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			break
		}

		input := string(p)
		for _, ch := range input {
			switch {
			case ch == '\r' || ch == '\n':
				if strings.TrimSpace(cmdBuffer) != "" {
					conn.WriteMessage(websocket.BinaryMessage, []byte("\r\n"))
					if !session.IsAgentConnected() {
						conn.WriteMessage(websocket.BinaryMessage, []byte("⚠ Agent not connected.\r\naegios:~$ "))
					} else {
						select {
						case session.Commands <- cmdBuffer:
						default:
							conn.WriteMessage(websocket.BinaryMessage, []byte("⚠ Agent buffer full.\r\naegios:~$ "))
						}
					}
				} else {
					conn.WriteMessage(websocket.BinaryMessage, []byte("\r\naegios:~$ "))
				}
				cmdBuffer = ""
			case ch == 127 || ch == 8:
				if len(cmdBuffer) > 0 {
					cmdBuffer = cmdBuffer[:len(cmdBuffer)-1]
					conn.WriteMessage(websocket.BinaryMessage, []byte("\b \b"))
				}
			case ch == 3:
				cmdBuffer = ""
				conn.WriteMessage(websocket.BinaryMessage, []byte("^C\r\naegios:~$ "))
			default:
				cmdBuffer += string(ch)
				conn.WriteMessage(websocket.BinaryMessage, []byte(string(ch)))
			}
		}
	}
}

// ─── TakeAction / Status / Script Serving ───

func TakeActionHandler(c *gin.Context) {
	var req struct {
		Token         string `json:"token"`
		FindingID     string `json:"finding_id"`
		Command       string `json:"command"`
		CorrectConfig string `json:"correct_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// ── Phase 3: finding_id-based remediation (non-blocking) ──
	if req.FindingID != "" && req.Token != "" {
		info, err := LookupSessionToken(req.Token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Fetch command from agent_output by finding_id
		command, correctConfig, err := database.GetRemediationCommandByFinding(req.FindingID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No remediation command found for this finding"})
			return
		}

		// Check agent connection OR direct config upload mode
		session := GetSession(req.Token)

		var configContent string
		hasDirectConfig := false
		if info.ContextName != "" {
			conf, errConf := database.GetConfigCredential(info.OrgID, info.ContextName)
			if errConf == nil && conf != "" && len(conf) > 10 {
				hasDirectConfig = true
				configContent = conf
			}
		}

		if (session == nil || !session.IsAgentConnected()) && !hasDirectConfig {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No agent connected. Please run the agent script or upload a config first."})
			return
		}

		// Create remediation execution record
		execID, err := database.CreateRemediationExecution(req.FindingID, info.OrgID, command)
		if err != nil {
			log.Printf("Failed to create remediation execution: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create execution record"})
			return
		}

		// Send execute command to agent via WebSocket
		var agentConn *websocket.Conn
		if session != nil {
			agentConn = session.GetAgentConn()
		}

		if agentConn != nil {
			execMsg, _ := json.Marshal(map[string]interface{}{
				"type":       "execute",
				"command_id": execID,
				"command":    command,
				"manifest":   correctConfig,
			})
			if err := agentConn.WriteMessage(websocket.TextMessage, execMsg); err != nil {
				log.Printf("Failed to send execute to agent: %v", err)
				database.UpdateRemediationStatus(execID, "failed")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send command to agent"})
				return
			}
		} else if hasDirectConfig {
			// Direct Execution fallback using Uploaded kubeconfig
			go func(cmdLine string, execID int, conf string, manifest string) {
				tmpfile, err := os.CreateTemp("", "kubeconfig-*")
				if err != nil {
					database.UpdateRemediationStatus(execID, "failed")
					return
				}
				defer os.Remove(tmpfile.Name())
				tmpfile.WriteString(conf)
				tmpfile.Close()

				// If it's an apply -f command, create the manifest locally
				var manifestFile string
				if manifest != "" && strings.Contains(cmdLine, "apply -f") {
					re := regexp.MustCompile(`-f\s+([a-zA-Z0-9_.-]+)`)
					matches := re.FindStringSubmatch(cmdLine)
					if len(matches) > 1 {
						manifestFile = matches[1]
						os.WriteFile(manifestFile, []byte(manifest), 0644)
						defer os.Remove(manifestFile)
					}
				}

				// Execute kubectl using bash wrapper
				cmd := exec.Command("bash", "-c", cmdLine)
				cmd.Env = append(os.Environ(), "KUBECONFIG="+tmpfile.Name())
				out, err := cmd.CombinedOutput()

				exitCode := 0
				status := "success"
				if err != nil {
					exitCode = 1
					status = "failed"
				}

				database.AppendRemediationOutput(execID, string(out))
				database.CompleteRemediation(execID, status, exitCode)
			}(command, execID, configContent, correctConfig)
		}

		// Update status to running
		database.UpdateRemediationStatus(execID, "running")

		// Send separator to frontend terminal
		if fc := session.GetFrontendConn(); fc != nil {
			separator := fmt.Sprintf("\r\n─────────────────────────────────────\r\n🚀 Executing remediation for Finding #%s\r\n─────────────────────────────────────\r\n", req.FindingID)
			fc.WriteMessage(websocket.BinaryMessage, []byte(separator))
		}

		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"execution_id": execID,
			"status":       "running",
			"message":      "Command sent to agent. Watch terminal for output.",
		})
		return
	}

	// ── Legacy flow (backward compatible): token + command ──
	if req.Token == "" || req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token and command (or finding_id) are required"})
		return
	}

	info, err := LookupSessionToken(req.Token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid token"})
		return
	}

	err = database.SaveAgentOutput(info.OrgID, req.Token, req.Command, req.CorrectConfig, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store command"})
		return
	}

	// Also send to the live agent immediately if connected
	session := GetSession(req.Token)
	if session != nil && session.IsAgentConnected() {
		select {
		case session.Commands <- req.Command:
		default:
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Remediation command stored and queued.",
	})
}

func GetActionStatusHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token required"})
		return
	}

	command, correctConfig, err := database.GetLatestAgentOutput(token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "has_action": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"has_action":     true,
		"command":        command,
		"correct_config": correctConfig,
	})
}

// ─── Phase 3: RemediationStatusHandler ───

func RemediationStatusHandler(c *gin.Context) {
	execIDStr := c.Query("execution_id")
	if execIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution_id is required"})
		return
	}

	var execID int
	fmt.Sscanf(execIDStr, "%d", &execID)
	if execID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid execution_id"})
		return
	}

	exec, err := database.GetRemediationByID(execID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Execution not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"execution_id": exec.ID,
		"finding_id":   exec.FindingID,
		"status":       exec.Status,
		"output":       exec.Output,
		"exit_code":    exec.ExitCode,
		"executed_at":  exec.ExecutedAt,
		"completed_at": exec.CompletedAt,
	})
}

func AgentScriptHandler(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	backendURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	script := strings.Replace(agentScript, "@@BACKEND_URL@@", backendURL+"/api", 1)
	script = strings.ReplaceAll(script, "\r", "")

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", "inline; filename=\"kube-connect-script.sh\"")
	c.String(http.StatusOK, script)
}

func AgentScriptPyHandler(c *gin.Context) {
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", "inline; filename=\"agent.py\"")
	script := strings.ReplaceAll(agentPythonScript, "\r", "")
	c.String(http.StatusOK, script)
}

// AgentScriptV2Handler serves the bash script tailored for the Phase 2 flow
func AgentScriptV2Handler(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	backendURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	script := strings.Replace(agentScriptV2, "@@BACKEND_URL@@", backendURL+"/api", 1)
	script = strings.ReplaceAll(script, "\r", "")

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", "inline; filename=\"kube-connect-script-v2.sh\"")
	c.String(http.StatusOK, script)
}

func ConfigStatusHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token required"})
		return
	}

	info, err := LookupSessionToken(token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "has_config": false})
		return
	}

	configData, err := database.GetConfigCredential(info.OrgID, info.ContextName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "has_config": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"has_config": configData != "",
	})
}

// AgentPollHandler — HTTP fallback for agent polling (if WebSocket is unavailable)
func AgentPollHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	session := GetOrCreateSession(token)
	session.SetAgentConnected(true)

	select {
	case cmd := <-session.Commands:
		c.JSON(http.StatusOK, gin.H{"command": cmd})
	case <-time.After(25 * time.Second):
		c.JSON(http.StatusOK, gin.H{"command": ""})
	}
}

// AgentResultHandler — HTTP fallback for receiving results
func AgentResultHandler(c *gin.Context) {
	var req struct {
		Token     string `json:"token"`
		Output    string `json:"output"`
		OutputB64 string `json:"output_b64"`
		ExitCode  int    `json:"exit_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	session := GetSession(req.Token)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active session"})
		return
	}

	output := req.Output
	if req.OutputB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.OutputB64)
		if err == nil {
			output = string(decoded)
		}
	}

	select {
	case session.Results <- output:
		c.JSON(http.StatusOK, gin.H{"success": true})
	case <-time.After(5 * time.Second):
		c.JSON(http.StatusOK, gin.H{"success": true, "warning": "No frontend connected"})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Phase 2: New Handlers — Session Init, Upload Config (Bearer), Status Polling
// All existing handlers above are UNTOUCHED.
// ═══════════════════════════════════════════════════════════════════════════════

// SessionInitHandler handles POST /session/init
// Generates an in-memory token (never stored in DB) and returns a curl command.
func SessionInitHandler(c *gin.Context) {
	var req struct {
		ContextName  string `json:"context_name"`
		SessionToken string `json:"session_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.ContextName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_name is required"})
		return
	}

	// Extract session token from JSON body or Authorization header
	sessionToken := req.SessionToken
	if sessionToken == "" {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			sessionToken = authHeader[7:]
		}
	}

	var orgID string

	if sessionToken != "" {
		githubUsername, err := database.ValidateSession(sessionToken)
		if err == nil {
			orgID, _ = database.GetUserOrgID(githubUsername)
		}
	}

	// Fallback mechanism
	if orgID == "" {
		err := database.DB.QueryRow(`SELECT org_id FROM organization LIMIT 1`).Scan(&orgID)
		if err != nil || orgID == "" {
			fallbackOrgID := "1"
			database.CreateOrganization(fallbackOrgID, "DUMMY", "Fallback Org")
			orgID = fallbackOrgID
		}
	}

	// Save pending record in DB (NO token in DB)
	if err := database.CreatePendingConfig(orgID, req.ContextName); err != nil {
		log.Printf("Phase2: Failed to create pending config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize session"})
		return
	}

	// Generate in-memory token
	token, err := GeneratePhase2Token()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Store token in memory only (15-min TTL, one-time use)
	StorePhase2Token(token, TokenSessionInfo{
		OrgID:       orgID,
		ContextName: req.ContextName,
	})

	// Build the curl command
	scheme := "http"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	backendURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	curlCommand := fmt.Sprintf(
		"curl -fsSL %s/api/session/agent-script-v2 | bash -s %s %s \"%s/api\"",
		backendURL, req.ContextName, token, backendURL,
	)

	c.JSON(http.StatusOK, gin.H{
		"curl_command": curlCommand,
		"org_id":       orgID,
		"token":        token,
	})
}

// UploadConfigPhase2Handler handles POST /api/upload-config
// Validates a Bearer token from memory, stores the kubeconfig in DB, and invalidates the token.
func UploadConfigPhase2Handler(c *gin.Context) {
	// Extract Bearer token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
		return
	}
	token := authHeader[7:]

	// Look up token in memory
	info, err := LookupPhase2Token(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Read the uploaded config file
	file, err := c.FormFile("config")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Config file is required"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config file"})
		return
	}
	defer f.Close()

	buf := make([]byte, file.Size)
	f.Read(buf)
	configContent := string(buf)

	// Rewrite localhost/127.0.0.1 → host.docker.internal (same as existing UploadConfigHandler)
	re := regexp.MustCompile(`(https?://)(127\.0\.0\.1|localhost)(:\d+)`)
	configContent = re.ReplaceAllString(configContent, "${1}host.docker.internal${3}")

	// Skip TLS verification when using host.docker.internal
	if strings.Contains(configContent, "host.docker.internal") {
		// Replace certificate-authority-data with insecure-skip-tls-verify
		reCA := regexp.MustCompile(`(?m)^\s*certificate-authority-data:.*$`)
		configContent = reCA.ReplaceAllString(configContent, "    insecure-skip-tls-verify: true")
	}

	// Persist in DB → status = 'active'
	if err := database.ActivateConfig(info.OrgID, info.ContextName, configContent); err != nil {
		log.Printf("Phase2: Failed to activate config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	// Create long-lived session token
	sessionToken, _ := database.GenerateTerminalToken()
	StoreSessionToken(sessionToken, TokenSessionInfo{
		OrgID:       info.OrgID,
		ContextName: info.ContextName,
	})

	// Delete token from memory (one-time use — now invalidated)
	DeletePhase2Token(token)

	log.Printf("Phase2: Config uploaded for org=%s context=%s", info.OrgID, info.ContextName)

	c.JSON(http.StatusOK, gin.H{
		"message":       "Config uploaded successfully",
		"session_token": sessionToken,
	})
}

// SessionStatusHandler handles GET /session/status
// Polls the DB for the current status of a config_credentials record.
func SessionStatusHandler(c *gin.Context) {
	contextName := c.Query("context_name")
	orgID := c.Query("org_id")

	if contextName == "" || orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_name and org_id are required"})
		return
	}

	status, err := database.GetConfigStatusByContext(orgID, contextName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
		return
	}

	token := ""
	if status == "active" {
		token, _ = GetSessionTokenByOrg(orgID)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        status,
		"session_token": token,
	})
}
