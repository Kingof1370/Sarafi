import axios from 'axios';

// Stateful client-side mock backend for offline/standalone Render deployment
// without changing the structure of the Next.js frontend pages.
const isClient = typeof window !== 'undefined';

// Local storage state keys
const USERS_KEY = 'velyx_mock_users';
const CURRENT_USER_KEY = 'velyx_mock_current_user';
const ORDERS_KEY = 'velyx_mock_orders';
const BALANCES_KEY = 'velyx_mock_balances';
const SESSIONS_KEY = 'velyx_mock_sessions';
const API_KEYS_KEY = 'velyx_mock_api_keys';
const KYC_KEY = 'velyx_mock_kyc';
const ALERTS_KEY = 'velyx_mock_alerts';
const CASES_KEY = 'velyx_mock_cases';
const BACKUPS_KEY = 'velyx_mock_backups';
const RECOVERY_KEY = 'velyx_mock_recovery';

// Helpers to get/set state
const getState = (key: string, defaultVal: any) => {
  if (!isClient) return defaultVal;
  const val = localStorage.getItem(key);
  return val ? JSON.parse(val) : defaultVal;
};

const setState = (key: string, val: any) => {
  if (isClient) {
    localStorage.setItem(key, JSON.stringify(val));
  }
};

// Initialize default mock data if not present
if (isClient) {
  if (!localStorage.getItem(USERS_KEY)) {
    // Default user
    setState(USERS_KEY, [{ email: 'user@velyxora.com', password: 'password' }]);
  }
  if (!localStorage.getItem(BALANCES_KEY)) {
    setState(BALANCES_KEY, { USDT: 15450.0, BTC: 0.25, ETH: 1.5 });
  }
  if (!localStorage.getItem(SESSIONS_KEY)) {
    setState(SESSIONS_KEY, [
      {
        id: 'sess-1',
        device_id: 'dev-chrome-oregon',
        ip_address: '198.51.100.42',
        user_agent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
        created_at: new Date(Date.now() - 3600000).toISOString(),
        is_current: false
      },
      {
        id: 'sess-current',
        device_id: 'dev-current',
        ip_address: '203.0.113.195',
        user_agent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
        created_at: new Date().toISOString(),
        is_current: true
      }
    ]);
  }
  if (!localStorage.getItem(ORDERS_KEY)) {
    setState(ORDERS_KEY, [
      {
        id: 'ord-101',
        client_order_id: 'cl-001',
        symbol: 'BTC-USDT',
        side: 'BUY',
        type: 'LIMIT',
        price: 50000.0,
        quantity: 0.1,
        filled_quantity: 0.1,
        status: 'FILLED',
        time_in_force: 'GTC',
        created_at: new Date(Date.now() - 7200000).toISOString()
      }
    ]);
  }
  if (!localStorage.getItem(KYC_KEY)) {
    setState(KYC_KEY, {
      user_id: 'usr_logged_in',
      tier: 'STANDARD',
      status: 'VERIFIED',
      provider_ref: 'prov-ref-847291',
      document_metadata: JSON.stringify({ document_type: 'PASSPORT', document_id: 'A1234567' }),
      rejection_reason: '',
      attempts: 1,
      created_at: new Date(Date.now() - 86400000).toISOString(),
      updated_at: new Date(Date.now() - 86400000).toISOString()
    });
  }
  if (!localStorage.getItem(ALERTS_KEY)) {
    setState(ALERTS_KEY, [
      {
        id: 'alt-1',
        user_id: 'usr_logged_in',
        transaction_id: 'tx-84920',
        rule_triggered: 'Large Transaction Velocity Monitor',
        risk_score: 0.72,
        evidence: 'Inbound deposit of 25,000 USDT exceeds standard standard tier profile speed',
        severity: 'MEDIUM',
        status: 'OPEN',
        reviewer: '',
        resolution_reason: '',
        created_at: new Date(Date.now() - 1800000).toISOString(),
        updated_at: new Date(Date.now() - 1800000).toISOString()
      }
    ]);
  }
  if (!localStorage.getItem(CASES_KEY)) {
    setState(CASES_KEY, [
      {
        id: 'case-201',
        user_id: 'usr_logged_in',
        status: 'OPEN',
        investigator_id: 'inv-42',
        resolution: '',
        created_at: new Date(Date.now() - 1800000).toISOString(),
        updated_at: new Date(Date.now() - 1800000).toISOString(),
        notes: [
          { author_id: 'system', note: 'Case auto-opened due to velocity warning alert' }
        ]
      }
    ]);
  }
  if (!localStorage.getItem(BACKUPS_KEY)) {
    setState(BACKUPS_KEY, [
      {
        id: 'bak-992',
        backup_type: 'DATABASE',
        status: 'COMPLETED',
        filepath: '/var/backups/velyxora_db_20260809.enc',
        timestamp: new Date(Date.now() - 1200000).toISOString()
      }
    ]);
  }
  if (!localStorage.getItem(RECOVERY_KEY)) {
    setState(RECOVERY_KEY, [
      {
        id: 'rec-501',
        status: 'VERIFIED',
        details: 'Schema and double-entry balance validations passed successfully.',
        timestamp: new Date(Date.now() - 600000).toISOString()
      }
    ]);
  }
}

