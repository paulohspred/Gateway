import { useQuery } from "@tanstack/react-query";
import { getFleetEvents } from "../api/fleet";
import { Panel } from "../components/Panel";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

export function EventsPage() {
  const query = useQuery({ queryKey: ["fleet-events"], queryFn: getFleetEvents, refetchInterval: 10_000 });
  return <><Topbar title="Eventos" subtitle="Linha temporal factual recebida do Rapid via RC Monitor" onRefresh={() => void query.refetch()} refreshing={query.isFetching}/><div className="content-grid"><Panel title="Eventos recentes"><div className="table-wrap"><table className="data-table"><thead><tr><th>Hora</th><th>Gerador</th><th>Tipo</th><th>Mensagem</th></tr></thead><tbody>{(query.data ?? []).slice(0, 200).map((event) => <tr key={event.id}><td>{formatDateTime(event.occurredAt)}</td><td>{event.generatorId}</td><td>{event.type}</td><td>{event.message}</td></tr>)}</tbody></table></div></Panel></div></>;
}
