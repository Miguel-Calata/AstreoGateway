import { Route, Routes } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthGate } from "@/features/auth/AuthGate";
import { Overview } from "@/features/overview/Overview";
import { ProvidersList } from "@/features/providers/ProvidersList";
import { ProviderDetail } from "@/features/providers/ProviderDetail";
import { AliasesList } from "@/features/aliases/AliasesList";
import { GatewayKeysList } from "@/features/gateway-keys/GatewayKeysList";
import { DiscoveryBoard } from "@/features/discovery/DiscoveryBoard";
import { LogsDashboard } from "@/features/logs/LogsDashboard";
import { LogsList } from "@/features/logs/LogsList";

export default function App() {
  return (
    <TooltipProvider delayDuration={150}>
      <Routes>
        <Route element={<AuthGate />}>
          <Route path="/" element={<Overview />} />
          <Route path="/providers" element={<ProvidersList />} />
          <Route path="/providers/:id" element={<ProviderDetail />} />
          <Route path="/aliases" element={<AliasesList />} />
          <Route path="/gateway-keys" element={<GatewayKeysList />} />
          <Route path="/discovery" element={<DiscoveryBoard />} />
          <Route path="/logs" element={<LogsDashboard />} />
          <Route path="/logs/requests" element={<LogsList />} />
        </Route>
      </Routes>
    </TooltipProvider>
  );
}