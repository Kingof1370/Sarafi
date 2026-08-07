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

export default function Home() {
  const { user, accessToken, setAuth, logout } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [isRegister, setIsRegister] = useState(false);

  // App Tabs: trading vs security
  const [activeTab, setActiveTab] = useState<'trading' | 'security'>('trading');

  // Theme & Layout state
  const [theme, setTheme] = useState<'dark' | 'light'>('dark');
  const [layoutMode, setLayoutMode] = useState<'classic' | 'pro'>('pro');

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
      fetchSecurityState();
      const interval = setInterval(() => {
        fetchOrders();
        fetchSecurityState();
      }, 4000);
      return () => clearInterval(interval);
    }
  }, [accessToken, fetchOrders, fetchSecurityState]);

  const totalCost = parseFloat(price) * parseFloat(quantity);
  const estimatedFees = totalCost * (side === 'BUY' ? 0.002 : 0.001);

  return (
    <main className={`min-h-screen p-6 font-sans transition-colors ${theme === 'dark' ? 'bg-slate-950 text-slate-100' : 'bg-slate-50 text-slate-900'}`}>
      {/* Upper Navigation Header */}
      <header className={`max-w-7xl mx-auto flex justify-between items-center pb-4 border-b ${theme === 'dark' ? 'border-slate-800' : 'border-slate-300'}`}>
        <div className="flex items-center gap-6">
          <h1 className="text-2xl font-extrabold tracking-wider bg-gradient-to-r from-cyan-400 to-blue-600 bg-clip-text text-transparent">
            VELYXORA PRO
          </h1>
          <div className="flex items-center gap-4 bg-slate-900/60 p-1.5 rounded-lg border border-slate-800 text-xs">
            <span className="text-slate-400 font-semibold">{symbol}</span>
            <span className="text-emerald-400 font-bold">${ticker.lastPrice.toFixed(2)}</span>
            <span className="text-slate-500">24h High: ${ticker.high.toFixed(2)}</span>
            <span className="text-slate-500">24h Low: ${ticker.low.toFixed(2)}</span>
          </div>
        </div>

        {/* Workspace Customizer Controls */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-1.5 bg-slate-900/60 p-1 rounded-lg border border-slate-800 text-xs">
            <button
              onClick={() => setActiveTab(activeTab === 'trading' ? 'security' : 'trading')}
              className={`px-3 py-1 rounded font-bold transition ${activeTab === 'security' ? 'bg-cyan-500 text-slate-950' : 'text-slate-300 hover:bg-slate-800'}`}
            >
              {activeTab === 'trading' ? '🛡️ Security Center' : '📈 Back to Trading'}
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
                <div className="grid grid-cols-3 text-rose-400 bg-rose-950/20">
                  <span>50050.00</span>
                  <span className="text-right">1.50</span>
                  <span className="text-right">$75,075</span>
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
                <div className="grid grid-cols-3 text-emerald-400">
                  <span>49950.00</span>
                  <span className="text-right">2.10</span>
                  <span className="text-right">$104,895</span>
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
                <div className="grid grid-cols-3 text-rose-400">
                  <span>49995.00</span>
                  <span className="text-right">0.0500</span>
                  <span className="text-right text-slate-500">12:34:49</span>
                </div>
              </div>
            </div>
          </section>

          {/* Right Column: Portfolio Allocations */}
          <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl flex flex-col justify-between">
            <div>
              <h3 className="text-sm font-bold text-slate-300 border-b border-slate-800 pb-2 mb-3">Portfolio Allocations</h3>
              <div className="space-y-4">
                <div className="bg-slate-950 p-3 rounded-lg border border-slate-800/60">
                  <span className="block text-[10px] text-slate-500 font-medium">Estimated Balance Sheet (USDT)</span>
                  <span className="block text-lg font-bold text-slate-200 mt-1">$15,450.00</span>
                </div>
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
                      <p className="text-[10px] text-slate-600 mt-1">Started: {new Date(sess.created_at).toLocaleString()}</p>
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* MFA QR/Enrollment Settings */}
            <div className="border-t border-slate-800 mt-6 pt-4">
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">P0003 MFA Setup Control</h4>
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
                    <span className="text-slate-500 text-[10px] block">MFA Key Secret (Base32):</span>
                    <code className="text-slate-300 font-bold block mt-0.5 bg-slate-900 p-1 rounded select-all text-center">{mfaSecret}</code>
                  </div>
                  <div>
                    <span className="text-slate-500 text-[10px] block">QR Code Enrollment URI:</span>
                    <input
                      readOnly
                      value={mfaQrUrl}
                      className="w-full bg-slate-900 border border-slate-800 rounded p-1 text-[10px] text-slate-400 focus:outline-none"
                    />
                  </div>
                  {mfaBackupCodes.length > 0 && (
                    <div>
                      <span className="text-slate-500 text-[10px] block">MFA Backup Codes:</span>
                      <div className="grid grid-cols-2 gap-1 mt-1 font-mono text-[10px]">
                        {mfaBackupCodes.map((code) => (
                          <span key={code} className="bg-slate-900 text-slate-300 border border-slate-800 text-center py-0.5 rounded">{code}</span>
                        ))}
                      </div>
                    </div>
                  )}
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
                <label className="block text-slate-400 mb-1 font-semibold">Permissions / Scopes</label>
                <div className="space-y-2 mt-2 bg-slate-950 p-3 rounded-lg border border-slate-800/80">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={keyPerms.walletRead}
                      onChange={(e) => setKeyPermissions({ ...keyPerms, walletRead: e.target.checked })}
                      className="rounded border-slate-800 bg-slate-900 text-cyan-500 focus:ring-cyan-500"
                    />
                    <span>wallet:read (Fetch balances & address)</span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={keyPerms.walletWrite}
                      onChange={(e) => setKeyPermissions({ ...keyPerms, walletWrite: e.target.checked })}
                      className="rounded border-slate-800 bg-slate-900 text-cyan-500 focus:ring-cyan-500"
                    />
                    <span>wallet:write (Withdraw assets)</span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={keyPerms.tradingWrite}
                      onChange={(e) => setKeyPermissions({ ...keyPerms, tradingWrite: e.target.checked })}
                      className="rounded border-slate-800 bg-slate-900 text-cyan-500 focus:ring-cyan-500"
                    />
                    <span>trading:write (Place & cancel orders)</span>
                  </label>
                </div>
              </div>

              <div>
                <label className="block text-slate-400 mb-1 font-semibold">IP Allowlist (Comma Separated Address / CIDR)</label>
                <input
                  type="text"
                  value={keyIPAllowlist}
                  onChange={(e) => setKeyIPAllowlist(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500"
                  placeholder="e.g., 12.34.56.78, 192.168.1.0/24"
                />
              </div>

              <div>
                <label className="block text-slate-400 mb-1 font-semibold">Expiration Duration</label>
                <select
                  value={keyExpiryDays}
                  onChange={(e) => setKeyExpiryDays(Number(e.target.value))}
                  className="w-full px-2 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-200"
                >
                  <option value={7}>7 Days</option>
                  <option value={30}>30 Days</option>
                  <option value={90}>90 Days</option>
                  <option value={365}>365 Days</option>
                </select>
              </div>

              <div>
                <label className="block text-slate-400 mb-1 font-semibold">MFA Validation Code (P0003 hurdle)</label>
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
                      <div className="mt-2 space-y-1 text-slate-500 text-[10px]">
                        <p className="font-mono">API Key: {key.api_key}</p>
                        <p>Scopes: <span className="text-cyan-400">{key.permissions}</span></p>
                        {key.ip_allowlist && <p className="truncate">IPs: {key.ip_allowlist}</p>}
                        <p>Expires: {new Date(key.expires_at).toLocaleDateString()}</p>
                        {key.last_used_at && <p>Last Used: {new Date(key.last_used_at).toLocaleString()}</p>}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* Created Credentials Modal Overlay */}
            {createdKey && (
              <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm flex justify-center items-center p-4 z-50">
                <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 shadow-2xl relative">
                  <h4 className="text-lg font-bold text-emerald-400 mb-2">🎉 API Key Generated</h4>
                  <p className="text-xs text-slate-400 mb-4">
                    Copy and store your API secret immediately! For security reasons, <span className="text-amber-400 font-bold">it will never be displayed again</span>.
                  </p>

                  <div className="space-y-3 font-mono text-xs text-left mb-6">
                    <div className="bg-slate-950 p-2.5 rounded border border-slate-800">
                      <span className="text-slate-500 text-[10px] block uppercase font-bold">API Key ID</span>
                      <span className="text-slate-300 block select-all font-semibold break-all mt-1">{createdKey.apiKey}</span>
                    </div>
                    <div className="bg-slate-950 p-2.5 rounded border border-slate-800">
                      <span className="text-slate-500 text-[10px] block uppercase font-bold text-rose-400">API Key Secret</span>
                      <span className="text-slate-300 block select-all font-bold break-all mt-1">{createdKey.apiSecret}</span>
                    </div>
                  </div>

                  <button
                    onClick={() => setCreatedKey(null)}
                    className="w-full py-2 bg-emerald-500 hover:bg-emerald-400 text-slate-950 rounded-lg text-xs font-bold"
                  >
                    I Have Safely Saved the Secret
                  </button>
                </div>
              </div>
            )}
          </section>
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
