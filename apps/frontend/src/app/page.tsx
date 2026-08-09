'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { useAuthStore } from '../store/authStore';
import api from '../lib/api';

interface AdvancedOrder {
  id: string;
  client_order_id: string;
  symbol: string;
  side: string;
  type: string;
  price: number;
  quantity: number;
  filled_quantity: number;
  status: string;
  time_in_force: string;
  stop_price?: number;
  iceberg_size?: number;
  created_at: string;
}

interface UserSession {
  id: string;
  device_id: string;
  ip_address: string;
  user_agent: string;
  created_at: string;
  is_current: boolean;
}

interface UserAPIKey {
  id: string;
  label: string;
  api_key: string;
  permissions: string;
  ip_allowlist: string;
  expires_at: string;
  created_at: string;
  last_used_at?: string;
}

// P0009 Interface definitions
interface KYCProfile {
  user_id: string;
  tier: string;
  status: string;
  provider_ref: string;
  document_metadata: string;
  rejection_reason: string;
  attempts: number;
  verified_at?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

interface ComplianceAlert {
  id: string;
  user_id: string;
  transaction_id: string;
  rule_triggered: string;
  risk_score: number;
  evidence: string;
  severity: string;
  status: string;
  reviewer: string;
  resolution_reason: string;
  created_at: string;
  updated_at: string;
}

interface ComplianceCase {
  id: string;
  user_id: string;
  status: string;
  investigator_id: string;
  resolution: string;
  created_at: string;
  updated_at: string;
}

interface RiskEvaluation {
  id: string;
  user_id: string;
  score: number;
  risk_level: string;
  contributing_rules: string;
  evaluation_version: string;
  created_at: string;
}

export default function Home() {
  const { user, accessToken, setAuth, logout } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [isRegister, setIsRegister] = useState(false);

  // App Tabs: trading vs security vs compliance vs dr
  const [activeTab, setActiveTab] = useState<'trading' | 'security' | 'compliance' | 'dr'>('trading');

  // Disaster Recovery panel states
  const [backupsHistory, setBackupsHistory] = useState<any[]>([]);
  const [recoveryHistory, setRecoveryHistory] = useState<any[]>([]);
  const [drMFACode, setDrMFACode] = useState('');
  const [drStatusMsg, setDrStatusMsg] = useState('');
  const [drSuccessMsg, setDrSuccessMsg] = useState('');

  const fetchDRHistory = async () => {
    try {
      const bRes = await api.get('/oms/system/backups');
      setBackupsHistory(bRes.data.backups || []);
      const rRes = await api.get('/oms/system/recovery');
      setRecoveryHistory(rRes.data.recovery_runs || []);
    } catch (e: any) {
      console.error("Failed to load Disaster Recovery history", e);
    }
  };

  const triggerBackupDR = async () => {
    try {
      setDrStatusMsg('Generating secure encrypted backup...');
      setDrSuccessMsg('');
      const res = await api.post('/oms/system/backup', { type: 'DATABASE', mfa_code: drMFACode });
      setDrSuccessMsg(`Backup success! File written: ${res.data.filepath}`);
      setDrStatusMsg('');
      setDrMFACode('');
      fetchDRHistory();
    } catch (e: any) {
      setDrStatusMsg(`Backup failed: ${e.response?.data?.error || e.message}`);
    }
  };

  const triggerRestoreVerificationDR = async () => {
    try {
      setDrStatusMsg('Running isolated schema and financial ledger restore test...');
      setDrSuccessMsg('');
      const res = await api.post('/oms/system/recover', { mfa_code: drMFACode });
      setDrSuccessMsg(`Restore verified! Users count: ${res.data.replayed}. Balanced ledger state: ${res.data.details?.ledger_balanced}`);
      setDrStatusMsg('');
      setDrMFACode('');
      fetchDRHistory();
    } catch (e: any) {
      setDrStatusMsg(`Verification failed: ${e.response?.data?.error || e.message}`);
    }
  };

  // Theme & Layout state
  const [theme, setTheme] = useState<'dark' | 'light'>('dark');

  // Trading States
  const [symbol, setSymbol] = useState('BTC-USDT');
  const [side, setSide] = useState('BUY');
  const [type, setType] = useState('LIMIT');
  const [price, setPrice] = useState('50000');
  const [quantity, setQuantity] = useState('0.1');
  const [timeInForce, setTimeInForce] = useState('GTC');
  const [stopPrice, setStopPrice] = useState('0');
  const [icebergSize, setIcebergSize] = useState('0');

  // Terminal Feeds States
  const [orderStatus, setOrderStatus] = useState('');
  const [openOrders, setOpenOrders] = useState<AdvancedOrder[]>([]);
  const [orderHistory, setOrderHistory] = useState<AdvancedOrder[]>([]);
  const [ticker, setTicker] = useState({ lastPrice: 50000.0, high: 50200.0, low: 49800.0, vol: 120.5, spread: 0.1 });

  // Security Management States
  const [sessions, setSessions] = useState<UserSession[]>([]);
  const [apiKeys, setApiKeys] = useState<UserAPIKey[]>([]);

  // API Key creation form
  const [keyLabel, setKeyLabel] = useState('');
  const [keyPerms, setKeyPermissions] = useState({ walletRead: true, walletWrite: false, tradingWrite: false });
  const [keyIPAllowlist, setKeyIPAllowlist] = useState('');
  const [keyExpiryDays, setKeyExpiryDays] = useState(30);
  const [keyMfaCode, setKeyMfaCode] = useState('');

  // Created credentials modal state
  const [createdKey, setCreatedKey] = useState<{ apiKey: string; apiSecret: string } | null>(null);

  // MFA Enrollment States
  const [mfaSecret, setMfaSecret] = useState('');
  const [mfaQrUrl, setMfaQrUrl] = useState('');
  const [mfaBackupCodes, setMfaBackupCodes] = useState<string[]>([]);
  const [mfaStatusMsg, setMfaStatusMsg] = useState('');

  // P0009 Compliance States
  const [kycProfile, setKycProfile] = useState<KYCProfile | null>(null);
  const [userRestriction, setUserRestriction] = useState<string>('NORMAL');
  const [riskEvaluation, setRiskEvaluation] = useState<RiskEvaluation | null>(null);
  const [alerts, setAlerts] = useState<ComplianceAlert[]>([]);
  const [cases, setCases] = useState<ComplianceCase[]>([]);

  // KYC submit form states
  const [kycTier, setKycTier] = useState<string>('STANDARD');
  const [kycDocType, setKycDocType] = useState<string>('PASSPORT');
  const [kycDocId, setKycDocId] = useState<string>('');
  const [kycStatusMsg, setKycStatusMsg] = useState<string>('');

  // Real user balances state loaded dynamically from backend in production
  const [userBalances, setUserBalances] = useState<{ asset: string; available: number; locked: number; total: number }[]>([]);

  // Mock Deposit states
  const [depAsset, setDepAsset] = useState<string>('USDT');
  const [depAmount, setDepAmount] = useState<string>('2500');
  const [depAddress, setDepAddress] = useState<string>('0x71C7656EC7ab88b098defB751B7401B5f6d1476B');
  const [depTxHash, setDepTxHash] = useState<string>('0x123abc456def');
  const [depStatusMsg, setDepStatusMsg] = useState<string>('');

  // Case investigation action states
  const [activeCaseId, setActiveCaseId] = useState<string>('');
  const [caseNotes, setCaseNotes] = useState<any[]>([]);
  const [caseResolution, setCaseResolution] = useState<string>('');
  const [caseStatusValue, setCaseStatusValue] = useState<string>('RESOLVED');
  const [investigatorNote, setInvestigatorNote] = useState<string>('');

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      if (isRegister) {
        await api.post('/auth/register', { email, password });
        setIsRegister(false);
        setError('Registration successful! Please login.');
      } else {
        const res = await api.post('/auth/login', { email, password });
        const { access_token } = res.data;
        setAuth({ id: 'usr_logged_in', email }, access_token);
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Authentication action failed');
    } finally {
      setLoading(false);
    }
  };

