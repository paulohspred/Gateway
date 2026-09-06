import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { AlarmsPage } from "./pages/AlarmsPage";
import { CommunicationPage } from "./pages/CommunicationPage";
import { EventsPage } from "./pages/EventsPage";
import { GeneratorDetailPage } from "./pages/GeneratorDetailPage";
import { GeneratorsPage } from "./pages/GeneratorsPage";
import { OverviewPage } from "./pages/OverviewPage";

export function App() {
  return <Routes><Route element={<AppShell/>}><Route index element={<OverviewPage/>}/><Route path="generators" element={<GeneratorsPage/>}/><Route path="generators/:id" element={<GeneratorDetailPage/>}/><Route path="alarms" element={<AlarmsPage/>}/><Route path="events" element={<EventsPage/>}/><Route path="communication" element={<CommunicationPage/>}/><Route path="*" element={<Navigate to="/" replace/>}/></Route></Routes>;
}
