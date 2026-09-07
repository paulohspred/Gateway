import type { ReactNode } from "react";

type IconProps = { className?: string; size?: number; color?: string };

function Svg({ size = 16, className = "scada-icon", color, children }: IconProps & { children: ReactNode }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" className={className} aria-hidden style={color ? { color } : undefined}>
    <g fill="none" stroke="currentColor" strokeWidth="1.65" strokeLinejoin="round" strokeLinecap="round">{children}</g>
  </svg>;
}

export function IconOilCan(props: IconProps) { return <Svg {...props}><path d="M3 10h9.3l3.2 3H19v4.2H7.2A4.2 4.2 0 0 1 3 13z"/><path d="M6.5 10V7.2h5.3V10m7.1-.7 2.1-2.2"/><path d="M20.6 12.2c1.4 1.7 1.7 2.3 1.7 3a1.7 1.7 0 0 1-3.4 0c0-.7.4-1.4 1.7-3Z"/></Svg>; }
export function IconThermometer(props: IconProps) { return <Svg {...props}><path d="M9.6 5.3a2.4 2.4 0 0 1 4.8 0v8.4a4.7 4.7 0 1 1-4.8 0z"/><path d="M12 7v8.2"/><path d="M17.2 7.3c1-.8 1.9-.8 2.8 0m-2.8 3.2c1-.8 1.9-.8 2.8 0"/></Svg>; }
export function IconFuelPump(props: IconProps) { return <Svg {...props}><path d="M5 3.5h9.5v17H5z"/><path d="M7 6h5.5v5H7z"/><path d="M14.5 7h2.3l2.1 2.2v7.3a2.1 2.1 0 0 0 2.1 2.1V9.2l-2.1-2.1"/><path d="M5 20.5h10"/></Svg>; }
export function IconBattery(props: IconProps) { return <Svg {...props}><rect x="3.5" y="7" width="17" height="12.5" rx="1.2"/><path d="M8 4.5V7m8-2.5V7M7 13h5m-2.5-2.5v5M14.5 13H18"/></Svg>; }
export function IconBolt(props: IconProps) { return <Svg {...props}><path d="m13.5 2.5-7.2 11.6h5.4l-1.2 7.4 7.2-11.6h-5.3z"/></Svg>; }
export function IconClock(props: IconProps) { return <Svg {...props}><circle cx="12" cy="12" r="9"/><path d="M12 6.7v5.7l4 2.4"/></Svg>; }
export function IconRunHours(props: IconProps) { return <Svg {...props}><circle cx="12" cy="12" r="8.7"/><path d="M12 12 16.2 7.8M4.2 12h2.2m11.2 0h2.2M12 4.2v2.2"/><path d="M7.2 17.1 5.7 18.6m11.1-1.5 1.5 1.5"/></Svg>; }
export function IconHouse({ size = 16, className }: IconProps) { return <svg width={size} height={size} viewBox="0 0 24 24" className={className} aria-hidden><path d="M3.2 11.2 12 3.6l8.8 7.6" fill="none" stroke="currentColor" strokeWidth="2.1" strokeLinejoin="round" strokeLinecap="round"/><path d="M5.4 10.6V20.2h4.4v-5.2h4.4v5.2h4.4V10.6" fill="currentColor" stroke="currentColor" strokeWidth="1.2" strokeLinejoin="round"/></svg>; }