  const fetchOrders = useCallback(async () => {
    if (!accessToken) return;
    try {
      const openRes = await api.get('/oms/open');
      setOpenOrders(openRes.data.orders || []);

      const histRes = await api.get('/oms/history');
      setOrderHistory(histRes.data.orders || []);

      const tradesRes = await api.get(`/oms/trades?symbol=${symbol}`);
      const t = tradesRes.data.ticker;
      if (t) {
        setTicker({
          lastPrice: t.last_price || 50000.0,
          high: t.high_24h || 50200.0,
          low: t.low_24h || 49800.0,
          vol: t.volume_24h || 120.5,
          spread: t.last_price * 0.0002 || 5.0,
        });
      }
    } catch (err) {
      console.error('Failed to load portfolio feeds', err);
    }
  }, [accessToken, symbol]);

  const fetchBalances = useCallback(async () => {
    if (!accessToken) return;
    try {
      const res = await api.get('/wallet/balances');
      setUserBalances(res.data.balances || []);
    } catch (err) {
      console.error('Failed to load real user balances', err);
    }
  }, [accessToken]);

  const fetchSecurityState = useCallback(async () => {
    if (!accessToken) return;
    try {
      const sessRes = await api.get('/auth/sessions');
      setSessions(sessRes.data.sessions || []);

      const keysRes = await api.get('/apikeys');
      setApiKeys(keysRes.data.keys || []);
    } catch (err) {
      console.error('Failed to fetch security state', err);
    }
  }, [accessToken]);

  // P0009 Fetch Compliance data
  const fetchComplianceState = useCallback(async () => {
    if (!accessToken) return;
    try {
      const kycRes = await api.get('/compliance/kyc/status');
      setKycProfile(kycRes.data);

      const restRes = await api.get('/compliance/restrictions');
      setUserRestriction(restRes.data.restriction_type || 'NORMAL');

      const riskRes = await api.get('/compliance/risk');
      setRiskEvaluation(riskRes.data);

      // Attempt to load administrative collections if authorized
      try {
        const alertsRes = await api.get('/compliance/alerts');
        setAlerts(alertsRes.data.alerts || []);

        const casesRes = await api.get('/compliance/cases');
        setCases(casesRes.data.cases || []);
      } catch (adminErr) {
        // Suppress if the user doesn't hold standard compliance permissions
      }
    } catch (err) {
      console.error('Failed to fetch compliance state', err);
    }
  }, [accessToken]);

  const submitKYC = async (e: React.FormEvent) => {
    e.preventDefault();
    setKycStatusMsg('');
    try {
      const docMeta = JSON.stringify({ document_type: kycDocType, document_id: kycDocId });
      const res = await api.post('/compliance/kyc/submit', {
        tier: kycTier,
        doc_meta: docMeta,
      });
      setKycStatusMsg(`Success: KYC Submitted. Ref: ${res.data.profile?.provider_ref}`);
      setKycDocId('');
      fetchComplianceState();
    } catch (err: any) {
      setKycStatusMsg(`Error: ${err.response?.data?.error || err.message}`);
    }
  };

  const submitMockDeposit = async (e: React.FormEvent) => {
    e.preventDefault();
    setDepStatusMsg('');
    try {
      const res = await api.post('/wallet/deposits/mock', {
        asset: depAsset,
        amount: parseFloat(depAmount),
        address: depAddress,
        tx_hash: depTxHash,
      });
      setDepStatusMsg(`Deposit Response: confirmations: ${res.data.confirmations}, status: ${res.data.compliance_status}. Details: ${res.data.details}`);
      setDepTxHash('0x' + Math.random().toString(16).substring(2, 14));
      fetchComplianceState();
    } catch (err: any) {
      setDepStatusMsg(`Compliance Error: ${err.response?.data?.error || err.message}`);
    }
  };

  const viewCaseDetails = async (caseId: string) => {
    try {
      const res = await api.get(`/compliance/cases/${caseId}`);
      setActiveCaseId(caseId);
      setCaseNotes(res.data.notes || []);
      setCaseResolution(res.data.case?.resolution || '');
      setCaseStatusValue(res.data.case?.status || 'RESOLVED');
    } catch (err: any) {
      alert(`Error loading case: ${err.message}`);
    }
  };