// Create a mock client that intercepts calls
const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Mock interceptor
api.interceptors.request.use(async (config) => {
  const url = config.url || '';
  const method = config.method?.toUpperCase() || 'GET';
  const data = config.data;

  // Let's resolve the request with our mock stateful data directly
  // avoiding any network errors.
  let responseData: any = {};
  let status = 200;

  try {
    if (url.includes('/auth/register')) {
      const { email, password } = data;
      const users = getState(USERS_KEY, []);
      if (users.some((u: any) => u.email === email)) {
        status = 400;
        responseData = { error: 'User already exists' };
      } else {
        users.push({ email, password });
        setState(USERS_KEY, users);
        status = 200;
        responseData = { message: 'Registered successfully' };
      }
    } else if (url.includes('/auth/login')) {
      const { email, password } = data;
      const users = getState(USERS_KEY, []);
      const matched = users.find((u: any) => u.email === email && u.password === password);
      if (matched) {
        status = 200;
        responseData = { access_token: 'mock-jwt-token-123456' };
        setState(CURRENT_USER_KEY, email);
      } else {
        status = 401;
        responseData = { error: 'Invalid corporate email or security passphrase' };
      }
    } else if (url.includes('/auth/sessions')) {
      if (method === 'GET') {
        responseData = { sessions: getState(SESSIONS_KEY, []) };
      } else if (method === 'DELETE') {
        const parts = url.split('/');
        const id = parts[parts.length - 1];
        let sessions = getState(SESSIONS_KEY, []);
        sessions = sessions.filter((s: any) => s.id !== id);
        setState(SESSIONS_KEY, sessions);
        responseData = { message: 'Session terminated' };
      }
    } else if (url.includes('/auth/logout-all')) {
      let sessions = getState(SESSIONS_KEY, []);
      sessions = sessions.filter((s: any) => s.is_current);
      setState(SESSIONS_KEY, sessions);
      responseData = { message: 'All other sessions terminated' };
    } else if (url.includes('/apikeys')) {
      if (method === 'GET') {
        responseData = { keys: getState(API_KEYS_KEY, []) };
      } else if (method === 'POST') {
        const { label, permissions, ip_allowlist, expires_in_days } = data;
        const keys = getState(API_KEYS_KEY, []);
        const newKey = {
          id: 'key-' + Math.random().toString(36).substring(2, 9),
          label,
          api_key: 'velyx_pk_' + Math.random().toString(36).substring(2, 15),
          permissions,
          ip_allowlist,
          expires_at: new Date(Date.now() + expires_in_days * 86400000).toISOString(),
          created_at: new Date().toISOString()
        };
        keys.push(newKey);
        setState(API_KEYS_KEY, keys);
        status = 200;
        responseData = {
          api_key: newKey.api_key,
          api_secret: 'velyx_sk_' + Math.random().toString(36).substring(2, 20)
        };
      } else if (method === 'DELETE') {
        const parts = url.split('/');
        const id = parts[parts.length - 1];
        let keys = getState(API_KEYS_KEY, []);
        keys = keys.filter((k: any) => k.id !== id);
        setState(API_KEYS_KEY, keys);
        responseData = { message: 'API key revoked' };
      }
    } else if (url.includes('/mfa/enable')) {
      responseData = {
        mfa_secret: 'KVKVE43VJVJW2Y2T',
        qr_code_url: 'https://api.qrserver.com/v1/create-qr-code/?size=150x150&data=otpauth://totp/Velyxora:user@velyxora.com%3Fsecret%3DKVKVE43VJVJW2Y2T%26issuer%3DVelyxora',
        backup_codes: ['4928-1039', '5820-9482', '2019-3847', '5829-1039']
      };
    } else if (url.includes('/wallet/balances')) {
      responseData = { balances: getState(BALANCES_KEY, { USDT: 15450.0, BTC: 0.25, ETH: 1.5 }) };
    } else if (url.includes('/oms/orders')) {
      if (method === 'POST') {
        const { symbol, side, type, price, quantity, time_in_force } = data;
        const orders = getState(ORDERS_KEY, []);
        const newOrder = {
          id: 'ord-' + Math.floor(100 + Math.random() * 900),
          client_order_id: 'cl-' + Math.floor(100 + Math.random() * 900),
          symbol,
          side,
          type,
          price,
          quantity,
          filled_quantity: type === 'MARKET' ? quantity : 0.0,
          status: type === 'MARKET' ? 'FILLED' : 'OPEN',
          time_in_force,
          created_at: new Date().toISOString()
        };
        orders.push(newOrder);
        setState(ORDERS_KEY, orders);

        // Update balance simulated calculations
        const balances = getState(BALANCES_KEY, {});
        const totalCost = price * quantity;
        if (side === 'BUY') {
          if (balances.USDT >= totalCost) {
            balances.USDT -= totalCost;
            if (type === 'MARKET' || true) { // Auto filled for direct demo usability
              balances.BTC += quantity;
            }
          }
        } else {
          if (balances.BTC >= quantity) {
            balances.BTC -= quantity;
            if (type === 'MARKET' || true) {
              balances.USDT += totalCost;
            }
          }
        }
        setState(BALANCES_KEY, balances);

        responseData = { order: newOrder };
      } else if (method === 'DELETE') {
        const parts = url.split('/');
        const id = parts[parts.length - 1];
        let orders = getState(ORDERS_KEY, []);
        const order = orders.find((o: any) => o.id === id);
        if (order) {
          order.status = 'CANCELLED';
        }
        setState(ORDERS_KEY, orders);
        responseData = { message: 'Order cancelled successfully' };
      } else if (url.includes('/cancel-bulk')) {
        let orders = getState(ORDERS_KEY, []);
        orders.forEach((o: any) => {
          if (o.status === 'OPEN') o.status = 'CANCELLED';
        });
        setState(ORDERS_KEY, orders);
        responseData = { message: 'All open orders cancelled' };
      }
    } else if (url.includes('/oms/open')) {
      const orders = getState(ORDERS_KEY, []);
      responseData = { orders: orders.filter((o: any) => o.status === 'OPEN') };
    } else if (url.includes('/oms/history')) {
      const orders = getState(ORDERS_KEY, []);
      responseData = { orders: orders.filter((o: any) => o.status !== 'OPEN') };
    } else if (url.includes('/oms/trades')) {
      responseData = {
        ticker: {
          last_price: 50000.0 + (Math.random() - 0.5) * 100,
          high_24h: 50200.0,
          low_24h: 49800.0,
          volume_24h: 120.5
        }
      };
    } else if (url.includes('/compliance/kyc/status')) {
      responseData = getState(KYC_KEY, {});
    } else if (url.includes('/compliance/restrictions')) {
      responseData = { restriction_type: 'NORMAL' };
    } else if (url.includes('/compliance/risk')) {
      responseData = {
        id: 'risk-1',
        user_id: 'usr_logged_in',
        score: 0.12,
        risk_level: 'LOW',
        contributing_rules: 'Standard operations. Active corporate verified session. Low multi-asset transaction speed.',
        evaluation_version: '1.2.0',
        created_at: new Date().toISOString()
      };
    } else if (url.includes('/compliance/kyc/submit')) {
      const { tier, doc_meta } = data;
      const kyc = {
        user_id: 'usr_logged_in',
        tier,
        status: 'VERIFIED',
        provider_ref: 'prov-ref-' + Math.floor(100000 + Math.random() * 900000),
        document_metadata: doc_meta,
        rejection_reason: '',
        attempts: 1,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString()
      };
      setState(KYC_KEY, kyc);
      responseData = { profile: kyc };
    } else if (url.includes('/wallet/deposits/mock')) {
      const { asset, amount } = data;
      const balances = getState(BALANCES_KEY, { USDT: 0, BTC: 0, ETH: 0 });
      if (!balances[asset]) balances[asset] = 0;
      balances[asset] += parseFloat(amount);
      setState(BALANCES_KEY, balances);

      responseData = {
        confirmations: 12,
        compliance_status: 'CLEARED',
        details: `Successfully cleared AML screening. Allocated ${amount} ${asset} securely to Ledger balances.`
      };
    } else if (url.includes('/compliance/alerts')) {
      responseData = { alerts: getState(ALERTS_KEY, []) };
    } else if (url.includes('/compliance/cases')) {
      if (method === 'GET') {
        const parts = url.split('/');
        if (parts[parts.length - 2] === 'cases') {
          const id = parts[parts.length - 1];
          const cases = getState(CASES_KEY, []);
          const cs = cases.find((c: any) => c.id === id);
          responseData = { case: cs, notes: cs ? cs.notes : [] };
        } else {
          responseData = { cases: getState(CASES_KEY, []) };
        }
      } else if (method === 'PATCH') {
        const parts = url.split('/');
        const id = parts[parts.length - 1];
        const { status: newStatus, resolution, note } = data;
        const cases = getState(CASES_KEY, []);
        const cs = cases.find((c: any) => c.id === id);
        if (cs) {
          cs.status = newStatus;
          cs.resolution = resolution;
          if (note) {
            cs.notes.push({ author_id: 'investigator', note });
          }
        }
        setState(CASES_KEY, cases);
        responseData = { message: 'Case updated' };
      }
    } else if (url.includes('/oms/system/backups')) {
      responseData = { backups: getState(BACKUPS_KEY, []) };
    } else if (url.includes('/oms/system/recovery')) {
      responseData = { recovery_runs: getState(RECOVERY_KEY, []) };
    } else if (url.includes('/oms/system/backup')) {
      const backups = getState(BACKUPS_KEY, []);
      const newBk = {
        id: 'bak-' + Math.floor(100 + Math.random() * 900),
        backup_type: 'DATABASE',
        status: 'COMPLETED',
        filepath: `/var/backups/velyxora_db_${new Date().toISOString().slice(0, 10).replace(/-/g, '')}.enc`,
        timestamp: new Date().toISOString()
      };
      backups.unshift(newBk);
      setState(BACKUPS_KEY, backups);
      responseData = newBk;
    } else if (url.includes('/oms/system/recover')) {
      const runs = getState(RECOVERY_KEY, []);
      const newRun = {
        id: 'rec-' + Math.floor(100 + Math.random() * 900),
        status: 'VERIFIED',
        details: 'Schema validation completed. Zero financial transaction discrepancies found.',
        timestamp: new Date().toISOString()
      };
      runs.unshift(newRun);
      setState(RECOVERY_KEY, runs);
      responseData = { replayed: 1, details: { ledger_balanced: true } };
    } else {
      // Fallback
      responseData = {};
    }

    // Force axios request handler to bypass actual network and return the mocked response directly
    config.adapter = () => {
      return Promise.resolve({
        data: responseData,
        status: status,
        statusText: status === 200 ? 'OK' : 'Error',
        headers: {},
        config: config,
      });
    };
  } catch (err: any) {
    console.error('Interceptor exception:', err);
  }

  return config;
});

api.interceptors.request.use((config) => {
  if (typeof window !== 'undefined') {
    const token = localStorage.getItem('access_token');
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`;
    }
  }
  return config;
});

export default api;
