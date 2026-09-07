import { expect, it } from "vitest";
import { metricSupported, type FleetRow } from "./fleet";
const row = { generator:{id:"g",name:"G",siteId:"s",controller:{manufacturer:"X",model:"Y"}}, telemetry:null, alarms:[], capabilities:{generatorId:"g",profileId:"p",profileStatus:"LAB",telemetry:true,alarms:false,events:false,maintenance:false,remoteControl:false,metrics:[{key:"engine.rpm",displayName:"RPM",kind:"number",required:true,staleAfterSeconds:30}]} } as FleetRow;
it("distinguishes supported from unsupported metric keys",()=>{expect(metricSupported(row,"engine.rpm")).toBe(true);expect(metricSupported(row,"fuel.level")).toBe(false);expect(metricSupported({...row,capabilities:null},"engine.rpm")).toBeUndefined()});
