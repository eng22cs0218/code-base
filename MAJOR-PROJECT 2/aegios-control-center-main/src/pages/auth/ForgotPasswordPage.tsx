import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Shield, ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useAuth } from '@/contexts/AuthContext';
import { toast } from 'sonner';

const ForgotPasswordPage = () => {
  const { resetPassword } = useAuth();
  const [usernameOrEmail, setUsernameOrEmail] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!usernameOrEmail) {
      toast.error('Please enter your GitHub username or email');
      return;
    }

    setIsLoading(true);
    const success = await resetPassword(usernameOrEmail);
    setIsLoading(false);

    if (success) {
      setIsSubmitted(true);
      toast.success('Password reset instructions sent!');
    } else {
      toast.error('Failed to send reset instructions');
    }
  };

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4">
      <Card className="w-full max-w-md border-cyber-border bg-card glow-border">
        <CardHeader className="text-center space-y-4">
          <div className="flex justify-center">
            <img src="/aegios_logo.png" alt="Aegios Logo" className="h-16 w-16 object-contain rounded-md" />
          </div>
          <CardTitle className="text-3xl font-bold text-primary glow-text">
            Aegios
          </CardTitle>
          <p className="text-muted-foreground">Reset your password</p>
        </CardHeader>
        <CardContent>
          {!isSubmitted ? (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="usernameOrEmail" className="text-foreground">
                  GitHub Username or Email
                </Label>
                <Input
                  id="usernameOrEmail"
                  type="text"
                  placeholder="Enter your GitHub username or email"
                  value={usernameOrEmail}
                  onChange={(e) => setUsernameOrEmail(e.target.value)}
                  className="bg-secondary border-cyber-border text-foreground"
                  disabled={isLoading}
                />
              </div>

              <Button
                type="submit"
                className="w-full bg-primary hover:bg-primary/90 text-background"
                disabled={isLoading}
              >
                {isLoading ? 'Sending...' : 'Reset Password'}
              </Button>

              <div className="text-center">
                <Link
                  to="/login"
                  className="inline-flex items-center gap-2 text-sm text-primary hover:text-primary/80 transition-colors"
                >
                  <ArrowLeft className="h-4 w-4" />
                  Back to Login
                </Link>
              </div>
            </form>
          ) : (
            <div className="space-y-4">
              <div className="p-4 rounded-lg bg-primary/10 border border-primary/20">
                <p className="text-sm text-foreground text-center">
                  If the account exists, a reset link has been sent to your registered email address.
                </p>
              </div>

              <div className="text-center">
                <Link
                  to="/login"
                  className="inline-flex items-center gap-2 text-sm text-primary hover:text-primary/80 transition-colors"
                >
                  <ArrowLeft className="h-4 w-4" />
                  Back to Login
                </Link>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};

export default ForgotPasswordPage;
