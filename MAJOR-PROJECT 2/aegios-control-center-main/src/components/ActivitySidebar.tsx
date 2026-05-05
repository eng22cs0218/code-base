import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { AlertCircle, CheckCircle, Info } from "lucide-react";
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

const ActivitySidebar = () => {
  const { activities } = useSecurityContext();

  return (
    <Card className="h-[calc(100vh-6rem)] border-cyber-border bg-card glow-border border-double">
      <CardHeader>
        <CardTitle className="text-lg text-primary">Recent Activity</CardTitle>
      </CardHeader>
      <CardContent>
        <ScrollArea className="h-[calc(100vh-12rem)]">
          <div className="space-y-4">
            {activities.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-4">No recent activity</p>
            ) : (
              activities.map((activity) => {
                const Icon = getIconForType(activity.type);
                return (
                  <div key={activity.id} className="flex gap-3 p-3 rounded-lg bg-secondary border border-cyber-border hover:border-primary transition-colors">
                    <Icon className={`h-5 w-5 mt-0.5 ${activity.type === "success" ? "text-primary" : activity.type === "warning" ? "text-destructive" : "text-accent"}`} />
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
export default ActivitySidebar;