import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Play, ChevronDown, ChevronRight } from "lucide-react";

interface CommandInputAreaProps {
  serviceName: string;
  onApply: (command: string) => Promise<{ success: boolean; message: string; output?: string }>;
}

const CommandInputArea = ({ serviceName, onApply }: CommandInputAreaProps) => {
  const [command, setCommand] = useState("");
  const [isApplying, setIsApplying] = useState(false);
  const [output, setOutput] = useState<string | null>(null);
  const [isOutputOpen, setIsOutputOpen] = useState(false);

  const handleApply = async () => {
    if (!command.trim()) return;

    setIsApplying(true);
    try {
      const result = await onApply(command.trim());
      if (result.success) {
        setOutput(result.output || result.message);
        setIsOutputOpen(true);
        setCommand("");
      }
    } catch (error) {
      console.error("Failed to apply command:", error);
    } finally {
      setIsApplying(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Command Input */}
      <Card className="neon-border bg-card">
        <CardHeader>
          <CardTitle className="text-primary neon-text">Apply Security Action</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="relative">
            <Textarea
              placeholder="Type your text here or command"
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              className="min-h-[120px] bg-secondary/50 neon-border focus:border-primary resize-none pr-20"
              disabled={isApplying}
              aria-label={`Command input for service ${serviceName}`}
            />
            <Button
              onClick={handleApply}
              disabled={!command.trim() || isApplying}
              className="absolute bottom-3 right-3 bg-primary hover:bg-primary/80 text-primary-foreground neon-border"
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
              <CardContent>
                <pre className="bg-secondary/50 neon-border rounded p-4 text-sm text-foreground overflow-x-auto whitespace-pre-wrap">
                  {output}
                </pre>
              </CardContent>
            </CollapsibleContent>
          </Card>
        </Collapsible>
      )}
    </div>
  );
};

export default CommandInputArea;