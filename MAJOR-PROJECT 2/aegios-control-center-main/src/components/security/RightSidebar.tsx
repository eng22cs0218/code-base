import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { AlertCircle, CheckCircle, Info, WifiOff } from "lucide-react";
import { useSecurityContext } from "@/contexts/SecurityContext";

const getTimeAgo = (date: Date) => {
  const seconds = Math.floor((new Date().getTime() - new Date(date).getTime()) / 1000);
  let interval = seconds / 31536000;
  if (interval > 1) return Math.floor(interval) + " years ago";
  interval = seconds / 2592000;
  if (interval > 1) return Math.floor(interval) + " months ago";
  interval = seconds / 86400;
  if (interval > 1) return Math.floor(interval) + " days ago";
  interval = seconds / 3600;
  if (interval > 1) return Math.floor(interval) + " hours ago";
  interval = seconds / 60;
  if (interval >= 1) return Math.floor(interval) + " min ago";
  if (seconds < 30) return "Just now";
  return Math.floor(seconds) + " sec ago";
};

const getIconForType = (type: string) => {
  switch (type) {
    case 'success':
      return CheckCircle;
    case 'warning':
      return AlertCircle;
    default:
      return Info;
  }
};

const RightSidebar = () => {
  const { isConnected, activities } = useSecurityContext();

  return (
    <Card className="h-full border-cyber-border bg-card glow-border border-double flex flex-col min-h-0">
      <CardHeader className="flex-shrink-0">
        <div className="flex items-center justify-between">
          <CardTitle className="text-lg text-primary glow-text">Recent Activity</CardTitle>
          {!isConnected && (
            <div className="flex items-center gap-1 text-xs text-destructive">
              <WifiOff className="h-3 w-3" />
              <span>Disconnected</span>
            </div>
          )}
        </div>
      </CardHeader>
      <CardContent className="flex-1 overflow-hidden p-0">
        <ScrollArea className="h-full px-6 pb-6">
          <div className="space-y-4">
            {activities.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-4">No recent activity</p>
            ) : (
              activities.map((activity) => {
                const Icon = getIconForType(activity.type);
                return (
                  <div 
                    key={activity.id} 
                    className="flex gap-3 p-3 rounded-lg bg-secondary border border-cyber-border hover:border-primary transition-colors"
                  >
                    <Icon className={`h-5 w-5 mt-0.5 ${
                      activity.type === "success" 
                        ? "text-primary" 
                        : activity.type === "warning" 
                        ? "text-destructive" 
                        : "text-accent"
                    }`} />
                    <div className="flex-1 space-y-1">
                      <p className="text-sm text-foreground leading-tight">
                        {activity.message}
                      </p>
                      <p className="text-xs text-muted-foreground">{getTimeAgo(activity.timestamp)}</p>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  );
};

export default RightSidebar;