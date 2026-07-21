import { ApiServerSettingsCards } from '../components/ApiServerSettingsCards';
import { restService } from '@services/rest.service';

export function RestApiPage() {
  return (
    <ApiServerSettingsCards
      service={restService}
      serverTitle="REST API server"
      awid="rest-server"
      endpointForPort={(port) => `http://127.0.0.1:${port}/api/aw`}
      description={
        <>
          The same capabilities over plain HTTP: <code>POST /api/aw</code> runs any{' '}
          <code>aw</code> action and <code>GET /api/actions</code> lists them. Access requires
          the bearer token below, passes the Agent Firewall, and works only while the vault is
          unlocked.
        </>
      }
    />
  );
}
