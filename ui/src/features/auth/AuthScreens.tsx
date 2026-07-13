import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";
import { useBootstrapMutation, useLogin, useBootstrapQuery } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";

export function AuthCard({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-b from-background to-background/80 p-6">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <div className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground uppercase">AstreoGateway</div>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight">{title}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">{children}</div>
        <p className="mt-6 text-center text-xs text-muted-foreground">{subtitle}</p>
      </div>
    </div>
  );
}

export function Bootstrap() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const mut = useBootstrapMutation();
  const navigate = useNavigate();

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await mut.mutateAsync({ username, password });
      navigate("/");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Bootstrap failed");
    }
  };

  return (
    <AuthCard title="Create admin" subtitle="First-run setup">
      <form onSubmit={onSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="username">Username</Label>
          <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} autoFocus required />
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">Password</Label>
          <Input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
          <p className="text-xs text-muted-foreground">At least 8 characters.</p>
        </div>
        <Button type="submit" className="w-full" disabled={mut.isPending}>
          {mut.isPending ? <Spinner /> : "Create admin & sign in"}
        </Button>
      </form>
    </AuthCard>
  );
}

export function Login() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const login = useLogin();
  const bootstrapQ = useBootstrapQuery();
  const navigate = useNavigate();

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await login.mutateAsync({ username, password });
      navigate("/");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Sign in failed");
    }
  };

  return (
    <AuthCard title="Sign in" subtitle="AstreoGateway admin">
      <form onSubmit={onSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="username">Username</Label>
          <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} autoFocus required />
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">Password</Label>
          <Input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        <Button type="submit" className="w-full" disabled={login.isPending}>
          {login.isPending ? <Spinner /> : "Sign in"}
        </Button>
        {bootstrapQ.data?.needed && (
          <p className="text-center text-xs text-muted-foreground">No admin exists yet. Refresh to bootstrap.</p>
        )}
      </form>
    </AuthCard>
  );
}