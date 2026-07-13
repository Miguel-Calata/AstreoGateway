import { Outlet, useNavigate } from "react-router-dom";
import { useEffect } from "react";
import { useBootstrapQuery, useSession } from "@/lib/queries";
import { Spinner } from "@/components/ui/spinner";
import { Bootstrap, Login } from "./AuthScreens";
import { Layout } from "@/components/layout/Layout";

export function AuthGate() {
  const bootstrapQ = useBootstrapQuery();
  const sessionQ = useSession();
  const navigate = useNavigate();

  useEffect(() => {
    if (sessionQ.isError && bootstrapQ.data && !bootstrapQ.data.needed) {
      // Logged out (cookie expired) — keep on login screen, no redirect needed.
    }
  }, [sessionQ.isError, bootstrapQ.data, navigate]);

  if (bootstrapQ.isLoading || (sessionQ.isLoading && !bootstrapQ.data?.needed === undefined)) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Spinner className="size-6" />
      </div>
    );
  }

  if (bootstrapQ.data?.needed) return <Bootstrap />;
  if (sessionQ.isError || !sessionQ.data) return <Login />;

  return (
    <Layout>
      <Outlet />
    </Layout>
  );
}