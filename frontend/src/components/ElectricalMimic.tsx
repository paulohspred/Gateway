import type { Telemetry } from "../api/schemas";
import { GeneratorEnergyFlow } from "./GeneratorEnergyFlow";

export function ElectricalMimic({ telemetry }: { telemetry: Telemetry }) {
  return <div className="detail-energy-flow"><GeneratorEnergyFlow telemetry={telemetry}/><div className="detail-flow-note"><strong>Sinótico read-only</strong><span>Estados elétricos são derivados somente de estados explícitos da controladora e dos disjuntores. Tensão, RPM e kW não fecham contato nem energizam fonte por inferência.</span></div></div>;
}
