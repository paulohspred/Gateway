# RC Monitor Frontend

Frontend operacional read-only do RC Monitor.

## Desenvolvimento

```bash
npm install
npm run dev
```

Por padrão o Vite encaminha `/api`, `/healthz` e `/readyz` para `http://127.0.0.1:18100`.
Ajuste `RC_MONITOR_DEV_PROXY` se necessário.

## Gates locais

```bash
npm run typecheck
npm test
npm run build
```

## Segurança funcional

Esta fase não expõe START, STOP, RESET, TEST, transferência, setpoints nem acknowledge.
O frontend só consome endpoints GET do RC Monitor.