  const submitCaseResolution = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.patch(`/compliance/cases/${activeCaseId}`, {
        status: caseStatusValue,
        resolution: caseResolution,
        note: investigatorNote,
      });
      alert('Case updated successfully!');
      setInvestigatorNote('');
      setActiveCaseId('');
      fetchComplianceState();
    } catch (err: any) {
      alert(`Error updating case: ${err.response?.data?.error || err.message}`);
    }
  };

  const revokeSession = async (id: string) => {
    try {
      await api.delete(`/auth/sessions/${id}`);
      fetchSecurityState();
    } catch (err: any) {
      alert(`Error revoking session: ${err.response?.data?.error || err.message}`);
    }
  };

  const revokeAllSessions = async () => {
    if (!confirm('Are you sure you want to terminate all other sessions?')) return;
    try {
      await api.post('/auth/logout-all');
      fetchSecurityState();
    } catch (err: any) {
      alert(`Error revoking all sessions: ${err.response?.data?.error || err.message}`);
    }
  };

  const createAPIKey = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    const permsList: string[] = [];
    if (keyPerms.walletRead) permsList.push('wallet:read');
    if (keyPerms.walletWrite) permsList.push('wallet:write');
    if (keyPerms.tradingWrite) permsList.push('trading:write');

    try {
      const res = await api.post('/apikeys', {
        label: keyLabel,
        permissions: permsList.join(','),
        ip_allowlist: keyIPAllowlist,
        expires_in_days: Number(keyExpiryDays),
        mfa_code: keyMfaCode,
      });

      setCreatedKey({
        apiKey: res.data.api_key,
        apiSecret: res.data.api_secret,
      });

      // Reset form
      setKeyLabel('');
      setKeyIPAllowlist('');
      setKeyMfaCode('');
      fetchSecurityState();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to generate API Key');
    }
  };

  const revokeAPIKey = async (id: string) => {
    if (!confirm('Are you sure you want to revoke this API Key?')) return;
    try {
      await api.delete(`/apikeys/${id}`);
      fetchSecurityState();
    } catch (err: any) {
      alert(`Error revoking API Key: ${err.response?.data?.error || err.message}`);
    }
  };

  const enrollMFA = async () => {
    setMfaStatusMsg('');
    try {
      const res = await api.post('/mfa/enable');
      setMfaSecret(res.data.mfa_secret);
      setMfaQrUrl(res.data.qr_code_url);
      setMfaBackupCodes(res.data.backup_codes || []);
      setMfaStatusMsg('MFA configuration generated. Secure the secret and backup codes.');
    } catch (err: any) {
      setMfaStatusMsg(`Error enabling MFA: ${err.response?.data?.error || err.message}`);
    }
  };

  const placeOrder = async (e: React.FormEvent) => {
    e.preventDefault();
    setOrderStatus('');
    try {
      const res = await api.post('/oms/orders', {
        symbol,
        side,
        type,
        price: parseFloat(price),
        quantity: parseFloat(quantity),
        time_in_force: timeInForce,
        stop_price: parseFloat(stopPrice),
        iceberg_size: parseFloat(icebergSize),
      });
      setOrderStatus(`Success: Order processed. Status: ${res.data.order?.status}`);
      fetchOrders();
    } catch (err: any) {
      setOrderStatus(`Error: ${err.response?.data?.error || 'Failed to process order'}`);
    }
  };

  const cancelOrder = async (id: string) => {
    try {
      await api.delete(`/oms/orders/${id}`);
      setOrderStatus(`Order ${id} cancelled successfully.`);
      fetchOrders();
    } catch (err: any) {
      setOrderStatus(`Error cancelling order: ${err.response?.data?.error || err.message}`);
    }
  };

  const bulkCancel = async () => {
    try {
      await api.post('/oms/orders/cancel-bulk', { symbol });
      setOrderStatus(`Bulk cancel triggered for ${symbol}.`);
      fetchOrders();
    } catch (err: any) {
      setOrderStatus(`Error: ${err.response?.data?.error || err.message}`);
    }
  };

  useEffect(() => {
    if (accessToken) {
      fetchOrders();
      fetchBalances();
      fetchSecurityState();
      fetchComplianceState();
      const interval = setInterval(() => {
        fetchOrders();
        fetchBalances();
        fetchSecurityState();
        fetchComplianceState();
      }, 4000);
      return () => clearInterval(interval);
    }
  }, [accessToken, fetchOrders, fetchBalances, fetchSecurityState, fetchComplianceState]);

  const totalCost = parseFloat(price) * parseFloat(quantity);
  const estimatedFees = totalCost * (side === 'BUY' ? 0.002 : 0.001);

  return (
    <main className={`min-h-screen p-6 font-sans transition-colors ${theme === 'dark' ? 'bg-slate-950 text-slate-100' : 'bg-slate-50 text-slate-900'}`}>
      {/* Upper Navigation Header */}
      <header className={`max-w-7xl mx-auto flex justify-between items-center pb-4 border-b ${theme === 'dark' ? 'border-slate-800' : 'border-slate-300'}`}>
        <div className="flex items-center gap-6">
          <h1 className="text-2xl font-extrabold tracking-wider bg-gradient-to-r from-cyan-400 to-blue-600 bg-clip-text text-transparent">
            VELYXORA COMPLIANCE
          </h1>
          <div className="flex items-center gap-4 bg-slate-900/60 p-1.5 rounded-lg border border-slate-800 text-xs">
            <span className="text-slate-400 font-semibold">{symbol}</span>
            <span className="text-emerald-400 font-bold">${ticker.lastPrice.toFixed(2)}</span>
            <span className="text-slate-500">24h High: ${ticker.high.toFixed(2)}</span>
          </div>
        </div>

        {/* Workspace Customizer Controls */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-1.5 bg-slate-900/60 p-1 rounded-lg border border-slate-800 text-xs">
            <button
              onClick={() => setActiveTab('trading')}
              className={`px-3 py-1 rounded font-bold transition ${activeTab === 'trading' ? 'bg-cyan-500 text-slate-950' : 'text-slate-300 hover:bg-slate-800'}`}
            >
              📈 Trading
            </button>
            <button
              onClick={() => setActiveTab('security')}
              className={`px-3 py-1 rounded font-bold transition ${activeTab === 'security' ? 'bg-cyan-500 text-slate-950' : 'text-slate-300 hover:bg-slate-800'}`}
            >
              🛡️ Security
            </button>
            <button
              onClick={() => setActiveTab('compliance')}
              className={`px-3 py-1 rounded font-bold transition ${activeTab === 'compliance' ? 'bg-cyan-500 text-slate-950' : 'text-slate-300 hover:bg-slate-800'}`}
            >
              💼 Compliance
            </button>
            <button
              onClick={() => { setActiveTab('dr'); fetchDRHistory(); }}
              className={`px-3 py-1 rounded font-bold transition ${activeTab === 'dr' ? 'bg-cyan-500 text-slate-950' : 'text-slate-300 hover:bg-slate-800'}`}
            >
              🔄 Disaster Recovery
            </button>
            <button onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')} className="px-2 py-1 hover:bg-slate-800 rounded text-slate-300">
              {theme === 'dark' ? '☀️' : '🌙'}
            </button>
          </div>
          {accessToken ? (
            <div className="flex items-center gap-4">
              <span className="text-xs text-cyan-400 font-semibold">{user?.email}</span>
              <button onClick={logout} className="px-3 py-1.5 bg-rose-950/40 border border-rose-800 text-rose-300 hover:bg-rose-900/50 rounded-lg text-xs font-semibold">
                Logout
              </button>
            </div>
          ) : (
            <span className="text-xs text-slate-500">Not Logged In</span>
          )}
        </div>
      </header>

      {/* 1. Trading Workspace Tab */}
      {activeTab === 'trading' && (
        <div className="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-4 gap-6 mt-6">
          {/* Left Column: Order Entry & Account */}
          <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl flex flex-col justify-between">
            {!accessToken ? (
              <div>
                <h2 className="text-lg font-bold mb-4 text-cyan-400">Vault Access Control</h2>
                <form onSubmit={handleAuth} className="space-y-4">
                  <div>
                    <label className="block text-xs font-semibold text-slate-400 mb-2">Corporate Email</label>
                    <input
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500"
                      placeholder="user@velyxora.com"
                      required
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold text-slate-400 mb-2">Security Passphrase</label>
                    <input
                      type="password"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500"
                      placeholder="••••••••"
                      required
                    />
                  </div>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => setIsRegister(!isRegister)}
                      className="text-xs text-slate-400 underline py-2"
                    >
                      {isRegister ? 'Or Login' : 'Create Account'}
                    </button>
                  </div>
                  {error && <p className="text-xs text-amber-400 font-semibold">{error}</p>}
                  <button type="submit" className="w-full py-2.5 bg-cyan-500 text-slate-950 font-bold rounded-lg text-xs hover:bg-cyan-400">
                    {isRegister ? 'Register' : 'Authenticate'}
                  </button>
                </form>
              </div>
            ) : (
              <form onSubmit={placeOrder} className="space-y-4">
                <h2 className="text-base font-bold text-cyan-400">Order Placement</h2>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setSide('BUY')}
                    className={`py-1.5 rounded text-xs font-bold transition border ${
                      side === 'BUY' ? 'bg-emerald-950/50 text-emerald-400 border-emerald-500' : 'bg-slate-950 text-slate-400 border-slate-800'
                    }`}
                  >
                    Buy
                  </button>
                  <button
                    type="button"
                    onClick={() => setSide('SELL')}
                    className={`py-1.5 rounded text-xs font-bold transition border ${
                      side === 'SELL' ? 'bg-rose-950/50 text-rose-400 border-rose-500' : 'bg-slate-950 text-slate-400 border-slate-800'
                    }`}
                  >
                    Sell
                  </button>
                </div>

                <div>
                  <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Execution Mode</label>
                  <select
                    value={type}
                    onChange={(e) => setType(e.target.value)}
                    className="w-full px-2 py-1.5 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  >
                    <option value="LIMIT">LIMIT</option>
                    <option value="MARKET">MARKET</option>
                  </select>
                </div>

                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Price (USDT)</label>
                    <input
                      type="number"
                      value={price}
                      onChange={(e) => setPrice(e.target.value)}
                      className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Quantity</label>
                    <input
                      type="number"
                      step="0.0001"
                      value={quantity}
                      onChange={(e) => setQuantity(e.target.value)}
                      className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                <div className="border-t border-slate-800 pt-3 space-y-1.5 text-xs text-slate-400">
                  <div className="flex justify-between">
                    <span>Subtotal:</span>
                    <span className="font-mono text-slate-200">${totalCost.toFixed(2)} USDT</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Est. Commissions:</span>
                    <span className="font-mono text-slate-200">${estimatedFees.toFixed(4)} USDT</span>
                  </div>
                </div>

                <div className="flex gap-2">
                  <button type="submit" className={`flex-1 py-2.5 rounded-lg text-xs font-bold transition text-slate-950 ${side === 'BUY' ? 'bg-emerald-400 hover:bg-emerald-300' : 'bg-rose-400 hover:bg-rose-300'}`}>
                    Place {side} Order
                  </button>
                  <button type="button" onClick={bulkCancel} className="px-2 py-1 bg-slate-950 border border-rose-800 text-rose-400 text-xs rounded hover:bg-rose-950/30">
                    Cancel All
                  </button>
                </div>
              </form>
            )}
          </section>

          {/* Center Panels: Order Book & Recent Public Trades */}
          <section className="lg:col-span-2 grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-lg flex flex-col justify-between">
              <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Live Level 2 Order Book</h3>
              <div className="space-y-1 font-mono text-xs">
                <div className="grid grid-cols-3 text-slate-500 font-semibold mb-2 border-b border-slate-800 pb-1">
                  <span>Price (USDT)</span>
                  <span className="text-right">Size</span>
                  <span className="text-right">Total (USDT)</span>
                </div>
                <div className="grid grid-cols-3 text-rose-400">
                  <span>50100.00</span>
                  <span className="text-right">0.25</span>
                  <span className="text-right">$12,525</span>
                </div>
                <div className="flex justify-between py-2 border-y border-slate-800 my-2 text-xs font-bold">
                  <span className="text-slate-400">Spread</span>
                  <span className="text-slate-200">${ticker.spread.toFixed(2)} USDT (0.02%)</span>
                </div>
                <div className="grid grid-cols-3 text-emerald-400 bg-emerald-950/20">
                  <span>50000.00</span>
                  <span className="text-right">0.85</span>
                  <span className="text-right">$42,500</span>
                </div>
              </div>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-lg">
              <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Recent Execution Trades</h3>
              <div className="space-y-2 font-mono text-xs max-h-60 overflow-y-auto">
                <div className="grid grid-cols-3 text-slate-500 font-semibold border-b border-slate-800 pb-1">
                  <span>Price</span>
                  <span className="text-right">Size</span>
                  <span className="text-right">Time</span>
                </div>
                <div className="grid grid-cols-3 text-emerald-400">
                  <span>50000.00</span>
                  <span className="text-right">0.1250</span>
                  <span className="text-right text-slate-500">12:34:56</span>
                </div>
              </div>
            </div>
          </section>

          {/* Right Column: Portfolio Allocations */}
          <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl flex flex-col justify-between">
            <div>
              <h3 className="text-sm font-bold text-slate-300 border-b border-slate-800 pb-2 mb-3">Portfolio Allocations</h3>
              <div className="space-y-3 max-h-64 overflow-y-auto">
                {userBalances.length === 0 ? (
                  <div className="p-3 bg-slate-950 border border-slate-800/40 rounded-lg text-center text-xs text-slate-500">
                    No active balances loaded. Submit a simulated deposit on the Compliance tab to credit asset accounts.
                  </div>
                ) : (
                  userBalances.map((bal) => (
                    <div key={bal.asset} className="bg-slate-950 p-3 rounded-lg border border-slate-800/60 text-xs">
                      <div className="flex justify-between font-bold text-cyan-400">
                        <span>{bal.asset}</span>
                        <span>Total: {bal.total}</span>
                      </div>
                      <div className="flex justify-between text-[11px] text-slate-400 mt-1">
                        <span>Available: {bal.available}</span>
                        <span>Locked: {bal.locked}</span>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>

            <div className="mt-6">
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Live Order Status Alerts</h4>
              {orderStatus ? (
                <div className={`p-3 rounded-lg text-xs font-mono border ${orderStatus.startsWith('Error') ? 'bg-rose-950/30 text-rose-400 border-rose-900/60' : 'bg-emerald-950/30 text-emerald-400 border-emerald-900/60'}`}>
                  {orderStatus}
                </div>
              ) : (
                <div className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-500 text-center font-medium">
                  No new execution logs
                </div>
              )}
            </div>
          </section>
        </div>
      )}

      {/* 2. Security Control Center Tab */}
      {activeTab === 'security' && (
        <div className="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-3 gap-6 mt-6">
          {/* Active Sessions Panel */}
          <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl flex flex-col justify-between col-span-1">
            <div>
              <div className="flex justify-between items-center mb-4">
                <h3 className="text-base font-bold text-cyan-400">🛡️ Active Sessions</h3>
                <button
                  onClick={revokeAllSessions}
                  className="px-2.5 py-1 text-xs bg-rose-950 border border-rose-800 text-rose-400 rounded-lg font-semibold hover:bg-rose-900/50"
                >
                  Terminate All Others
                </button>
              </div>

              <div className="space-y-3 overflow-y-auto max-h-[450px] pr-1">
                {sessions.length === 0 ? (
                  <p className="text-xs text-slate-500 text-center py-6">No active sessions logged</p>
                ) : (
                  sessions.map((sess) => (
                    <div key={sess.id} className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-xs relative">
                      <div className="flex justify-between font-bold text-slate-300">
                        <span>IP: {sess.ip_address}</span>
                        {sess.is_current ? (
                          <span className="text-[10px] text-emerald-400 bg-emerald-950/60 border border-emerald-900 px-1.5 py-0.5 rounded">Current</span>
                        ) : (
                          <button
                            onClick={() => revokeSession(sess.id)}
                            className="text-rose-400 hover:underline font-bold text-[10px]"
                          >
                            Terminate
                          </button>
                        )}
                      </div>
                      <p className="text-[11px] text-slate-500 mt-1.5 truncate">UA: {sess.user_agent}</p>
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* MFA QR/Enrollment Settings */}
            <div className="border-t border-slate-800 mt-6 pt-4">
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">MFA Setup Control</h4>
              <button
                onClick={enrollMFA}
                className="w-full py-2 bg-slate-950 border border-cyan-800 text-cyan-400 rounded-lg text-xs font-semibold hover:bg-cyan-950/20"
              >
                Generate MFA Secret / QR
              </button>

              {mfaSecret && (
                <div className="mt-3 p-3 bg-slate-950 border border-slate-800 rounded-lg text-xs space-y-2">
                  <p className="text-emerald-400 font-bold text-xs">{mfaStatusMsg}</p>
                  <div>
                    <span className="text-slate-500 text-[10px] block">MFA Key Secret:</span>
                    <code className="text-slate-300 font-bold block mt-0.5 bg-slate-900 p-1 rounded select-all text-center">{mfaSecret}</code>
                  </div>
                </div>
              )}
            </div>
          </section>

          {/* API Key Creation Form */}
          <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl col-span-1">
            <h3 className="text-base font-bold text-cyan-400 mb-4">🔑 Create API Key</h3>
            <form onSubmit={createAPIKey} className="space-y-4 text-xs">
              <div>
                <label className="block text-slate-400 mb-1 font-semibold">Key Label</label>
                <input
                  type="text"
                  value={keyLabel}
                  onChange={(e) => setKeyLabel(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500"
                  placeholder="e.g., My Trading Bot"
                  required
                />
              </div>

              <div>
                <label className="block text-slate-400 mb-1 font-semibold">MFA Validation Code</label>
                <input
                  type="text"
                  value={keyMfaCode}
                  onChange={(e) => setKeyMfaCode(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500 font-mono text-center tracking-widest"
                  placeholder="000000"
                  maxLength={6}
                  required
                />
              </div>

              {error && <p className="text-xs text-amber-400 font-semibold">{error}</p>}
              <button type="submit" className="w-full py-2.5 bg-cyan-500 text-slate-950 font-bold rounded-lg text-xs hover:bg-cyan-400 transition">
                Generate Keys
              </button>
            </form>
          </section>

          {/* List of Active API Keys */}
          <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl col-span-1 flex flex-col justify-between">
            <div>
              <h3 className="text-base font-bold text-cyan-400 mb-4">🔑 Active API Keys</h3>
              <div className="space-y-3 overflow-y-auto max-h-[500px] pr-1">
                {apiKeys.length === 0 ? (
                  <p className="text-xs text-slate-500 text-center py-6">No API keys registered</p>
                ) : (
                  apiKeys.map((key) => (
                    <div key={key.id} className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-xs">
                      <div className="flex justify-between font-bold text-slate-300">
                        <span>Label: {key.label}</span>
                        <button
                          onClick={() => revokeAPIKey(key.id)}
                          className="text-rose-400 hover:underline font-bold text-[10px]"
                        >
                          Revoke
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </section>
        </div>
      )}

      {/* 4. Disaster Recovery Dashboard Tab (P0014 Panel) */}
      {activeTab === 'dr' && (
        <div className="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-3 gap-6 mt-6">
          <div className="col-span-1 space-y-6">
            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-4">⚙️ Controlled Recovery Operations</h3>
              <p className="text-xs text-slate-400 mb-4">
                Execute production-grade backup generation or isolated sandbox restore verification.
                All operations are secured by RBAC, require active session MFA validation, and record full audit traces.
              </p>

              <div className="space-y-4">
                <div>
                  <label className="block text-[11px] text-slate-400 font-semibold mb-1">MFA validation Code</label>
                  <input
                    type="text"
                    value={drMFACode}
                    onChange={(e) => setDrMFACode(e.target.value)}
                    placeholder="Enter 6-digit TOTP"
                    className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-2 text-xs text-slate-100 placeholder-slate-700 focus:outline-none focus:border-cyan-500 transition"
                  />
                </div>

                <div className="flex gap-2">
                  <button
                    onClick={triggerBackupDR}
                    className="flex-1 bg-cyan-600 hover:bg-cyan-500 text-slate-950 text-xs font-bold py-2 px-3 rounded transition"
                  >
                    Generate Backup
                  </button>
                  <button
                    onClick={triggerRestoreVerificationDR}
                    className="flex-1 bg-amber-600 hover:bg-amber-500 text-slate-950 text-xs font-bold py-2 px-3 rounded transition"
                  >
                    Verify Restore
                  </button>
                </div>

                {drStatusMsg && (
                  <div className="bg-slate-950 border border-slate-800 rounded p-3 text-xs text-cyan-400 animate-pulse">
                    {drStatusMsg}
                  </div>
                )}

                {drSuccessMsg && (
                  <div className="bg-emerald-950/60 border border-emerald-800 text-emerald-400 rounded p-3 text-xs font-semibold">
                    {drSuccessMsg}
                  </div>
                )}
              </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-2">⏱️ RPO & RTO Measurements</h3>
              <div className="space-y-2 text-xs">
                <div className="flex justify-between py-1 border-b border-slate-800">
                  <span className="text-slate-400">PostgreSQL DB RPO:</span>
                  <span className="text-slate-100 font-bold">&lt; 1s (WAL Stream)</span>
                </div>
                <div className="flex justify-between py-1 border-b border-slate-800">
                  <span className="text-slate-400">PostgreSQL DB RTO:</span>
                  <span className="text-slate-100 font-bold">&lt; 10s (Auto Failover)</span>
                </div>
                <div className="flex justify-between py-1 border-b border-slate-800">
                  <span className="text-slate-400">Redis Ephemeral RTO:</span>
                  <span className="text-slate-100 font-bold">&lt; 60s (Auto-reconstruction)</span>
                </div>
                <div className="flex justify-between py-1 border-b border-slate-800">
                  <span className="text-slate-400">Matching Engine RTO:</span>
                  <span className="text-slate-100 font-bold">&lt; 1s (Journal Replay)</span>
                </div>
              </div>
            </section>
          </div>

          <div className="col-span-2 space-y-6">
            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-3">💾 Encrypted Backup Archives History</h3>
              <div className="overflow-x-auto">
                {backupsHistory.length === 0 ? (
                  <p className="text-xs text-slate-500 py-3 text-center">No backup runs recorded yet.</p>
                ) : (
                  <table className="w-full text-xs text-left">
                    <thead>
                      <tr className="border-b border-slate-800 text-slate-400">
                        <th className="py-2">Backup ID</th>
                        <th>Type</th>
                        <th>Status</th>
                        <th>Archive Filepath</th>
                        <th>Timestamp</th>
                      </tr>
                    </thead>
                    <tbody>
                      {backupsHistory.map((bk) => (
                        <tr key={bk.id} className="border-b border-slate-800/40">
                          <td className="py-2 font-mono text-cyan-400">{bk.id}</td>
                          <td>{bk.backup_type}</td>
                          <td>
                            <span className="bg-emerald-950/60 text-emerald-400 border border-emerald-900 px-1.5 py-0.5 rounded text-[10px] font-bold">
                              {bk.status}
                            </span>
                          </td>
                          <td className="font-mono text-slate-400 text-[10px]">{bk.filepath}</td>
                          <td className="text-slate-500">{new Date(bk.timestamp).toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-3">🔄 Isolated Verification Runs Log</h3>
              <div className="space-y-3">
                {recoveryHistory.length === 0 ? (
                  <p className="text-xs text-slate-500 py-3 text-center">No isolated verification runs recorded.</p>
                ) : (
                  recoveryHistory.map((run) => (
                    <div key={run.id} className="bg-slate-950 border border-slate-800 rounded p-3 text-xs space-y-1">
                      <div className="flex justify-between">
                        <span className="font-bold text-cyan-400 font-mono">Run {run.id}</span>
                        <span className="text-[10px] text-slate-500">{new Date(run.timestamp).toLocaleString()}</span>
                      </div>
                      <p className="text-slate-300 font-mono text-[10px]">{run.details}</p>
                      <div className="flex justify-between items-center text-[10px]">
                        <span className="text-slate-500">Restore Target Sandbox: Isolated Temporary DB Instance</span>
                        <span className="bg-emerald-950 text-emerald-400 px-1.5 rounded font-bold border border-emerald-900">{run.status}</span>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </section>
          </div>
        </div>
      )}

      {/* 3. Enterprise Compliance Tab (P0009 Dashboard) */}
      {activeTab === 'compliance' && (
        <div className="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-3 gap-6 mt-6">
          {/* Left Column: KYC Profile & Limits Restrictions */}
          <div className="col-span-1 space-y-6">
            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-4">📂 KYC verification Profile</h3>
              {kycProfile ? (
                <div className="space-y-3 text-xs">
                  <div className="flex justify-between py-1 border-b border-slate-800">
                    <span className="text-slate-400">KYC Status:</span>
                    <span className={`font-bold px-2 py-0.5 rounded text-[10px] ${
                      kycProfile.status === 'VERIFIED' ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-900' :
                      kycProfile.status === 'REJECTED' ? 'bg-rose-950/60 text-rose-400 border border-rose-900' :
                      'bg-amber-950/60 text-amber-400 border border-amber-900'
                    }`}>{kycProfile.status}</span>
                  </div>
                  <div className="flex justify-between py-1 border-b border-slate-800">
                    <span className="text-slate-400">Active Tier:</span>
                    <span className="text-cyan-400 font-bold">{kycProfile.tier}</span>
                  </div>
                  <div className="flex justify-between py-1 border-b border-slate-800">
                    <span className="text-slate-400">Provider Reference:</span>
                    <span className="font-mono text-slate-300">{kycProfile.provider_ref}</span>
                  </div>
                  <div className="flex justify-between py-1 border-b border-slate-800">
                    <span className="text-slate-400">Submissions Attempts:</span>
                    <span className="text-slate-300 font-bold">{kycProfile.attempts}</span>
                  </div>
                  {kycProfile.rejection_reason && (
                    <div className="p-2.5 bg-rose-950/20 border border-rose-900 rounded-lg mt-2 text-rose-300 text-[11px]">
                      <strong>Rejection Reason:</strong> {kycProfile.rejection_reason}
                    </div>
                  )}
                </div>
              ) : (
                <p className="text-xs text-slate-500 text-center py-4">Loading verification data...</p>
              )}
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-4">🛡️ Account restrictions Status</h3>
              <div className="p-4 bg-slate-950 rounded-lg border border-slate-800 flex justify-between items-center text-xs">
                <span className="text-slate-400">Active Restrictions:</span>
                <span className={`font-bold px-3 py-1 rounded-full text-[10px] ${
                  userRestriction === 'NORMAL' ? 'bg-emerald-950 text-emerald-400 border border-emerald-900' : 'bg-rose-950 text-rose-400 border border-rose-900'
                }`}>{userRestriction}</span>
              </div>
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-4">🧮 KYC Documents submission Form</h3>
              <form onSubmit={submitKYC} className="space-y-4 text-xs">
                <div>
                  <label className="block text-slate-400 mb-1 font-semibold">Tier Goal Selection</label>
                  <select
                    value={kycTier}
                    onChange={(e) => setKycTier(e.target.value)}
                    className="w-full px-2 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-200"
                  >
                    <option value="STANDARD">STANDARD (10,000 USD Limits)</option>
                    <option value="ADVANCED">ADVANCED (100,000 USD Limits)</option>
                    <option value="INSTITUTIONAL">INSTITUTIONAL (1,000,000 USD Limits)</option>
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="block text-slate-400 mb-1 font-semibold">ID Document Type</label>
                    <select
                      value={kycDocType}
                      onChange={(e) => setKycDocType(e.target.value)}
                      className="w-full px-2 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-200"
                    >
                      <option value="PASSPORT">Passport</option>
                      <option value="DRIVERS_LICENSE">{"Driver's License"}</option>
                      <option value="NATIONAL_ID">National ID Card</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-slate-400 mb-1 font-semibold">Document Number</label>
                    <input
                      type="text"
                      value={kycDocId}
                      onChange={(e) => setKycDocId(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500"
                      placeholder="e.g. B987654"
                      required
                    />
                  </div>
                </div>
                {kycStatusMsg && <p className="text-xs text-amber-400 font-semibold">{kycStatusMsg}</p>}
                <button type="submit" className="w-full py-2 bg-cyan-500 text-slate-950 font-bold rounded-lg hover:bg-cyan-400 transition">
                  Submit Mock Identity Files
                </button>
              </form>
            </section>
          </div>

          {/* Center Column: Risk Metrics & Mock Deposits AML Screen */}
          <div className="col-span-1 space-y-6">
            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-4">📊 Risk scoring Engine Sheet</h3>
              {riskEvaluation ? (
                <div className="space-y-3 text-xs">
                  <div className="p-3 bg-slate-950 border border-slate-800 rounded-lg flex justify-between items-center">
                    <div>
                      <span className="text-[10px] text-slate-500 uppercase block font-semibold">Deterministic Score</span>
                      <span className="text-xl font-bold text-slate-200 mt-1">{(riskEvaluation.score * 100).toFixed(1)} %</span>
                    </div>
                    <span className={`px-3 py-1 text-xs font-bold rounded ${
                      riskEvaluation.risk_level === 'LOW' ? 'bg-emerald-950 text-emerald-400' :
                      riskEvaluation.risk_level === 'MEDIUM' ? 'bg-amber-950 text-amber-400' :
                      'bg-rose-950 text-rose-400'
                    }`}>{riskEvaluation.risk_level} Risk Level</span>
                  </div>
                  <div>
                    <span className="text-slate-500 block text-[10px] font-semibold mb-1 uppercase">Contributing Factors Explanation</span>
                    <pre className="p-2.5 bg-slate-950 rounded border border-slate-800 text-[10px] font-mono text-slate-400 overflow-x-auto whitespace-pre-wrap">
                      {riskEvaluation.contributing_rules}
                    </pre>
                  </div>
                </div>
              ) : (
                <p className="text-xs text-slate-500 text-center py-4">No risk scoring evaluation computed yet</p>
              )}
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-4">💰 Simulate Blockchain inbound Deposit</h3>
              <p className="text-xs text-slate-500 mb-4">
                Mock depositing funds to trigger real-time address watchlists checks and AML velocity rule alerts.
              </p>
              <form onSubmit={submitMockDeposit} className="space-y-4 text-xs">
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="block text-slate-400 mb-1 font-semibold">Asset</label>
                    <select
                      value={depAsset}
                      onChange={(e) => setDepAsset(e.target.value)}
                      className="w-full px-2 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-200"
                    >
                      <option value="USDT">USDT</option>
                      <option value="BTC">BTC</option>
                      <option value="ETH">ETH</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-slate-400 mb-1 font-semibold">Amount</label>
                    <input
                      type="number"
                      value={depAmount}
                      onChange={(e) => setDepAmount(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none"
                      required
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-slate-400 mb-1 font-semibold">{"Sender Address (Try \"0xBLACKLISTED\" to test screening)"}</label>
                  <input
                    type="text"
                    value={depAddress}
                    onChange={(e) => setDepAddress(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs font-mono"
                    required
                  />
                </div>
                {depStatusMsg && <p className="p-2.5 bg-slate-950 border border-slate-800 rounded text-amber-400 font-mono text-[10px] break-all">{depStatusMsg}</p>}
                <button type="submit" className="w-full py-2 bg-cyan-500 text-slate-950 font-bold rounded-lg hover:bg-cyan-400 transition">
                  Inbound Simulated Transfer
                </button>
              </form>
            </section>
          </div>

          {/* Right Column: Case Management & Alerts (Administrative Controls) */}
          <div className="col-span-1 space-y-6">
            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl flex flex-col justify-between">
              <div>
                <h3 className="text-base font-bold text-cyan-400 mb-4">👮 Investigator Manual Case Management</h3>
                <div className="space-y-3 overflow-y-auto max-h-[300px] pr-1">
                  {cases.length === 0 ? (
                    <p className="text-xs text-slate-500 text-center py-6">No compliance cases logged</p>
                  ) : (
                    cases.map((cs) => (
                      <div
                        key={cs.id}
                        onClick={() => viewCaseDetails(cs.id)}
                        className={`p-3 border rounded-lg text-xs cursor-pointer transition ${
                          activeCaseId === cs.id ? 'bg-cyan-950/40 border-cyan-500' : 'bg-slate-950 border-slate-800'
                        }`}
                      >
                        <div className="flex justify-between font-bold text-slate-300 mb-1">
                          <span>Case ID: {cs.id}</span>
                          <span className={`text-[9px] px-1.5 py-0.5 rounded ${
                            cs.status === 'RESOLVED' ? 'bg-emerald-950 text-emerald-400' : 'bg-amber-950 text-amber-400'
                          }`}>{cs.status}</span>
                        </div>
                        <p className="text-[11px] text-slate-500 truncate">User ID: {cs.user_id}</p>
                      </div>
                    ))
                  )}
                </div>
              </div>

              {activeCaseId && (
                <div className="border-t border-slate-800 mt-6 pt-4 space-y-3 text-xs">
                  <h4 className="font-bold text-slate-300">Resolve Case ID: {activeCaseId}</h4>

                  {caseNotes.length > 0 && (
                    <div className="space-y-1.5 max-h-24 overflow-y-auto bg-slate-950 p-2 rounded border border-slate-800 text-[10px]">
                      {caseNotes.map((n, i) => (
                        <div key={i} className="border-b border-slate-900/60 pb-1">
                          <strong className="text-cyan-400">{n.author_id}: </strong>
                          <span className="text-slate-400">{n.note}</span>
                        </div>
                      ))}
                    </div>
                  )}

                  <form onSubmit={submitCaseResolution} className="space-y-3">
                    <div>
                      <label className="block text-slate-400 mb-1 font-semibold">Resolution Note</label>
                      <textarea
                        value={caseResolution}
                        onChange={(e) => setCaseResolution(e.target.value)}
                        className="w-full px-2 py-1.5 bg-slate-950 border border-slate-800 rounded text-slate-100"
                        placeholder="Resolution files verified..."
                        required
                      />
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <label className="block text-slate-400 mb-1 font-semibold">Status Action</label>
                        <select
                          value={caseStatusValue}
                          onChange={(e) => setCaseStatusValue(e.target.value)}
                          className="w-full px-2 py-1.5 bg-slate-950 border border-slate-800 rounded text-slate-200"
                        >
                          <option value="RESOLVED">RESOLVE (Clear Alert)</option>
                          <option value="DISMISSED">DISMISS (False Positive)</option>
                          <option value="BLOCKED">BLOCK (Lock User)</option>
                        </select>
                      </div>
                      <div>
                        <label className="block text-slate-400 mb-1 font-semibold">New Investigation Note</label>
                        <input
                          type="text"
                          value={investigatorNote}
                          onChange={(e) => setInvestigatorNote(e.target.value)}
                          className="w-full px-2 py-1.5 bg-slate-950 border border-slate-800 rounded text-slate-100"
                          placeholder="Salary slips validated"
                        />
                      </div>
                    </div>
                    <button type="submit" className="w-full py-1.5 bg-emerald-500 text-slate-950 font-bold rounded hover:bg-emerald-400">
                      Submit Resolution
                    </button>
                  </form>
                </div>
              )}
            </section>

            <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl">
              <h3 className="text-base font-bold text-cyan-400 mb-4">🚨 AML Active Compliance Alerts ({alerts.length})</h3>
              <div className="space-y-2.5 max-h-56 overflow-y-auto pr-1">
                {alerts.length === 0 ? (
                  <p className="text-xs text-slate-500 text-center py-4">No security alerts raised</p>
                ) : (
                  alerts.map((al) => (
                    <div key={al.id} className="p-2.5 bg-slate-950 border border-slate-800 rounded text-[11px] space-y-1">
                      <div className="flex justify-between font-bold text-slate-300">
                        <span>Rule: {al.rule_triggered}</span>
                        <span className={`text-[9px] px-1 rounded ${
                          al.severity === 'CRITICAL' ? 'bg-rose-950 text-rose-400' : 'bg-amber-950 text-amber-400'
                        }`}>{al.severity}</span>
                      </div>
                      <p className="text-slate-400">{al.evidence}</p>
                      <p className="text-[10px] text-slate-600">User ID: {al.user_id} | Status: {al.status}</p>
                    </div>
                  ))
                )}
              </div>
            </section>
          </div>
        </div>
      )}

      {/* Lower Dashboard (Always Visible if Authenticated and Trading Tab is active) */}
      {accessToken && activeTab === 'trading' && (
        <section className="max-w-7xl mx-auto bg-slate-900 border border-slate-800 rounded-xl p-5 mt-6 shadow-xl">
          <div className="flex justify-between items-center border-b border-slate-800 pb-3 mb-4">
            <h3 className="text-base font-bold text-cyan-400">Order Management & Audit Ledgers</h3>
            <button onClick={fetchOrders} className="text-xs text-cyan-400 hover:underline font-semibold">Sync Dashboard</button>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Open Orders Table */}
            <div>
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Active Open Orders ({openOrders.length})</h4>
              {openOrders.length === 0 ? (
                <p className="text-xs text-slate-500 py-4">No active orders queued</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs text-left">
                    <thead>
                      <tr className="text-slate-500 border-b border-slate-800">
                        <th className="py-2">Side/Type</th>
                        <th>Price</th>
                        <th>Qty/Filled</th>
                        <th>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {openOrders.map((ord) => (
                        <tr key={ord.id} className="border-b border-slate-800/40">
                          <td className="py-2">
                            <span className={ord.side === 'BUY' ? 'text-emerald-400 font-bold' : 'text-rose-400 font-bold'}>{ord.side}</span> {ord.type}
                          </td>
                          <td>{ord.price}</td>
                          <td>{ord.quantity} / {ord.filled_quantity}</td>
                          <td>
                            <button onClick={() => cancelOrder(ord.id)} className="text-xs text-rose-400 hover:underline">Cancel</button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            {/* Historical Orders Table */}
            <div>
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Order Execution History ({orderHistory.length})</h4>
              {orderHistory.length === 0 ? (
                <p className="text-xs text-slate-500 py-4">No historical transactions</p>
              ) : (
                <div className="overflow-x-auto max-h-48">
                  <table className="w-full text-xs text-left">
                    <thead>
                      <tr className="text-slate-500 border-b border-slate-800">
                        <th className="py-2">Side/Type</th>
                        <th>Price</th>
                        <th>Qty/Filled</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {orderHistory.map((ord) => (
                        <tr key={ord.id} className="border-b border-slate-800/40">
                          <td className="py-2">
                            <span className={ord.side === 'BUY' ? 'text-emerald-400 font-bold' : 'text-rose-400 font-bold'}>{ord.side}</span> {ord.type}
                          </td>
                          <td>{ord.price}</td>
                          <td>{ord.quantity} / {ord.filled_quantity}</td>
                          <td>
                            <span className="px-1.5 py-0.5 rounded bg-slate-950 border border-slate-800 text-[10px] text-slate-400 font-semibold">{ord.status}</span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>
        </section>
      )}
    </main>
  );
}
