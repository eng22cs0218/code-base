import { PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useSecurityContext } from "@/contexts/SecurityContext";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle } from 'lucide-react';

const COLORS = {
  Low: '#f97316',    // Tailwind orange-500
  High: '#eab308',   // Tailwind yellow-500
  Critical: '#FF0000' // Pure red
};

const ScoreVisuals = () => {
  const { score, isLoading } = useSecurityContext();

  if (isLoading || !score) {
    return (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card className="border-cyber-border bg-card glow-border">
          <CardHeader>
            <Skeleton className="h-6 w-32" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-64 w-full" />
          </CardContent>
        </Card>
        <Card className="border-cyber-border bg-card glow-border">
          <CardHeader>
            <Skeleton className="h-6 w-32" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-64 w-full" />
          </CardContent>
        </Card>
      </div>
    );
  }

  if (score.total === 0) {
    return (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card className="border-cyber-border bg-card glow-border">
          <CardHeader>
            <CardTitle className="text-green-muted">Security Status Distribution</CardTitle>
          </CardHeader>
          <CardContent className="h-64 flex items-center justify-center text-center">
            <div className="space-y-2">
              <AlertCircle className="h-10 w-10 mx-auto text-muted-foreground" />
              <p className="text-muted-foreground">No data available. Run validation to generate findings.</p>
            </div>
          </CardContent>
        </Card>
        <Card className="border-cyber-border bg-card glow-border">
          <CardHeader>
            <CardTitle className="text-green-muted">Risk Levels</CardTitle>
          </CardHeader>
          <CardContent className="h-64 flex items-center justify-center text-center">
            <div className="space-y-2">
              <AlertCircle className="h-10 w-10 mx-auto text-muted-foreground" />
              <p className="text-muted-foreground">No data available. Run validation to generate findings.</p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  const pieData = [
    { name: 'Low', value: score.percentages.Low, count: score.counts.Low, colorKey: 'Low' },
    { name: 'High', value: score.percentages.High, count: score.counts.High, colorKey: 'High' },
    { name: 'Critical', value: score.percentages.Critical, count: score.counts.Critical, colorKey: 'Critical' }
  ].filter(item => item.value > 0);

  const barData = [
    { name: 'Low', count: score.counts.Low, percentage: score.percentages.Low },
    { name: 'High', count: score.counts.High, percentage: score.percentages.High },
    { name: 'Critical', count: score.counts.Critical, percentage: score.percentages.Critical }
  ];

  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload;
      const status = data.name;
      
      return (
        <div className="bg-card/95 border border-green-500/50 rounded-lg p-3 shadow-md backdrop-blur-sm max-w-xs">
          <p className="text-green-400 font-semibold">{status} Findings</p>
          <p className="text-sm text-muted-foreground">
            {data.count} findings ({data.value.toFixed(2)}%)
          </p>
        </div>
      );
    }
    return null;
  };

  const CustomBarTooltip = ({ active, payload }: any) => {
    if (!active || !payload || payload.length === 0) {
      return null;
    }
    
    const data = payload[0].payload;
    
    return (
      <div className="bg-card/95 border border-green-500/50 rounded-lg p-3 shadow-md backdrop-blur-sm">
        <p className="text-green-400 font-semibold">{data.name}</p>
        <p className="text-sm text-foreground">Count: {data.count}</p>
        <p className="text-sm text-muted-foreground">Percentage: {data.percentage.toFixed(2)}%</p>
      </div>
    );
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* Pie Chart */}
      <Card className="border-cyber-border bg-card glow-border hover:border-green-500/50 transition-all duration-300">
        <CardHeader>
          <CardTitle className="text-green-muted">Security Status Distribution</CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={350}>
            <PieChart margin={{ top: 20, right: 30, bottom: 20, left: 40 }}>
              <Pie
                data={pieData}
                cx="50%"
                cy="50%"
                labelLine={{
                  stroke: 'hsl(var(--muted-foreground))',
                  strokeWidth: 1
                }}
                label={({ name, value }) => `${name}: ${value.toFixed(2)}%`}
                outerRadius={90}
                fill="#8884d8"
                dataKey="value"
                stroke="#000000"
              >
                {pieData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[entry.colorKey as keyof typeof COLORS]} stroke="#000000" />
                ))}
              </Pie>
              <Tooltip content={<CustomTooltip />} cursor={false} />
            </PieChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>

      {/* Bar Chart */}
      <Card className="border-cyber-border bg-card glow-border hover:border-green-500/50 transition-all duration-300">
        <CardHeader>
          <CardTitle className="text-green-muted">Risk Levels</CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={350}>
            <BarChart data={barData} margin={{ top: 20, right: 30, bottom: 20, left: 10 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--cyber-border))" />
              <XAxis 
                dataKey="name" 
                stroke="hsl(var(--muted-foreground))"
                fontSize={12}
              />
              <YAxis 
                stroke="hsl(var(--muted-foreground))"
                fontSize={12}
              />
              <Tooltip 
                content={<CustomBarTooltip />}
                cursor={false}
                wrapperStyle={{ outline: 'none' }}
              />
              <Bar 
                dataKey="count" 
                radius={[4, 4, 0, 0]}
              >
                {barData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[entry.name as keyof typeof COLORS]} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>
    </div>
  );
};

export default ScoreVisuals;